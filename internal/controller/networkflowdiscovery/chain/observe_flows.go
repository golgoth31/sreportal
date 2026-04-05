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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ObserveFlowsHandler enriches edges with LastSeen timestamps from a FlowObserver provider.
// When the observer is nil or the resource is remote, this handler is a no-op.
type ObserveFlowsHandler struct {
	k8sReader  client.Reader
	observer   domainnetpol.FlowObserver
	staleAfter time.Duration
}

// NewObserveFlowsHandler creates a new ObserveFlowsHandler.
// observer may be nil (graceful degradation: no timestamps populated).
func NewObserveFlowsHandler(k8sReader client.Reader, observer domainnetpol.FlowObserver, staleAfter time.Duration) *ObserveFlowsHandler {
	if staleAfter <= 0 {
		staleAfter = 1 * time.Hour
	}

	return &ObserveFlowsHandler{
		k8sReader:  k8sReader,
		observer:   observer,
		staleAfter: staleAfter,
	}
}

// Handle implements reconciler.Handler.
func (h *ObserveFlowsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, ChainData]) error {
	// No observer available — no-op.
	if h.observer == nil {
		return nil
	}

	// Remote resources get LastSeen from the remote portal gRPC response.
	if rc.Resource.Spec.IsRemote {
		return nil
	}

	logger := log.FromContext(ctx).WithName("observe-flows")

	if len(rc.Data.Edges) == 0 {
		return nil
	}

	// Carry forward existing LastSeen from the previous FlowEdgeSet.
	previousLastSeen := h.loadPreviousLastSeen(ctx, rc.Resource)

	now := time.Now()
	var staleEdges []domainnetpol.FlowEdge

	for i := range rc.Data.Edges {
		edge := &rc.Data.Edges[i]
		key := edge.From + "|" + edge.To + "|" + edge.EdgeType

		// Carry forward the previous timestamp.
		if prev, ok := previousLastSeen[key]; ok && edge.LastSeen == nil {
			edge.LastSeen = &prev
		}

		// Collect edges that need observation.
		if edge.LastSeen == nil || now.Sub(edge.LastSeen.Time) > h.staleAfter {
			staleEdges = append(staleEdges, domainnetpol.FlowEdge{
				From: edge.From, To: edge.To, EdgeType: edge.EdgeType,
			})
		}
	}

	if len(staleEdges) == 0 {
		logger.V(1).Info("all edges have fresh timestamps, skipping observation")
		return nil
	}

	// Query the observer for stale edges.
	observed, err := h.observer.LastSeen(ctx, staleEdges)
	if err != nil {
		logger.Error(err, "flow observer query failed, continuing without timestamps")
		return nil // Don't fail the chain.
	}

	// Merge observed timestamps back into rc.Data.Edges.
	updated := 0

	for i := range rc.Data.Edges {
		edge := &rc.Data.Edges[i]
		key := edge.From + "|" + edge.To + "|" + edge.EdgeType

		if t, ok := observed[key]; ok {
			mt := metav1.NewTime(t)
			edge.LastSeen = &mt
			updated++
		}
	}

	logger.V(1).Info("flow observation complete",
		"total", len(rc.Data.Edges),
		"queried", len(staleEdges),
		"updated", updated,
	)

	return nil
}

// loadPreviousLastSeen reads the existing FlowEdgeSet to recover LastSeen timestamps
// from the previous reconciliation cycle.
func (h *ObserveFlowsHandler) loadPreviousLastSeen(ctx context.Context, nfd *sreportalv1alpha1.NetworkFlowDiscovery) map[string]metav1.Time {
	name := nfd.Name + "-edges"
	var edgeSet sreportalv1alpha1.FlowEdgeSet

	if err := h.k8sReader.Get(ctx, client.ObjectKey{Name: name, Namespace: nfd.Namespace}, &edgeSet); err != nil {
		if !errors.IsNotFound(err) {
			log.FromContext(ctx).V(1).Info("could not load previous FlowEdgeSet", "err", err)
		}

		return nil
	}

	result := make(map[string]metav1.Time, len(edgeSet.Status.Edges))
	for _, e := range edgeSet.Status.Edges {
		if e.LastSeen != nil {
			key := e.From + "|" + e.To + "|" + e.EdgeType
			result[key] = *e.LastSeen
		}
	}

	return result
}
