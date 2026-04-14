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
	"time"

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
// The query frequency is controlled by spec.reconcileInterval in the FlowObserver CRD.
type ObserveFlowsHandler struct {
	k8sReader client.Reader

	mu                 sync.Mutex
	observer           domainnetpol.FlowObserver
	specHash           string        // tracks CRD changes to re-initialize
	reconcileInterval  time.Duration // from CRD spec
	lastQueryTime      time.Time     // when we last queried the observer
	evaluatedEdgeTypes map[string]bool
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

	// Throttle: skip query if the last one was too recent.
	h.mu.Lock()
	sinceLastQuery := time.Since(h.lastQueryTime)
	interval := h.reconcileInterval
	h.mu.Unlock()

	// Build the set of evaluated edge types (under lock since resolveObserver sets it).
	h.mu.Lock()
	evalTypes := h.evaluatedEdgeTypes
	h.mu.Unlock()

	if interval > 0 && sinceLastQuery < interval {
		logger.V(1).Info("skipping observation, next query in", "remaining", interval-sinceLastQuery)
		// Still carry forward Used/Evaluated flags even when skipping the query.
		prev := h.loadPreviousFlags(ctx, rc.Resource)
		for i := range rc.Data.Edges {
			edge := &rc.Data.Edges[i]
			key := edge.From + "|" + edge.To + "|" + edge.EdgeType
			if p, ok := prev[key]; ok {
				edge.Used = edge.Used || p.Used
				edge.Evaluated = edge.Evaluated || p.Evaluated
			}
			if !edge.Evaluated {
				edge.Evaluated = h.isEvaluable(edge, evalTypes)
			}
		}

		return nil
	}

	// Carry forward Used/Evaluated flags from the previous FlowEdgeSet.
	prev := h.loadPreviousFlags(ctx, rc.Resource)

	var unknownEdges []domainnetpol.FlowEdge

	for i := range rc.Data.Edges {
		edge := &rc.Data.Edges[i]
		key := edge.From + "|" + edge.To + "|" + edge.EdgeType

		if p, ok := prev[key]; ok {
			edge.Used = edge.Used || p.Used
			edge.Evaluated = edge.Evaluated || p.Evaluated
		}

		// Mark evaluable edges.
		edge.Evaluated = h.isEvaluable(edge, evalTypes)

		if !edge.Evaluated || edge.Used {
			continue
		}

		unknownEdges = append(unknownEdges, domainnetpol.FlowEdge{
			From: edge.From, To: edge.To, EdgeType: edge.EdgeType,
		})
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

	// Update last query time after successful query.
	h.mu.Lock()
	h.lastQueryTime = time.Now()
	h.mu.Unlock()

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

	// Parse reconcile interval from CRD spec.
	if fo.Spec.ReconcileInterval != "" {
		if d, err := time.ParseDuration(fo.Spec.ReconcileInterval); err == nil && d > 0 {
			h.reconcileInterval = d
		}
	}

	// Store evaluated edge types.
	evalTypes := fo.Spec.EvaluatedEdgeTypes
	h.evaluatedEdgeTypes = make(map[string]bool, len(evalTypes))
	for _, t := range evalTypes {
		h.evaluatedEdgeTypes[t] = true
	}

	return obs
}

// isEvaluable returns true if both source and destination node types are in the evaluated set.
func (h *ObserveFlowsHandler) isEvaluable(edge *sreportalv1alpha1.FlowEdge, evalTypes map[string]bool) bool {
	if len(evalTypes) == 0 {
		return false
	}

	srcType, _, _ := flowobserver.ParseNodeID(edge.From)
	dstType, _, _ := flowobserver.ParseNodeID(edge.To)

	return evalTypes[srcType] && evalTypes[dstType]
}

type previousFlags struct {
	Used      bool
	Evaluated bool
}

// loadPreviousFlags reads the existing FlowEdgeSet to recover Used and Evaluated flags.
func (h *ObserveFlowsHandler) loadPreviousFlags(ctx context.Context, nfd *sreportalv1alpha1.NetworkFlowDiscovery) map[string]previousFlags {
	name := nfd.Name + "-edges"
	var edgeSet sreportalv1alpha1.FlowEdgeSet

	if err := h.k8sReader.Get(ctx, client.ObjectKey{Name: name, Namespace: nfd.Namespace}, &edgeSet); err != nil {
		if !errors.IsNotFound(err) {
			log.FromContext(ctx).V(1).Info("could not load previous FlowEdgeSet", "err", err)
		}

		return nil
	}

	result := make(map[string]previousFlags, len(edgeSet.Status.Edges))
	for _, e := range edgeSet.Status.Edges {
		if e.Used || e.Evaluated {
			key := e.From + "|" + e.To + "|" + e.EdgeType
			result[key] = previousFlags{Used: e.Used, Evaluated: e.Evaluated}
		}
	}

	return result
}
