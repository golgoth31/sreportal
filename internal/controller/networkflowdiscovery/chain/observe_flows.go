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
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	"github.com/golgoth31/sreportal/internal/flowobserver"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ObserveFlowsHandler enriches edges with the Used flag by querying a Prometheus-based
// FlowObserver. The observer is lazily initialized from a FlowObserver CRD matching
// the portal ref. When no FlowObserver CRD exists, this handler is a no-op.
// Edges already marked Used=true are never re-queried.
type ObserveFlowsHandler struct {
	k8sReader client.Reader

	mu       sync.Mutex
	observer domainnetpol.FlowObserver
	specHash string // tracks CRD changes to re-initialize
}

// NewObserveFlowsHandler creates a new ObserveFlowsHandler.
func NewObserveFlowsHandler(k8sReader client.Reader) *ObserveFlowsHandler {
	return &ObserveFlowsHandler{
		k8sReader: k8sReader,
	}
}

// Handle implements reconciler.Handler.
func (h *ObserveFlowsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, ChainData]) error {
	if rc.Resource.Spec.IsRemote {
		return nil
	}

	logger := log.FromContext(ctx).WithName("observe-flows")

	if len(rc.Data.Edges) == 0 {
		return nil
	}

	// Resolve or create the observer from the FlowObserver CRD.
	observer := h.resolveObserver(ctx, rc.Resource)
	if observer == nil {
		return nil
	}

	// Carry forward Used flag from the previous FlowEdgeSet.
	previousUsed := h.loadPreviousUsed(ctx, rc.Resource)

	var unknownEdges []domainnetpol.FlowEdge

	for i := range rc.Data.Edges {
		edge := &rc.Data.Edges[i]
		key := edge.From + "|" + edge.To + "|" + edge.EdgeType

		if previousUsed[key] {
			edge.Used = true
		}

		if !edge.Used {
			unknownEdges = append(unknownEdges, domainnetpol.FlowEdge{
				From: edge.From, To: edge.To, EdgeType: edge.EdgeType,
			})
		}
	}

	if len(unknownEdges) == 0 {
		logger.V(1).Info("all edges already marked as used, skipping observation")
		return nil
	}

	observed, err := observer.Observed(ctx, unknownEdges)
	if err != nil {
		logger.Error(err, "flow observer query failed, continuing without updates")
		return nil
	}

	updated := 0

	for i := range rc.Data.Edges {
		edge := &rc.Data.Edges[i]
		if edge.Used {
			continue
		}

		key := edge.From + "|" + edge.To + "|" + edge.EdgeType
		if observed[key] {
			edge.Used = true
			updated++
		}
	}

	logger.V(1).Info("flow observation complete",
		"total", len(rc.Data.Edges),
		"queried", len(unknownEdges),
		"updated", updated,
	)

	return nil
}

// resolveObserver finds the FlowObserver CRD for the portal and lazily creates the observer.
func (h *ObserveFlowsHandler) resolveObserver(ctx context.Context, nfd *sreportalv1alpha1.NetworkFlowDiscovery) domainnetpol.FlowObserver {
	var list sreportalv1alpha1.FlowObserverList
	if err := h.k8sReader.List(ctx, &list, client.InNamespace(nfd.Namespace)); err != nil {
		logr.FromContextOrDiscard(ctx).V(1).Info("could not list FlowObserver CRDs", "err", err)
		return nil
	}

	var fo *sreportalv1alpha1.FlowObserver

	for i := range list.Items {
		if list.Items[i].Spec.PortalRef == nfd.Spec.PortalRef {
			fo = &list.Items[i]
			break
		}
	}

	if fo == nil {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.observer != nil && h.specHash == fo.ResourceVersion {
		return h.observer
	}

	obs := flowobserver.Discover(ctx, logr.FromContextOrDiscard(ctx).WithName("observe-flows"), fo.Spec)
	h.observer = obs
	h.specHash = fo.ResourceVersion

	return obs
}

// loadPreviousUsed reads the existing FlowEdgeSet to recover Used flags.
func (h *ObserveFlowsHandler) loadPreviousUsed(ctx context.Context, nfd *sreportalv1alpha1.NetworkFlowDiscovery) map[string]bool {
	name := nfd.Name + "-edges"
	var edgeSet sreportalv1alpha1.FlowEdgeSet

	if err := h.k8sReader.Get(ctx, client.ObjectKey{Name: name, Namespace: nfd.Namespace}, &edgeSet); err != nil {
		if !errors.IsNotFound(err) {
			log.FromContext(ctx).V(1).Info("could not load previous FlowEdgeSet", "err", err)
		}

		return nil
	}

	result := make(map[string]bool, len(edgeSet.Status.Edges))
	for _, e := range edgeSet.Status.Edges {
		if e.Used {
			key := e.From + "|" + e.To + "|" + e.EdgeType
			result[key] = true
		}
	}

	return result
}
