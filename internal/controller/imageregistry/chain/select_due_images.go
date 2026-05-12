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
	"math/rand/v2"
	"sort"
	"time"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// checkInterval is the nominal cadence for per-image registry lookups.
	checkInterval = 24 * time.Hour

	// catchUpThreshold is the fraction of due images that triggers catch-up jitter.
	catchUpThreshold = 0.5
)

// SelectDueImagesHandler inspects each Spec.Images[i] against its Status entry
// and decides which images need a registry lookup this cycle.
//
// Catch-up jitter (§6.2): if more than 50% of images are due AND the Status is
// non-empty (indicating a restart rather than a brand-new CR), each due image is
// assigned a random delay in [0, 24h]. Images with delay > 0 are skipped this
// cycle; the handler sets ChainData.RequeueAfter to the minimum remaining delay
// so the controller wakes up early enough to handle the next batch.
type SelectDueImagesHandler struct {
	// jitter returns the per-image delay during catch-up. Injected for tests; the
	// production default samples uniformly from [0, max).
	jitter func(max time.Duration) time.Duration
}

// NewSelectDueImagesHandler constructs a SelectDueImagesHandler.
func NewSelectDueImagesHandler() *SelectDueImagesHandler {
	return &SelectDueImagesHandler{
		jitter: func(max time.Duration) time.Duration {
			// rand.N is available in math/rand/v2 (Go 1.22+).
			return rand.N(max)
		},
	}
}

// NewSelectDueImagesHandlerWithJitter constructs a handler with a custom jitter
// function. Intended for tests that need deterministic delays.
func NewSelectDueImagesHandlerWithJitter(jitter func(max time.Duration) time.Duration) *SelectDueImagesHandler {
	return &SelectDueImagesHandler{jitter: jitter}
}

// Handle implements reconciler.Handler.
func (h *SelectDueImagesHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, ChainData]) error {
	logger := log.FromContext(ctx).WithName("select-due-images")

	now := time.Now()
	spec := rc.Resource.Spec
	status := rc.Resource.Status

	// Build a quick lookup map from key → status entry.
	statusByKey := make(map[string]*sreportalv1alpha1.ImageRegistryStatusEntry, len(status.Images))
	for i := range status.Images {
		statusByKey[status.Images[i].Key] = &status.Images[i]
	}

	total := len(spec.Images)
	if total == 0 {
		return nil
	}

	// Collect all "due" candidates first.
	type candidate struct {
		due DueImage
	}
	candidates := make([]candidate, 0, total)

	for i := range spec.Images {
		entry := spec.Images[i]
		st := statusByKey[entry.Key]

		if isDue(st, now) {
			candidates = append(candidates, candidate{
				due: DueImage{Spec: entry, Status: st},
			})
		}
	}

	dueCount := len(candidates)
	if dueCount == 0 {
		logger.V(1).Info("no due images this cycle")
		return nil
	}

	// Apply catch-up jitter only when:
	//   - Status is non-empty (not a brand-new CR), AND
	//   - More than 50% of images are due (post-restart catch-up storm).
	applyJitter := len(status.Images) > 0 && float64(dueCount) > catchUpThreshold*float64(total)

	if !applyJitter {
		rc.Data.DueImages = make([]DueImage, 0, dueCount)
		for _, c := range candidates {
			rc.Data.DueImages = append(rc.Data.DueImages, c.due)
		}
		logger.V(1).Info("due images selected", "count", dueCount, "total", total)
		return nil
	}

	// Catch-up jitter: assign random delay to each candidate.
	logger.V(1).Info("applying catch-up jitter", "dueCount", dueCount, "total", total)

	type deferredImage struct {
		delay time.Duration
		due   DueImage
	}

	rc.Data.DueImages = make([]DueImage, 0, dueCount)
	deferredList := make([]deferredImage, 0, dueCount)

	for _, c := range candidates {
		delay := h.jitter(checkInterval)
		if delay == 0 {
			// Process immediately.
			rc.Data.DueImages = append(rc.Data.DueImages, c.due)
			continue
		}
		// Skip this cycle; track this image and its delay so we can pick the
		// earliest wake-up across the whole deferred set (rather than only
		// remembering the minimum delay and dropping the rest of the data).
		deferredList = append(deferredList, deferredImage{delay: delay, due: c.due})
	}

	// If some images were deferred, ensure we requeue before the earliest
	// deferred delay expires. The deferred images themselves are not stored in
	// DueImages this cycle — they will be re-selected on the next reconcile
	// because their Status.LastCheckedAt was not updated.
	if len(deferredList) > 0 {
		sort.Slice(deferredList, func(i, j int) bool {
			return deferredList[i].delay < deferredList[j].delay
		})
		rc.Data.RequeueAfter = deferredList[0].delay
		logger.V(1).Info("deferred images due to catch-up jitter", "deferred", len(deferredList), "requeueAfter", deferredList[0].delay)
	}

	logger.V(1).Info("images selected after jitter", "processing", len(rc.Data.DueImages), "deferred", len(deferredList))
	return nil
}

// isDue returns true when the status entry indicates the image has not been
// checked within the last checkInterval.
func isDue(st *sreportalv1alpha1.ImageRegistryStatusEntry, now time.Time) bool {
	if st == nil || st.LastCheckedAt == nil {
		return true
	}
	return st.LastCheckedAt.Time.Add(checkInterval).Before(now)
}
