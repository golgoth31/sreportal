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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/statuspage"
)

// ReconcileComponentsHandler creates, updates, or deletes auto-managed
// Component CRs based on ComponentRequests collected from annotated endpoints.
type ReconcileComponentsHandler struct {
	client client.Client
}

// NewReconcileComponentsHandler creates a new ReconcileComponentsHandler.
func NewReconcileComponentsHandler(c client.Client) *ReconcileComponentsHandler {
	return &ReconcileComponentsHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *ReconcileComponentsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[struct{}, ChainData]) error {
	if rc.Data.Index == nil {
		return nil
	}

	logger := log.FromContext(ctx).WithName("reconcile-components")

	// Build the set of desired components keyed by CR name for fast lookup.
	desired := make(map[string]ComponentRequest, len(rc.Data.ComponentRequests))
	for _, req := range rc.Data.ComponentRequests {
		portal := rc.Data.Index.ByName[req.PortalName]
		if portal == nil {
			continue
		}
		if !portal.Spec.Features.IsStatusPageEnabled() {
			logger.V(1).Info("skipping component: status page disabled",
				"portal", req.PortalName, "component", req.DisplayName)
			continue
		}
		name := componentCRName(req.PortalName, req.DisplayName)
		desired[name] = req
	}

	// Create or update desired components.
	for name, req := range desired {
		portal := rc.Data.Index.ByName[req.PortalName]
		if err := h.reconcileComponent(ctx, portal, name, req); err != nil {
			logger.Error(err, "failed to reconcile component",
				"name", name, "portal", req.PortalName)
		}
	}

	// Delete orphaned auto-managed components.
	if err := h.deleteOrphans(ctx, rc.Data.Index, desired); err != nil {
		logger.Error(err, "failed to delete orphaned components")
	}

	return nil
}

func (h *ReconcileComponentsHandler) reconcileComponent(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	name string,
	req ComponentRequest,
) error {
	nn := types.NamespacedName{Name: name, Namespace: portal.Namespace}

	status := sreportalv1alpha1.ComponentStatusValue(req.Status)
	if status == "" {
		status = sreportalv1alpha1.ComponentStatusOperational
	}

	var existing sreportalv1alpha1.Component
	err := h.client.Get(ctx, nn, &existing)
	if apierrors.IsNotFound(err) {
		comp := &sreportalv1alpha1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: portal.Namespace,
				Labels: map[string]string{
					adapter.ManagedByLabelKey:   adapter.ManagedBySourceController,
					adapter.PortalAnnotationKey: portal.Name,
				},
			},
			Spec: sreportalv1alpha1.ComponentSpec{
				DisplayName: req.DisplayName,
				Group:       req.Group,
				Description: req.Description,
				Link:        req.Link,
				PortalRef:   portal.Name,
				Status:      status,
			},
		}
		if err := h.client.Create(ctx, comp); err != nil {
			return fmt.Errorf("create component %q: %w", name, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get component %q: %w", name, err)
	}

	// Update metadata from annotation, but never overwrite spec.status.
	existing.Spec.DisplayName = req.DisplayName
	existing.Spec.Group = req.Group
	existing.Spec.Description = req.Description
	existing.Spec.Link = req.Link

	// Ensure labels are set.
	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	existing.Labels[adapter.ManagedByLabelKey] = adapter.ManagedBySourceController
	existing.Labels[adapter.PortalAnnotationKey] = portal.Name

	if err := h.client.Update(ctx, &existing); err != nil {
		return fmt.Errorf("update component %q: %w", name, err)
	}
	return nil
}

// deleteOrphans removes auto-managed components (managed-by=source-controller)
// that are no longer in the desired set.
func (h *ReconcileComponentsHandler) deleteOrphans(
	ctx context.Context,
	idx *PortalIndex,
	desired map[string]ComponentRequest,
) error {
	for _, portal := range idx.Local {
		var list sreportalv1alpha1.ComponentList
		if err := h.client.List(ctx, &list,
			client.InNamespace(portal.Namespace),
			client.MatchingLabels{
				adapter.ManagedByLabelKey:   adapter.ManagedBySourceController,
				adapter.PortalAnnotationKey: portal.Name,
			},
		); err != nil {
			return fmt.Errorf("list components for portal %q: %w", portal.Name, err)
		}

		for i := range list.Items {
			if _, stillDesired := desired[list.Items[i].Name]; !stillDesired {
				if err := h.client.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
					return fmt.Errorf("delete orphaned component %q: %w", list.Items[i].Name, err)
				}
			}
		}
	}
	return nil
}

// componentCRName generates a deterministic CR name from portalRef and displayName.
func componentCRName(portalRef, displayName string) string {
	return statuspage.GenerateCRName(portalRef, displayName)
}
