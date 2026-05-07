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

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// UpdateReadstoreHandler builds the full ImageView slice from Spec.Images merged
// with resolution results (for due images) and the previous Status (for non-due
// images), then calls ImageWriter.ReplaceForNamespace.
//
// When onlyWhenResolved is true the handler is a no-op if no resolutions were
// produced this cycle (used for the post-lookup pass).
type UpdateReadstoreHandler struct {
	imageStore       domainimage.ImageWriter
	onlyWhenResolved bool
}

// NewUpdateReadstoreHandler constructs an UpdateReadstoreHandler for the early
// pass (always runs, uses current status data only).
func NewUpdateReadstoreHandler(imageStore domainimage.ImageWriter) *UpdateReadstoreHandler {
	return &UpdateReadstoreHandler{imageStore: imageStore}
}

// NewUpdateReadstoreIfResolvedHandler constructs an UpdateReadstoreHandler for
// the post-lookup pass (skips when no resolutions were produced).
func NewUpdateReadstoreIfResolvedHandler(imageStore domainimage.ImageWriter) *UpdateReadstoreHandler {
	return &UpdateReadstoreHandler{imageStore: imageStore, onlyWhenResolved: true}
}

// Handle implements reconciler.Handler.
func (h *UpdateReadstoreHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, ChainData]) error {
	logger := log.FromContext(ctx).WithName("update-readstore")

	if h.onlyWhenResolved && len(rc.Data.Resolutions) == 0 {
		logger.V(2).Info("skipping readstore update: no new resolutions")
		return nil
	}

	spec := rc.Resource.Spec
	status := rc.Resource.Status

	// Build a status-by-key lookup for non-due images.
	statusByKey := make(map[string]sreportalv1alpha1.ImageRegistryStatusEntry, len(status.Images))
	for _, st := range status.Images {
		statusByKey[st.Key] = st
	}

	views := make([]domainimage.ImageView, 0, len(spec.Images))

	for _, entry := range spec.Images {
		view := domainimage.ImageView{
			PortalRef:  spec.PortalRef,
			Registry:   spec.Host,
			Repository: entry.Repository,
			Tag:        entry.OriginalTag,
			TagType:    domainimage.TagType(entry.TagType),

			OriginalImage: entry.OriginalImage,
			MutatedImage:  entry.MutatedImage,
			ChangeType:    entry.ChangeType,

			Workloads: toReadmodelWorkloads(entry.Workloads),
		}

		// Merge from resolution (for due images) or prior status (for non-due).
		if res, resolved := rc.Data.Resolutions[entry.Key]; resolved {
			view.LatestVersion = res.LatestVersion
			view.UpgradeAvailable = res.UpgradeAvailable
			view.LatestError = res.LastError
			t := res.LastCheckedAt
			view.LatestCheckedAt = &t
		} else if st, ok := statusByKey[entry.Key]; ok {
			// Preserve previously resolved state for non-due images.
			view.LatestVersion = st.LatestVersion
			view.UpgradeAvailable = st.UpgradeAvailable
			view.LatestError = st.LastError
			if st.LastCheckedAt != nil {
				t := st.LastCheckedAt.Time
				view.LatestCheckedAt = &t
			}
		}

		views = append(views, view)
	}

	logger.V(1).Info("updating readstore", "portal", spec.PortalRef, "host", spec.Host, "namespace", spec.Namespace, "entries", len(views))

	if err := h.imageStore.ReplaceForNamespace(ctx, spec.PortalRef, spec.Host, spec.Namespace, views); err != nil {
		return fmt.Errorf("replace readstore for (%s, %s, %s): %w", spec.PortalRef, spec.Host, spec.Namespace, err)
	}
	return nil
}

// toReadmodelWorkloads converts CRD workload refs to domain WorkloadRef.
func toReadmodelWorkloads(refs []sreportalv1alpha1.ImageRegistryWorkloadRef) []domainimage.WorkloadRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]domainimage.WorkloadRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, domainimage.WorkloadRef{
			Kind:      r.Kind,
			Namespace: r.Namespace,
			Name:      r.Name,
			Container: r.Container,
			Source:    domainimage.ContainerSourceSpec,
		})
	}
	return out
}
