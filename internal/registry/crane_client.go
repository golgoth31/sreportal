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

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
)

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

	tags, err := remote.List(repo, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
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
