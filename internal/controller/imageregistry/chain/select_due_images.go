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
	"time"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// checkInterval is the nominal cadence for per-image registry lookups.
	checkInterval = 24 * time.Hour

	// jitterWindow is the maximum random spread applied to catch-up delays and
	// to the final requeue interval.
	jitterWindow = time.Hour

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
type SelectDueImagesHandler struct{}

// NewSelectDueImagesHandler constructs a SelectDueImagesHandler.
func NewSelectDueImagesHandler() *SelectDueImagesHandler {
	return &SelectDueImagesHandler{}
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

	minDelay := checkInterval + jitterWindow // sentinel: "infinity"
	rc.Data.DueImages = make([]DueImage, 0, dueCount)

	for _, c := range candidates {
		// rand.N is available in math/rand/v2 (Go 1.22+).
		delay := rand.N(checkInterval)
		if delay == 0 {
			// Process immediately.
			rc.Data.DueImages = append(rc.Data.DueImages, c.due)
		} else {
			// Skip this cycle; track when to wake up.
			if delay < minDelay {
				minDelay = delay
			}
		}
	}

	// If some images were deferred, ensure we requeue before their delay expires.
	deferred := dueCount - len(rc.Data.DueImages)
	if deferred > 0 {
		rc.Data.RequeueAfter = minDelay
		logger.V(1).Info("deferred images due to catch-up jitter", "deferred", deferred, "requeueAfter", minDelay)
	}

	logger.V(1).Info("images selected after jitter", "processing", len(rc.Data.DueImages), "deferred", deferred)
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
