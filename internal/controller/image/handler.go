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

// Package image contains the event-driven image inventory controllers.
package image

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/log"
)

// WorkloadHandler owns the per-workload upsert/delete logic shared by every
// thin reconciler. It lists the ImageInventory CRs, filters them in-memory
// against the workload, and updates the store accordingly.
type WorkloadHandler struct {
	client client.Client
	store  domainimage.ImageWriter
}

// NewWorkloadHandler constructs a WorkloadHandler.
func NewWorkloadHandler(c client.Client, store domainimage.ImageWriter) *WorkloadHandler {
	return &WorkloadHandler{client: c, store: store}
}

// HandleUpsert updates the per-workload contribution of `wk` in every
// ImageInventory that selects it.
func (h *WorkloadHandler) HandleUpsert(
	ctx context.Context,
	wk domainimage.WorkloadKey,
	spec corev1.PodSpec,
	objLabels labels.Set,
) error {
	logger := log.FromContext(ctx).WithValues("workload", wk)

	var invList sreportalv1alpha1.ImageInventoryList
	if err := h.client.List(ctx, &invList); err != nil {
		return fmt.Errorf("list ImageInventory: %w", err)
	}

	var firstErr error
	for i := range invList.Items {
		inv := &invList.Items[i]
		if !matchesInventory(inv, wk, objLabels) {
			continue
		}
		images := imageViewsFromPodSpec(inv.Spec.PortalRef, wk.Kind, wk.Namespace, wk.Name, spec)
		if err := h.store.ReplaceWorkload(ctx, inv.Spec.PortalRef, wk, images); err != nil {
			logger.Error(err, "store.ReplaceWorkload failed", "portalRef", inv.Spec.PortalRef)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// HandleDelete removes this workload's contribution from every portal.
func (h *WorkloadHandler) HandleDelete(ctx context.Context, wk domainimage.WorkloadKey) error {
	return h.store.DeleteWorkloadAllPortals(ctx, wk)
}

// matchesInventory decides whether `wk` (with the given object labels) is in
// scope of `inv` according to watchedKinds, namespaceFilter and labelSelector.
func matchesInventory(inv *sreportalv1alpha1.ImageInventory, wk domainimage.WorkloadKey, objLabels labels.Set) bool {
	kinds := inv.Spec.EffectiveWatchedKinds()
	if !slices.Contains(kinds, sreportalv1alpha1.ImageInventoryKind(wk.Kind)) {
		return false
	}
	if inv.Spec.NamespaceFilter != "" && inv.Spec.NamespaceFilter != wk.Namespace {
		return false
	}
	if inv.Spec.LabelSelector != "" {
		sel, err := labels.Parse(inv.Spec.LabelSelector)
		if err != nil {
			// Fail-open: the CR was meant to be validated upstream; if a
			// malformed selector slipped through we treat it as "no filter"
			// so scans still happen rather than dropping silently.
			return true
		}
		if !sel.Matches(objLabels) {
			return false
		}
	}
	return true
}

// imageViewsFromPodSpec converts a PodSpec into the per-container ImageView
// projections contributed by one workload. (Previously lived in scanner.go.)
func imageViewsFromPodSpec(portalRef, kind, namespace, name string, spec corev1.PodSpec) []domainimage.ImageView {
	out := make([]domainimage.ImageView, 0, len(spec.Containers)+len(spec.InitContainers))
	appendContainer := func(c corev1.Container) {
		ref, err := domainimage.ParseReference(c.Image)
		if err != nil {
			return
		}
		out = append(out, domainimage.ImageView{
			PortalRef:  portalRef,
			Registry:   ref.Registry,
			Repository: ref.Repository,
			Tag:        ref.Tag,
			TagType:    ref.TagType,
			Workloads: []domainimage.WorkloadRef{{
				Kind:      kind,
				Namespace: namespace,
				Name:      name,
				Container: c.Name,
			}},
		})
	}
	for _, c := range spec.Containers {
		appendContainer(c)
	}
	for _, c := range spec.InitContainers {
		appendContainer(c)
	}
	return out
}

// ImageViewsFromPodSpec is an exported wrapper used by the ImageInventory
// chain's full-scan handler.
func ImageViewsFromPodSpec(portalRef, kind, namespace, name string, spec corev1.PodSpec) []domainimage.ImageView {
	return imageViewsFromPodSpec(portalRef, kind, namespace, name, spec)
}
