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

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ObserveFlowsHandler enriches edges with the Used flag from a FlowObserver provider.
// When the observer is nil or the resource is remote, this handler is a no-op.
// Edges already marked Used=true are never re-queried.
type ObserveFlowsHandler struct {
	k8sReader client.Reader
	observer  domainnetpol.FlowObserver
}

// NewObserveFlowsHandler creates a new ObserveFlowsHandler.
// observer may be nil (graceful degradation: Used stays false).
func NewObserveFlowsHandler(k8sReader client.Reader, observer domainnetpol.FlowObserver) *ObserveFlowsHandler {
	return &ObserveFlowsHandler{
		k8sReader: k8sReader,
		observer:  observer,
	}
}

// Handle implements reconciler.Handler.
func (h *ObserveFlowsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, ChainData]) error {
	if h.observer == nil {
		return nil
	}

	if rc.Resource.Spec.IsRemote {
		return nil
	}

	logger := log.FromContext(ctx).WithName("observe-flows")

	if len(rc.Data.Edges) == 0 {
		return nil
	}

	// Carry forward Used flag from the previous FlowEdgeSet.
	previousUsed := h.loadPreviousUsed(ctx, rc.Resource)

	var unknownEdges []domainnetpol.FlowEdge

	for i := range rc.Data.Edges {
		edge := &rc.Data.Edges[i]
		key := edge.From + "|" + edge.To + "|" + edge.EdgeType

		// Carry forward from previous reconciliation.
		if previousUsed[key] {
			edge.Used = true
		}

		// Only query edges not yet marked as used.
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

	// Query the observer for unknown edges.
	observed, err := h.observer.Observed(ctx, unknownEdges)
	if err != nil {
		logger.Error(err, "flow observer query failed, continuing without updates")
		return nil
	}

	// Merge results back into rc.Data.Edges.
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

// loadPreviousUsed reads the existing FlowEdgeSet to recover Used flags
// from the previous reconciliation cycle.
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
