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

// Package components provides a manager.Runnable that periodically reads
// EnrichedEndpoints from the SourceEndpointStore and reconciles auto-managed
// Component CRs derived from sreportal.io/component annotations.
package components

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/statuspage"
)

// portalIndex is a pre-computed lookup structure built from the portal list.
type portalIndex struct {
	Main   *sreportalv1alpha1.Portal
	ByName map[string]*sreportalv1alpha1.Portal
	Local  []*sreportalv1alpha1.Portal
}

// componentRequest is a request to create or update an auto-managed Component CR.
type componentRequest struct {
	PortalName  string
	DisplayName string
	Group       string
	Description string
	Link        string
	Status      string
}

// dedupeKey is used for deduplication of ComponentRequests.
type dedupeKey struct {
	portalName  string
	displayName string
}

// Reconciler is the consumer-side Runnable that periodically reconciles
// auto-managed Component CRs from the SourceEndpointStore.
type Reconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Reader   domainsource.SourceEndpointReader
	Interval time.Duration
}

var _ manager.Runnable = (*Reconciler)(nil)

// Start runs the consumer loop until ctx is cancelled. Each tick calls cycle.
func (r *Reconciler) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("components.reconciler")
	r.cycle(ctx)
	t := time.NewTicker(r.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			r.cycle(ctx)
			logger.V(2).Info("cycle complete")
		}
	}
}

// cycle lists all Portal CRs, collects ComponentRequests from the
// SourceEndpointStore via annotations, and reconciles desired Component CRs.
func (r *Reconciler) cycle(ctx context.Context) {
	logger := log.FromContext(ctx).WithName("components.cycle")

	// Build portal index.
	idx, err := r.buildPortalIndex(ctx)
	if err != nil {
		logger.Error(err, "failed to list Portal resources")
		return
	}
	if len(idx.Local) == 0 {
		logger.V(1).Info("no local portals found, skipping components cycle")
		return
	}

	// Collect ComponentRequests from all known source kinds.
	seen := make(map[dedupeKey]struct{})
	var requests []componentRequest

	for _, kind := range r.Reader.Kinds() {
		entries, err := r.Reader.Lookup(kind, "", "")
		if err != nil {
			logger.Error(err, "lookup failed", "kind", kind)
			continue
		}
		for _, entry := range entries {
			req := r.toComponentRequest(ctx, entry, idx)
			if req == nil {
				continue
			}
			key := dedupeKey{portalName: req.PortalName, displayName: req.DisplayName}
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			requests = append(requests, *req)
		}
	}

	// Build the desired set of Component CRs keyed by CR name.
	desired := make(map[string]componentRequest, len(requests))
	for _, req := range requests {
		portal := idx.ByName[req.PortalName]
		if portal == nil {
			continue
		}
		if !portal.Spec.Features.IsStatusPageEnabled() {
			logger.V(1).Info("skipping component: status page disabled",
				"portal", req.PortalName, "component", req.DisplayName)
			continue
		}
		name := statuspage.GenerateCRName(req.PortalName, req.DisplayName)
		desired[name] = req
	}

	// Create or update desired components.
	for name, req := range desired {
		portal := idx.ByName[req.PortalName]
		if err := r.reconcileComponent(ctx, portal, name, req); err != nil {
			logger.Error(err, "failed to reconcile component",
				"name", name, "portal", req.PortalName)
		}
	}

	// Delete orphaned auto-managed components.
	if err := r.deleteOrphans(ctx, idx, desired); err != nil {
		logger.Error(err, "failed to delete orphaned components")
	}
}

