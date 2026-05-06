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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	// asyncResolveTimeout is the maximum duration for a background resolution goroutine.
	asyncResolveTimeout = 30 * time.Minute
)

// ResolveLatestVersionsHandler queries the registry-of-origin for the latest
// semver tag for each DueImage. Resolution runs asynchronously in a background
// goroutine so the reconciliation chain returns immediately. Once done, the
// goroutine patches the CR status directly, which triggers a new (fast)
// reconciliation that updates the readstore and counters. A sync.Map prevents
// duplicate goroutines per CR.
type ResolveLatestVersionsHandler struct {
	registryClient domainimageregistry.Client
	hostLimiter    *registry.HostLimiter
	client         client.Client
	running        sync.Map // CR "namespace/name" → struct{}
}

// NewResolveLatestVersionsHandler constructs a ResolveLatestVersionsHandler.
func NewResolveLatestVersionsHandler(
	registryClient domainimageregistry.Client,
	hostLimiter *registry.HostLimiter,
	c client.Client,
) *ResolveLatestVersionsHandler {
	return &ResolveLatestVersionsHandler{
		registryClient: registryClient,
		hostLimiter:    hostLimiter,
		client:         c,
	}
}

// Handle implements reconciler.Handler. It dispatches a background goroutine for
// the rate-limited registry lookups and returns immediately. If a goroutine is
// already running for this CR (detected via running map), the cycle is skipped.
func (h *ResolveLatestVersionsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, ChainData]) error {
	if len(rc.Data.DueImages) == 0 {
		return nil
	}

	crKey := rc.Resource.Namespace + "/" + rc.Resource.Name
	if _, loaded := h.running.LoadOrStore(crKey, struct{}{}); loaded {
		return nil
	}

	// Snapshot everything needed from rc before returning — rc and ctx are
	// owned by the reconciliation goroutine and must not be accessed later.
	dueImages := make([]DueImage, len(rc.Data.DueImages))
	copy(dueImages, rc.Data.DueImages)
	host := rc.Resource.Spec.Host
	crName := rc.Resource.Name
	crNamespace := rc.Resource.Namespace
	logger := log.FromContext(ctx).WithName("resolve-latest-versions").WithValues("cr", crKey)

	go func() {
		defer h.running.Delete(crKey)

		bgCtx, cancel := context.WithTimeout(context.Background(), asyncResolveTimeout)
		defer cancel()

		resolutions := h.ResolveSync(bgCtx, host, dueImages)

		if err := h.patchStatus(bgCtx, crNamespace, crName, resolutions); err != nil {
			logger.Error(err, "async: failed to patch ImageRegistry status")
		}
	}()

	return nil
}

// ResolveSync performs rate-limited registry lookups synchronously and returns
// the resolution map. It is the core resolution logic extracted for testability.
func (h *ResolveLatestVersionsHandler) ResolveSync(ctx context.Context, host string, dueImages []DueImage) map[string]Resolution {
	logger := log.FromContext(ctx).WithName("resolve-latest-versions")

	sem := make(chan struct{}, maxConcurrentLookups)

	type result struct {
		resolution Resolution
	}
	results := make([]result, len(dueImages))

	var wg sync.WaitGroup
	for i, due := range dueImages {
		wg.Add(1)
		go func(idx int, img DueImage) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = result{resolution: h.resolveOne(ctx, host, img, logger)}
		}(i, due)
	}
	wg.Wait()

	resolutions := make(map[string]Resolution, len(results))
	for _, r := range results {
		resolutions[r.resolution.Key] = r.resolution
	}
	return resolutions
}

// patchStatus merges resolutions into the CR's Status.Images and persists via
// a status subresource patch. It re-fetches the CR to avoid conflicts with
// concurrent UpdateStatusHandler runs. A 404 (CR deleted) is silently ignored.
func (h *ResolveLatestVersionsHandler) patchStatus(ctx context.Context, namespace, name string, resolutions map[string]Resolution) error {
	var ir sreportalv1alpha1.ImageRegistry
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &ir); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get ImageRegistry: %w", err)
	}

	base := ir.DeepCopy()

	statusByKey := make(map[string]sreportalv1alpha1.ImageRegistryStatusEntry, len(ir.Status.Images))
	for _, st := range ir.Status.Images {
		statusByKey[st.Key] = st
	}
	for key, res := range resolutions {
		t := metav1.NewTime(res.LastCheckedAt)
		statusByKey[key] = sreportalv1alpha1.ImageRegistryStatusEntry{
			Key:              key,
			LatestVersion:    res.LatestVersion,
			UpgradeAvailable: res.UpgradeAvailable,
			LastCheckedAt:    &t,
			LastError:        res.LastError,
		}
	}

	newImages := make([]sreportalv1alpha1.ImageRegistryStatusEntry, 0, len(ir.Spec.Images))
	for _, entry := range ir.Spec.Images {
		if st, ok := statusByKey[entry.Key]; ok {
			newImages = append(newImages, st)
		} else {
			newImages = append(newImages, sreportalv1alpha1.ImageRegistryStatusEntry{Key: entry.Key})
		}
	}
	ir.Status.Images = newImages

	return h.client.Status().Patch(ctx, &ir, client.MergeFrom(base))
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
