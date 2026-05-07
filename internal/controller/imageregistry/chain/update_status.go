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

// UpdateStatusHandler recomputes summary counters from the current
// Status.Images, sets the Ready condition, updates ObservedGeneration, and
// refreshes Prometheus gauges.
//
// IMPORTANT: Status.Images is owned exclusively by
// ResolveLatestVersionsHandler.patchStatus. This handler MUST NOT mutate
// Status.Images — UpdateStatusHandler runs every reconcile while the resolve
// goroutine runs once per checkInterval, and both use client.MergeFrom which
// sends the full slice on diff. Mutating Status.Images here would clobber
// resolutions written concurrently by the goroutine.
//
// Counters are computed from ir.Status.Images as-is. New spec entries that
// have not yet been seen by patchStatus will not contribute to imageCount
// until the next resolve cycle materialises their placeholder. This brief
// inconsistency is preferred over racing on the slice.
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

	// ChangeType is sourced from the spec, indexed by Key, so counters stay
	// correct even if Status.Images and Spec.Images diverge transiently.
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
	for _, st := range ir.Status.Images {
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

	portal := spec.PortalRef
	host := spec.Host
	ns := spec.Namespace
	metrics.ImageRegistryEntriesTotal.WithLabelValues(portal, host, ns).Set(float64(imageCount))
	metrics.ImageRegistryUpgradesTotal.WithLabelValues(portal, host, ns).Set(float64(upgradeCount))
	metrics.ImageRegistryMutatedTotal.WithLabelValues(portal, host, ns).Set(float64(mutatedCount))
	metrics.ImageRegistryInjectedTotal.WithLabelValues(portal, host, ns).Set(float64(injectedCount))

	return nil
}
