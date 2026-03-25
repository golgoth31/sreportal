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

package networkflowdiscovery

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// ConditionTypeReady indicates the graph was successfully built.
	ConditionTypeReady = "Ready"
)

// UpdateStatusHandler writes the computed graph into FlowNodeSet and FlowEdgeSet CRDs,
// pushes the graph to the in-memory read store, and updates the parent NetworkFlowDiscovery status with counts.
type UpdateStatusHandler struct {
	client          client.Client
	flowGraphWriter domainnetpol.FlowGraphWriter
}

// NewUpdateStatusHandler creates a new UpdateStatusHandler.
func NewUpdateStatusHandler(c client.Client) *UpdateStatusHandler {
	return &UpdateStatusHandler{client: c}
}

// SetFlowGraphWriter sets the writer used to push graph data to the in-memory read store.
func (h *UpdateStatusHandler) SetFlowGraphWriter(w domainnetpol.FlowGraphWriter) {
	h.flowGraphWriter = w
}

// Handle implements reconciler.Handler.
func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, ChainData]) error {
	logger := log.FromContext(ctx).WithName("update-status")
	now := metav1.Now()
	nfd := rc.Resource
	ns := nfd.Namespace
	nfdName := nfd.Name

	// Ensure FlowNodeSet
	if err := h.ensureFlowNodeSet(ctx, nfd, ns, nfdName, rc.Data.Nodes); err != nil {
		return err
	}

	// Ensure FlowEdgeSet
	if err := h.ensureFlowEdgeSet(ctx, nfd, ns, nfdName, rc.Data.Edges); err != nil {
		return err
	}

	// Push graph to in-memory read store
	if h.flowGraphWriter != nil {
		portalRef := nfd.Spec.PortalRef
		if err := h.flowGraphWriter.ReplaceNodes(ctx, nfdName, portalRef, flowNodesToViews(rc.Data.Nodes)); err != nil {
			return fmt.Errorf("push nodes to read store: %w", err)
		}

		if err := h.flowGraphWriter.ReplaceEdges(ctx, nfdName, portalRef, flowEdgesToViews(rc.Data.Edges)); err != nil {
			return fmt.Errorf("push edges to read store: %w", err)
		}
	}

	// Update parent status
	base := nfd.DeepCopy()
	nfd.Status.NodeCount = len(rc.Data.Nodes)
	nfd.Status.EdgeCount = len(rc.Data.Edges)
	nfd.Status.LastReconcileTime = &now

	readyCondition := metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "ReconcileSucceeded",
		Message:            fmt.Sprintf("discovered %d nodes and %d edges", len(rc.Data.Nodes), len(rc.Data.Edges)),
		LastTransitionTime: now,
	}
	meta.SetStatusCondition(&nfd.Status.Conditions, readyCondition)

	logger.V(1).Info("updating status", "nodes", len(rc.Data.Nodes), "edges", len(rc.Data.Edges))

	if err := h.client.Status().Patch(ctx, nfd, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch NetworkFlowDiscovery status: %w", err)
	}

	return nil
}

func (h *UpdateStatusHandler) ensureFlowNodeSet(ctx context.Context, nfd *sreportalv1alpha1.NetworkFlowDiscovery, ns, nfdName string, nodes []sreportalv1alpha1.FlowNode) error {
	name := nfdName + "-nodes"
	var nodeSet sreportalv1alpha1.FlowNodeSet

	err := h.client.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, &nodeSet)
	if errors.IsNotFound(err) {
		nodeSet = sreportalv1alpha1.FlowNodeSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: sreportalv1alpha1.FlowNodeSetSpec{
				DiscoveryRef: nfdName,
			},
		}
		if err := ctrl.SetControllerReference(nfd, &nodeSet, h.client.Scheme()); err != nil {
			return fmt.Errorf("set owner reference on FlowNodeSet: %w", err)
		}
		if err := h.client.Create(ctx, &nodeSet); err != nil {
			return fmt.Errorf("create FlowNodeSet: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("get FlowNodeSet: %w", err)
	}

	base := nodeSet.DeepCopy()
	nodeSet.Status.Nodes = nodes

	if err := h.client.Status().Patch(ctx, &nodeSet, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch FlowNodeSet status: %w", err)
	}

	return nil
}

func (h *UpdateStatusHandler) ensureFlowEdgeSet(ctx context.Context, nfd *sreportalv1alpha1.NetworkFlowDiscovery, ns, nfdName string, edges []sreportalv1alpha1.FlowEdge) error {
	name := nfdName + "-edges"
	var edgeSet sreportalv1alpha1.FlowEdgeSet

	err := h.client.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, &edgeSet)
	if errors.IsNotFound(err) {
		edgeSet = sreportalv1alpha1.FlowEdgeSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: sreportalv1alpha1.FlowEdgeSetSpec{
				DiscoveryRef: nfdName,
			},
		}
		if err := ctrl.SetControllerReference(nfd, &edgeSet, h.client.Scheme()); err != nil {
			return fmt.Errorf("set owner reference on FlowEdgeSet: %w", err)
		}
		if err := h.client.Create(ctx, &edgeSet); err != nil {
			return fmt.Errorf("create FlowEdgeSet: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("get FlowEdgeSet: %w", err)
	}

	base := edgeSet.DeepCopy()
	edgeSet.Status.Edges = edges

	if err := h.client.Status().Patch(ctx, &edgeSet, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch FlowEdgeSet status: %w", err)
	}

	return nil
}

func flowNodesToViews(nodes []sreportalv1alpha1.FlowNode) []domainnetpol.FlowNode {
	views := make([]domainnetpol.FlowNode, len(nodes))
	for i, n := range nodes {
		views[i] = domainnetpol.FlowNode{
			ID:        n.ID,
			Label:     n.Label,
			Namespace: n.Namespace,
			NodeType:  n.NodeType,
			Group:     n.Group,
		}
	}

	return views
}

func flowEdgesToViews(edges []sreportalv1alpha1.FlowEdge) []domainnetpol.FlowEdge {
	views := make([]domainnetpol.FlowEdge, len(edges))
	for i, e := range edges {
		views[i] = domainnetpol.FlowEdge{
			From:     e.From,
			To:       e.To,
			EdgeType: e.EdgeType,
		}
	}

	return views
}
