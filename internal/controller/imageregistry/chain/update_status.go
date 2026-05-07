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
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// UpdateStatusHandler merges resolved entries into Status.Images[], recomputes
// summary counters, sets the Ready condition, and updates Prometheus gauges.
type UpdateStatusHandler struct {
	client client.Client
}

// NewUpdateStatusHandler constructs an UpdateStatusHandler.
func NewUpdateStatusHandler(c client.Client) *UpdateStatusHandler {
	return &UpdateStatusHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, ChainData]) error {
	logger := log.FromContext(ctx).WithName("update-status")

	ir := rc.Resource
	spec := ir.Spec
	base := ir.DeepCopy()

	// Build a mutable map from the existing Status.Images for merge.
	statusByKey := make(map[string]sreportalv1alpha1.ImageRegistryStatusEntry, len(ir.Status.Images))
	for _, st := range ir.Status.Images {
		statusByKey[st.Key] = st
	}

	// Apply resolutions for due images.
	for key, res := range rc.Data.Resolutions {
		t := metav1.NewTime(res.LastCheckedAt)
		statusByKey[key] = sreportalv1alpha1.ImageRegistryStatusEntry{
			Key:              key,
			LatestVersion:    res.LatestVersion,
			UpgradeAvailable: res.UpgradeAvailable,
			LastCheckedAt:    &t,
			LastError:        res.LastError,
		}
	}

	// Rebuild Status.Images in spec order for determinism.
	newImages := make([]sreportalv1alpha1.ImageRegistryStatusEntry, 0, len(spec.Images))
	for _, entry := range spec.Images {
		if st, ok := statusByKey[entry.Key]; ok {
			newImages = append(newImages, st)
		} else {
			// Entry not yet resolved — insert an empty placeholder so the
			// listMapKey slot exists in the status.
			newImages = append(newImages, sreportalv1alpha1.ImageRegistryStatusEntry{Key: entry.Key})
		}
	}
	ir.Status.Images = newImages

	// Recompute summary counters in a single pass over the merged status entries
	// so all values come from the same source of truth. ChangeType is sourced
	// from the spec, indexed by Key, to avoid drift if the slice order ever
	// diverges from the spec.
	changeTypeByKey := make(map[string]string, len(spec.Images))
	for _, entry := range spec.Images {
		changeTypeByKey[entry.Key] = entry.ChangeType
	}
	var (
		imageCount    int32
		upgradeCount  int32
		mutatedCount  int32
		injectedCount int32
	)
	for _, st := range newImages {
		imageCount++
		if st.UpgradeAvailable {
			upgradeCount++
		}
		switch changeTypeByKey[st.Key] {
		case "mutated":
			mutatedCount++
		case "injected":
			injectedCount++
		}
	}
	ir.Status.ImageCount = imageCount
	ir.Status.UpgradeAvailableCount = upgradeCount
	ir.Status.MutatedCount = mutatedCount
	ir.Status.InjectedCount = injectedCount
	ir.Status.ObservedGeneration = ir.GetGeneration()

	// Set the Ready condition.
	now := metav1.Now()
	readyCondition := metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonReconciled,
		Message:            ReconciledMessage,
		LastTransitionTime: now,
	}
	meta.SetStatusCondition(&ir.Status.Conditions, readyCondition)

	logger.V(1).Info("patching status", "images", imageCount, "upgrades", upgradeCount, "mutated", mutatedCount, "injected", injectedCount)

	if err := h.client.Status().Patch(ctx, ir, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch ImageRegistry status: %w", err)
	}

	// Update Prometheus gauges.
	portal := spec.PortalRef
	host := spec.Host
	ns := spec.Namespace
	metrics.ImageRegistryEntriesTotal.WithLabelValues(portal, host, ns).Set(float64(imageCount))
	metrics.ImageRegistryUpgradesTotal.WithLabelValues(portal, host, ns).Set(float64(upgradeCount))
	metrics.ImageRegistryMutatedTotal.WithLabelValues(portal, host, ns).Set(float64(mutatedCount))
	metrics.ImageRegistryInjectedTotal.WithLabelValues(portal, host, ns).Set(float64(injectedCount))

	return nil
}
