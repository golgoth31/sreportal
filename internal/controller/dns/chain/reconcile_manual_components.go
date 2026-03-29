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

package dns

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

// ReconcileManualComponentsHandler creates, updates, or deletes an auto-managed
// Component CR based on the DNS CR-level sreportal.io/component annotation.
type ReconcileManualComponentsHandler struct {
	client client.Client
}

// NewReconcileManualComponentsHandler creates a new ReconcileManualComponentsHandler.
func NewReconcileManualComponentsHandler(c client.Client) *ReconcileManualComponentsHandler {
	return &ReconcileManualComponentsHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *ReconcileManualComponentsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DNS, ChainData]) error {
	resource := rc.Resource
	logger := log.FromContext(ctx).WithName("reconcile-manual-components")

	compSpec := adapter.ComponentAnnotationsFromMap(resource.Annotations)

	if compSpec == nil {
		// No component annotation — clean up any previously created component.
		return h.deleteManaged(ctx, resource)
	}

	// Verify the portal has status page enabled.
	var portal sreportalv1alpha1.Portal
	if err := h.client.Get(ctx, types.NamespacedName{
		Name:      resource.Spec.PortalRef,
		Namespace: resource.Namespace,
	}, &portal); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("portal not found, skipping component creation",
				"portal", resource.Spec.PortalRef)
			return nil
		}
		return fmt.Errorf("get portal %q: %w", resource.Spec.PortalRef, err)
	}
	if !portal.Spec.Features.IsStatusPageEnabled() {
		logger.V(1).Info("status page disabled, skipping component",
			"portal", resource.Spec.PortalRef)
		return nil
	}

	name := statuspage.GenerateCRName(resource.Spec.PortalRef, compSpec.DisplayName)
	nn := types.NamespacedName{Name: name, Namespace: resource.Namespace}

	status := sreportalv1alpha1.ComponentStatusValue(compSpec.Status)
	if status == "" {
		status = sreportalv1alpha1.ComponentStatusOperational
	}

	var existing sreportalv1alpha1.Component
	err := h.client.Get(ctx, nn, &existing)
	if apierrors.IsNotFound(err) {
		comp := &sreportalv1alpha1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: resource.Namespace,
				Labels: map[string]string{
					adapter.ManagedByLabelKey:   adapter.ManagedByDNSController,
					adapter.PortalAnnotationKey: resource.Spec.PortalRef,
				},
			},
			Spec: sreportalv1alpha1.ComponentSpec{
				DisplayName: compSpec.DisplayName,
				Group:       compSpec.Group,
				Description: compSpec.Description,
				Link:        compSpec.Link,
				PortalRef:   resource.Spec.PortalRef,
				Status:      status,
			},
		}
		if err := h.client.Create(ctx, comp); err != nil {
			return fmt.Errorf("create component %q: %w", name, err)
		}
		logger.Info("created component from DNS annotation", "name", name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get component %q: %w", name, err)
	}

	// Update metadata, never overwrite spec.status.
	existing.Spec.DisplayName = compSpec.DisplayName
	existing.Spec.Group = compSpec.Group
	existing.Spec.Description = compSpec.Description
	existing.Spec.Link = compSpec.Link

	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	existing.Labels[adapter.ManagedByLabelKey] = adapter.ManagedByDNSController
	existing.Labels[adapter.PortalAnnotationKey] = resource.Spec.PortalRef

	if err := h.client.Update(ctx, &existing); err != nil {
		return fmt.Errorf("update component %q: %w", name, err)
	}
	return nil
}

// deleteManaged removes any component managed by dns-controller for this DNS CR's portal.
func (h *ReconcileManualComponentsHandler) deleteManaged(ctx context.Context, resource *sreportalv1alpha1.DNS) error {
	var list sreportalv1alpha1.ComponentList
	if err := h.client.List(ctx, &list,
		client.InNamespace(resource.Namespace),
		client.MatchingLabels{
			adapter.ManagedByLabelKey:   adapter.ManagedByDNSController,
			adapter.PortalAnnotationKey: resource.Spec.PortalRef,
		},
	); err != nil {
		return fmt.Errorf("list dns-managed components: %w", err)
	}
	for i := range list.Items {
		if err := h.client.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete dns-managed component %q: %w", list.Items[i].Name, err)
		}
	}
	return nil
}
