/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package registry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	cranename "github.com/google/go-containerregistry/pkg/name"

	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
)

// fakeRegistryHandler returns a minimal /v2/<repo>/tags/list response.
func fakeRegistryHandler(t *testing.T, repo string, code int, body string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == tV2Path:
			// auth ping
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
			w.WriteHeader(http.StatusOK)
			return
		case strings.Contains(r.URL.Path, "/tags/list") && strings.Contains(r.URL.Path, repo):
			w.WriteHeader(code)
			_, _ = w.Write([]byte(body))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestCraneClient_ListTags_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(fakeRegistryHandler(t, "library/nginx", http.StatusOK,
		`{"name":"library/nginx","tags":["1.0.0","1.2.0","latest"]}`))
	defer srv.Close()

	host := stripScheme(t, srv.URL)
	c := NewCraneClient()

	tags, err := c.ListTags(context.Background(), host, "library/nginx")
	if err != nil {
		t.Fatalf("ListTags error: %v", err)
	}
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(tags), tags)
	}
}

func TestCraneClient_ListTags_NotFoundReturnsEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always 404, including the /v2/ ping → registry "exists" but repo doesn't.
		// go-containerregistry's pingResp falls back when /v2/ returns 401/200; but here
		// we simulate a successful ping then 404 on tags.
		if r.URL.Path == tV2Path {
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"code":"NAME_UNKNOWN"}]}`))
	}))
	defer srv.Close()

	host := stripScheme(t, srv.URL)
	c := NewCraneClient()

	tags, err := c.ListTags(context.Background(), host, "missing/repo")
	if err != nil {
		t.Fatalf("404 must be swallowed (empty tags), got error: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected empty tags on 404, got %v", tags)
	}
}

func TestCraneClient_ListTags_RateLimitedSurfacesErrRateLimited(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tV2Path {
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	host := stripScheme(t, srv.URL)
	c := NewCraneClient()

	_, err := c.ListTags(context.Background(), host, "library/nginx")
	if err == nil {
		t.Fatalf("expected error on 429")
	}
	if !errors.Is(err, domainimageregistry.ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

func TestCraneClient_ListTags_ServerErrorPropagates(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tV2Path {
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	host := stripScheme(t, srv.URL)
	c := NewCraneClient()

	_, err := c.ListTags(context.Background(), host, "library/nginx")
	if err == nil {
		t.Fatalf("expected error on 500")
	}
	if errors.Is(err, domainimageregistry.ErrRateLimited) {
		t.Fatalf("500 should NOT be ErrRateLimited")
	}
}

func TestCraneClient_ListTags_ContextCanceled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(fakeRegistryHandler(t, "library/nginx", http.StatusOK,
		`{"name":"library/nginx","tags":["1.0.0"]}`))
	defer srv.Close()

	host := stripScheme(t, srv.URL)
	c := NewCraneClient()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call so the request is aborted immediately

	_, err := c.ListTags(ctx, host, "library/nginx")
	if err == nil {
		t.Fatalf("expected error from canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestCraneClient_ListTags_InvalidHost(t *testing.T) {
	t.Parallel()

	c := NewCraneClient()
	_, err := c.ListTags(context.Background(), "not a host", "library/nginx")
	if err == nil {
		t.Fatalf("expected error on invalid host")
	}
}

func TestCraneClient_ImageConfigLabels_Success(t *testing.T) {
	t.Parallel()

	// Spin up an in-memory OCI registry and push an image carrying OCI labels.
	srv := httptest.NewServer(ggcrregistry.New())
	defer srv.Close()
	host := stripScheme(t, srv.URL)

	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("build random image: %v", err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("config file: %v", err)
	}
	cfg = cfg.DeepCopy()
	cfg.Config.Labels = map[string]string{
		"org.opencontainers.image.source":   "https://github.com/acme/widget",
		"org.opencontainers.image.revision": "abc123def456",
	}
	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		t.Fatalf("mutate config: %v", err)
	}

	ref, err := cranename.ParseReference(host + "/acme/widget:v1.2.3")
	if err != nil {
		t.Fatalf("parse ref: %v", err)
	}
	if err := remote.Write(ref, img); err != nil {
		t.Fatalf("push image: %v", err)
	}

	c := NewCraneClient()
	labels, err := c.ImageConfigLabels(context.Background(), host, "acme/widget", "v1.2.3")
	if err != nil {
		t.Fatalf("ImageConfigLabels error: %v", err)
	}
	if labels["org.opencontainers.image.source"] != "https://github.com/acme/widget" {
		t.Fatalf("unexpected source label: %q", labels["org.opencontainers.image.source"])
	}
	if labels["org.opencontainers.image.revision"] != "abc123def456" {
		t.Fatalf("unexpected revision label: %q", labels["org.opencontainers.image.revision"])
	}
}

func TestCraneClient_ImageConfigLabels_NotFoundReturnsNil(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(ggcrregistry.New())
	defer srv.Close()
	host := stripScheme(t, srv.URL)

	c := NewCraneClient()
	labels, err := c.ImageConfigLabels(context.Background(), host, "missing/repo", "v1.0.0")
	if err != nil {
		t.Fatalf("404 must be swallowed (nil labels), got error: %v", err)
	}
	if labels != nil {
		t.Fatalf("expected nil labels on 404, got %v", labels)
	}
}

func TestCraneClient_ImageConfigLabels_InvalidHost(t *testing.T) {
	t.Parallel()

	c := NewCraneClient()
	_, err := c.ImageConfigLabels(context.Background(), "not a host", "library/nginx", "latest")
	if err == nil {
		t.Fatalf("expected error on invalid host")
	}
}

// stripScheme returns the "host:port" portion of an httptest URL, since
// go-containerregistry's name.NewRegistry expects a bare host.
func stripScheme(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse httptest URL: %v", err)
	}
	return u.Host
}
