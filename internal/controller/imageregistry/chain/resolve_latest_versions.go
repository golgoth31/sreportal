/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package chain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/registry"
)

const (
	// maxConcurrentLookups is the intra-CR concurrency limit for registry calls.
	maxConcurrentLookups = 4
)

// ResolveLatestVersionsHandler queries the registry-of-origin for the latest
// semver tag for each DueImage. Non-semver tags are marked as checked but no
// version is recorded. Rate-limiting is delegated to HostLimiter.
type ResolveLatestVersionsHandler struct {
	registryClient domainimageregistry.Client
	hostLimiter    *registry.HostLimiter
}

// NewResolveLatestVersionsHandler constructs a ResolveLatestVersionsHandler.
func NewResolveLatestVersionsHandler(
	registryClient domainimageregistry.Client,
	hostLimiter *registry.HostLimiter,
) *ResolveLatestVersionsHandler {
	return &ResolveLatestVersionsHandler{
		registryClient: registryClient,
		hostLimiter:    hostLimiter,
	}
}

// Handle implements reconciler.Handler.
func (h *ResolveLatestVersionsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, ChainData]) error {
	logger := log.FromContext(ctx).WithName("resolve-latest-versions")

	if len(rc.Data.DueImages) == 0 {
		return nil
	}

	host := rc.Resource.Spec.Host

	// Use a semaphore (buffered channel) to bound concurrency intra-CR.
	sem := make(chan struct{}, maxConcurrentLookups)

	type result struct {
		resolution Resolution
	}
	results := make([]result, len(rc.Data.DueImages))

	var wg sync.WaitGroup
	for i, due := range rc.Data.DueImages {
		wg.Add(1)
		go func(idx int, img DueImage) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res := h.resolveOne(ctx, host, img, logger)
			results[idx] = result{resolution: res}
		}(i, due)
	}
	wg.Wait()

	rc.Data.Resolutions = make(map[string]Resolution, len(results))
	for _, r := range results {
		rc.Data.Resolutions[r.resolution.Key] = r.resolution
	}
	return nil
}

// resolveOne runs the registry lookup for a single DueImage and returns the
// Resolution. It never returns an error — failures are embedded in Resolution.LastError.
func (h *ResolveLatestVersionsHandler) resolveOne(
	ctx context.Context,
	host string,
	img DueImage,
	logger *log.Logger,
) Resolution {
	now := time.Now()
	key := img.Spec.Key

	res := Resolution{
		Key:           key,
		LastCheckedAt: now,
	}

	// Skip non-semver tags — mark as checked, no version recorded.
	if domainimage.TagType(img.Spec.TagType) != domainimage.TagTypeSemver {
		metrics.RegistryLookupTotal.WithLabelValues(host, "skipped").Inc()
		logger.V(2).Info("skipping non-semver image", "key", key, "tagType", img.Spec.TagType)
		return res
	}

	// Rate-limit by host.
	if err := h.hostLimiter.Wait(ctx, host); err != nil {
		res.LastError = fmt.Sprintf("rate limiter wait: %s", err)
		metrics.RegistryLookupTotal.WithLabelValues(host, "error").Inc()
		return res
	}

	start := time.Now()
	tags, err := h.registryClient.ListTags(ctx, host, img.Spec.Repository)
	metrics.RegistryLookupDuration.WithLabelValues(host).Observe(time.Since(start).Seconds())

	if err != nil {
		if errors.Is(err, domainimageregistry.ErrRateLimited) {
			metrics.RegistryLookupTotal.WithLabelValues(host, "rate_limited").Inc()
		} else {
			metrics.RegistryLookupTotal.WithLabelValues(host, "error").Inc()
		}
		res.LastError = err.Error()
		logger.V(1).Info("registry lookup failed", "key", key, "error", err)
		return res
	}

	metrics.RegistryLookupTotal.WithLabelValues(host, "success").Inc()

	latest, found := domainimageregistry.PickLatestSemver(tags)
	if found {
		res.LatestVersion = latest
		res.UpgradeAvailable = domainimageregistry.IsUpgrade(img.Spec.OriginalTag, latest)
	}

	logger.V(2).Info("resolved latest version", "key", key, "latest", latest, "upgradeAvailable", res.UpgradeAvailable)
	return res
}
