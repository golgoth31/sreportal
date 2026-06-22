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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
)

// listTagsTimeout caps the duration of a single registry tag-list call. A slow
// or unresponsive registry would otherwise hold a goroutine for the full
// async-resolve budget (30 minutes), starving the in-flight semaphore.
const listTagsTimeout = 30 * time.Second

// imageConfigTimeout caps the duration of a single image-config fetch (manifest
// + config blob). Mirrors listTagsTimeout's rationale.
const imageConfigTimeout = 30 * time.Second

// CraneClient implements imageregistry.Client using google/go-containerregistry.
//
// Auth uses authn.DefaultKeychain (reads ~/.docker/config.json, IRSA, GCR/ECR
// helpers). No K8s-pull-secret support in v1 (cf. plan §5).
type CraneClient struct{}

// Compile-time interface check.
var _ domainimageregistry.Client = (*CraneClient)(nil)

// NewCraneClient builds a CraneClient.
func NewCraneClient() *CraneClient { return &CraneClient{} }

// ListTags returns the raw tag list for `host/repository`.
//
// Errors:
//   - 404 → empty tag list, no error (treated as "repo does not exist").
//   - 429 → wrapped imageregistry.ErrRateLimited (errors.Is-matchable).
//   - others → wrapped %w preserving the underlying transport error.
func (c *CraneClient) ListTags(ctx context.Context, host, repository string) ([]string, error) {
	reg, err := name.NewRegistry(host, name.StrictValidation, name.WithDefaultRegistry(host))
	if err != nil {
		return nil, fmt.Errorf("parse registry %q: %w", host, err)
	}
	repo := reg.Repo(strings.TrimPrefix(repository, "/"))

	callCtx, cancel := context.WithTimeout(ctx, listTagsTimeout)
	defer cancel()

	tags, err := remote.List(repo, remote.WithContext(callCtx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err == nil {
		return tags, nil
	}

	// Map transport errors. go-containerregistry wraps registry HTTP errors as *transport.Error.
	var te *transport.Error
	if errors.As(err, &te) {
		switch te.StatusCode {
		case http.StatusNotFound:
			// Repository does not exist on the registry — return empty.
			return nil, nil
		case http.StatusTooManyRequests:
			return nil, fmt.Errorf("list tags %s/%s: %w", host, repository, domainimageregistry.ErrRateLimited)
		}
	}
	return nil, fmt.Errorf("list tags %s/%s: %w", host, repository, err)
}

// ImageConfigLabels returns the OCI image config labels (the org.opencontainers.image.* set)
// for host/repository:reference. Used to resolve an image to its git source.
//
// Errors:
//   - 404 → nil labels, no error (treated as "image does not exist").
//   - 429 → wrapped imageregistry.ErrRateLimited (errors.Is-matchable).
//   - others → wrapped %w preserving the underlying transport error.
func (c *CraneClient) ImageConfigLabels(ctx context.Context, host, repository, reference string) (map[string]string, error) {
	// A digest reference is joined with "@", a tag reference with ":".
	sep := ":"
	if strings.HasPrefix(reference, "sha256:") {
		sep = "@"
	}
	ref, err := name.ParseReference(
		host+"/"+strings.TrimPrefix(repository, "/")+sep+reference,
		name.WithDefaultRegistry(host),
	)
	if err != nil {
		return nil, fmt.Errorf("parse reference %s/%s%s%s: %w", host, repository, sep, reference, err)
	}

	callCtx, cancel := context.WithTimeout(ctx, imageConfigTimeout)
	defer cancel()

	img, err := remote.Image(ref, remote.WithContext(callCtx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, c.mapImageErr(host, repository, reference, err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, c.mapImageErr(host, repository, reference, err)
	}
	return cfg.Config.Labels, nil
}

// mapImageErr maps go-containerregistry transport errors for image fetches,
// mirroring ListTags' conventions: 404 → empty (nil), 429 → ErrRateLimited.
func (c *CraneClient) mapImageErr(host, repository, reference string, err error) error {
	var te *transport.Error
	if errors.As(err, &te) {
		switch te.StatusCode {
		case http.StatusNotFound:
			return nil
		case http.StatusTooManyRequests:
			return fmt.Errorf("image config %s/%s:%s: %w", host, repository, reference, domainimageregistry.ErrRateLimited)
		}
	}
	return fmt.Errorf("image config %s/%s:%s: %w", host, repository, reference, err)
}
