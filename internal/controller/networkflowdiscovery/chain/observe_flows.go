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
	"sort"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	defaultStaleAfter = 5 * time.Minute
	defaultRetryAfter = 1 * time.Hour
	defaultMaxBatch   = 50
)

// ObserveFlowsHandler enriches edges with LastSeen timestamps from a FlowObserver provider.
// When the observer is nil or the resource is remote, this handler is a no-op.
//
// To avoid thundering-herd effects, edges are processed in batches and use two thresholds:
//   - staleAfter: how long before a previously-seen edge is re-queried (default 5m)
//   - retryAfter: how long before an edge with no result is re-queried (default 1h)
type ObserveFlowsHandler struct {
	k8sReader  client.Reader
	observer   domainnetpol.FlowObserver
	staleAfter time.Duration
	retryAfter time.Duration
	maxBatch   int

	// In-memory tracker for edges that were queried but returned no result.
	// Lost on operator restart — acceptable since it only affects query frequency.
	missedMu sync.Mutex
	missedAt map[string]time.Time // edge key → last time queried without result
}

// NewObserveFlowsHandler creates a new ObserveFlowsHandler.
// observer may be nil (graceful degradation: no timestamps populated).
func NewObserveFlowsHandler(k8sReader client.Reader, observer domainnetpol.FlowObserver, staleAfter time.Duration) *ObserveFlowsHandler {
	if staleAfter <= 0 {
		staleAfter = defaultStaleAfter
	}

	return &ObserveFlowsHandler{
		k8sReader:  k8sReader,
		observer:   observer,
		staleAfter: staleAfter,
		retryAfter: defaultRetryAfter,
		maxBatch:   defaultMaxBatch,
		missedAt:   make(map[string]time.Time),
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

	// Carry forward existing LastSeen from the previous FlowEdgeSet.
	previousLastSeen := h.loadPreviousLastSeen(ctx, rc.Resource)

	now := time.Now()

	// Candidate edges that need observation, with a priority score (older = higher priority).
	type candidate struct {
		index int // index in rc.Data.Edges
		edge  domainnetpol.FlowEdge
		age   time.Duration // time since last seen or last check
	}

	var candidates []candidate

	h.missedMu.Lock()

	for i := range rc.Data.Edges {
		edge := &rc.Data.Edges[i]
		key := edge.From + "|" + edge.To + "|" + edge.EdgeType

		// Carry forward the previous timestamp.
		if prev, ok := previousLastSeen[key]; ok && edge.LastSeen == nil {
			edge.LastSeen = &prev
		}

		// Determine if this edge needs observation.
		if edge.LastSeen != nil {
			// Edge has been seen before — re-query if stale.
			if age := now.Sub(edge.LastSeen.Time); age > h.staleAfter {
				candidates = append(candidates, candidate{index: i, edge: domainnetpol.FlowEdge{
					From: edge.From, To: edge.To, EdgeType: edge.EdgeType,
				}, age: age})
			}
		} else {
			// Edge never seen — check if we already queried recently without result.
			if missed, ok := h.missedAt[key]; ok && now.Sub(missed) < h.retryAfter {
				continue // Skip — we already tried recently and got nothing.
			}

			candidates = append(candidates, candidate{index: i, edge: domainnetpol.FlowEdge{
				From: edge.From, To: edge.To, EdgeType: edge.EdgeType,
			}, age: now.Sub(time.Time{})}) // max age — highest priority
		}
	}

	h.missedMu.Unlock()

	if len(candidates) == 0 {
		logger.V(1).Info("all edges have fresh timestamps, skipping observation")
		return nil
	}

	// Sort by age descending (oldest first = highest priority) and cap at maxBatch.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].age > candidates[j].age
	})

	if len(candidates) > h.maxBatch {
		candidates = candidates[:h.maxBatch]
	}

	// Build the query batch.
	queryEdges := make([]domainnetpol.FlowEdge, len(candidates))
	for i, c := range candidates {
		queryEdges[i] = c.edge
	}

	// Query the observer.
	observed, err := h.observer.LastSeen(ctx, queryEdges)
	if err != nil {
		logger.Error(err, "flow observer query failed, continuing without timestamps")
		return nil
	}

	// Merge results and update missed tracker.
	updated := 0

	h.missedMu.Lock()

	for _, c := range candidates {
		key := c.edge.From + "|" + c.edge.To + "|" + c.edge.EdgeType

		if t, ok := observed[key]; ok {
			mt := metav1.NewTime(t)
			rc.Data.Edges[c.index].LastSeen = &mt
			delete(h.missedAt, key) // Got a result — clear the miss tracker.
			updated++
		} else {
			h.missedAt[key] = now // No result — track so we don't retry for retryAfter.
		}
	}

	h.missedMu.Unlock()

	logger.V(1).Info("flow observation complete",
		"total", len(rc.Data.Edges),
		"queried", len(candidates),
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