// toComponentRequest extracts a ComponentRequest from an EnrichedEndpoint's
// SourceAnnotations. Returns nil when the endpoint should be skipped.
func (r *Reconciler) toComponentRequest(
	ctx context.Context,
	entry domainsource.EnrichedEndpoint,
	idx *portalIndex,
) *componentRequest {
	logger := log.FromContext(ctx).WithName("components.cycle")

	annotations := entry.SourceAnnotations
	ca := adapter.ComponentAnnotationsFromMap(annotations)
	if ca == nil {
		return nil
	}

	// Resolve portal.
	portalName := annotations[adapter.PortalAnnotationKey]
	var portal *sreportalv1alpha1.Portal
	if portalName == "" {
		// No annotation — route to main portal.
		if idx.Main == nil {
			logger.V(1).Info("no main portal; skipping unannotated component source",
				"kind", entry.Kind, "name", entry.Name, "ns", entry.Namespace)
			return nil
		}
		portal = idx.Main
		portalName = portal.Name
	} else {
		portal = idx.ByName[portalName]
		if portal == nil {
			logger.V(1).Info("portal not found; skipping component source",
				"portal", portalName, "kind", entry.Kind, "name", entry.Name)
			return nil
		}
		// Skip remote portals.
		if portal.Spec.Remote != nil {
			logger.V(1).Info("skipping component source for remote portal",
				"portal", portalName, "kind", entry.Kind, "name", entry.Name)
			return nil
		}
		// Skip portals with DNS feature disabled.
		if !portal.Spec.Features.IsDNSEnabled() {
			logger.V(1).Info("skipping component source for DNS-disabled portal",
				"portal", portalName, "kind", entry.Kind, "name", entry.Name)
			return nil
		}
	}

	return &componentRequest{
		PortalName:  portalName,
		DisplayName: ca.DisplayName,
		Group:       ca.Group,
		Description: ca.Description,
		Link:        ca.Link,
		Status:      ca.Status,
	}
}

// buildPortalIndex lists all Portal CRs and returns a lookup index.
func (r *Reconciler) buildPortalIndex(ctx context.Context) (*portalIndex, error) {
	var portalList sreportalv1alpha1.PortalList
	if err := r.Client.List(ctx, &portalList); err != nil {
		return nil, err
	}

	idx := &portalIndex{
		ByName: make(map[string]*sreportalv1alpha1.Portal, len(portalList.Items)),
		Local:  make([]*sreportalv1alpha1.Portal, 0),
	}

	logger := log.FromContext(ctx).WithName("components.cycle")

	for i := range portalList.Items {
		p := &portalList.Items[i]
		idx.ByName[p.Name] = p

		if p.Spec.Remote != nil {
			logger.V(1).Info("skipping remote portal", "name", p.Name)
			continue
		}
		if !p.Spec.Features.IsDNSEnabled() {
			logger.V(1).Info("skipping portal with DNS feature disabled", "name", p.Name)
			continue
		}
		idx.Local = append(idx.Local, p)
		if p.Spec.Main {
			idx.Main = p
		}
	}

	return idx, nil
}

// reconcileComponent creates or updates a single auto-managed Component CR.
func (r *Reconciler) reconcileComponent(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	name string,
	req componentRequest,
) error {
	nn := types.NamespacedName{Name: name, Namespace: portal.Namespace}

	status := sreportalv1alpha1.ComponentStatusValue(req.Status)
	if status == "" {
		status = sreportalv1alpha1.ComponentStatusOperational
	}

	var existing sreportalv1alpha1.Component
	err := r.Client.Get(ctx, nn, &existing)
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
		if err := ctrl.SetControllerReference(portal, comp, r.Scheme); err != nil {
			return fmt.Errorf("set owner reference on component %q: %w", name, err)
		}
		if err := r.Client.Create(ctx, comp); err != nil {
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

	if err := r.Client.Update(ctx, &existing); err != nil {
		return fmt.Errorf("update component %q: %w", name, err)
	}
	return nil
}

// deleteOrphans removes auto-managed Component CRs that are no longer desired.
func (r *Reconciler) deleteOrphans(
	ctx context.Context,
	idx *portalIndex,
	desired map[string]componentRequest,
) error {
	for _, portal := range idx.Local {
		var list sreportalv1alpha1.ComponentList
		if err := r.Client.List(ctx, &list,
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
				if err := r.Client.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
					return fmt.Errorf("delete orphaned component %q: %w", list.Items[i].Name, err)
				}
			}
		}
	}
	return nil
}
