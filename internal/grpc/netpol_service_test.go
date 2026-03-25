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

package grpc_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	netpolv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
)

// --- helpers ---

func makeFlowNodeSet(name, discoveryRef string, nodes []sreportalv1alpha1.FlowNode) *sreportalv1alpha1.FlowNodeSet {
	return &sreportalv1alpha1.FlowNodeSet{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       sreportalv1alpha1.FlowNodeSetSpec{DiscoveryRef: discoveryRef},
		Status:     sreportalv1alpha1.FlowNodeSetStatus{Nodes: nodes},
	}
}

func makeFlowEdgeSet(name, discoveryRef string, edges []sreportalv1alpha1.FlowEdge) *sreportalv1alpha1.FlowEdgeSet {
	return &sreportalv1alpha1.FlowEdgeSet{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       sreportalv1alpha1.FlowEdgeSetSpec{DiscoveryRef: discoveryRef},
		Status:     sreportalv1alpha1.FlowEdgeSetStatus{Edges: edges},
	}
}

func makeNFD(name, portalRef string) *sreportalv1alpha1.NetworkFlowDiscovery {
	return &sreportalv1alpha1.NetworkFlowDiscovery{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       sreportalv1alpha1.NetworkFlowDiscoverySpec{PortalRef: portalRef},
	}
}

func nodeIDs(nodes []*netpolv1.NetpolNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.Id
	}
	return ids
}

func edgePairs(edges []*netpolv1.NetpolEdge) []string {
	pairs := make([]string, len(edges))
	for i, e := range edges {
		pairs[i] = e.From + "->" + e.To
	}
	return pairs
}

// --- tests ---

func TestListNetworkPolicies_ReturnsAllNodesAndEdges_WhenNoFilter(t *testing.T) {
	scheme := newScheme(t)

	nodeSet := makeFlowNodeSet("ns1", "nfd-main", []sreportalv1alpha1.FlowNode{
		{ID: "svc:core:api", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"},
		{ID: "svc:core:web", Label: "web", Namespace: "core", NodeType: "service", Group: "Core"},
	})
	edgeSet := makeFlowEdgeSet("es1", "nfd-main", []sreportalv1alpha1.FlowEdge{
		{From: "svc:core:web", To: "svc:core:api", EdgeType: "internal"},
	})

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(nodeSet, edgeSet).
		WithStatusSubresource(nodeSet, edgeSet).
		Build()

	svc := svcgrpc.NewNetworkPolicyService(k8sClient)

	resp, err := svc.ListNetworkPolicies(
		context.Background(),
		connect.NewRequest(&netpolv1.ListNetworkPoliciesRequest{}),
	)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Nodes, 2)
	assert.Len(t, resp.Msg.Edges, 1)
	assert.Contains(t, nodeIDs(resp.Msg.Nodes), "svc:core:api")
	assert.Contains(t, nodeIDs(resp.Msg.Nodes), "svc:core:web")
	assert.Contains(t, edgePairs(resp.Msg.Edges), "svc:core:web->svc:core:api")
}

func TestListNetworkPolicies_DeduplicatesNodesAndEdges_AcrossMultipleSets(t *testing.T) {
	scheme := newScheme(t)

	ns1 := makeFlowNodeSet("ns1", "nfd-a", []sreportalv1alpha1.FlowNode{
		{ID: "svc:core:api", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"},
	})
	ns2 := makeFlowNodeSet("ns2", "nfd-b", []sreportalv1alpha1.FlowNode{
		{ID: "svc:core:api", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"},
		{ID: "svc:core:web", Label: "web", Namespace: "core", NodeType: "service", Group: "Core"},
	})
	es1 := makeFlowEdgeSet("es1", "nfd-a", []sreportalv1alpha1.FlowEdge{
		{From: "svc:core:web", To: "svc:core:api", EdgeType: "internal"},
	})
	es2 := makeFlowEdgeSet("es2", "nfd-b", []sreportalv1alpha1.FlowEdge{
		{From: "svc:core:web", To: "svc:core:api", EdgeType: "internal"},
	})

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ns1, ns2, es1, es2).
		WithStatusSubresource(ns1, ns2, es1, es2).
		Build()

	svc := svcgrpc.NewNetworkPolicyService(k8sClient)

	resp, err := svc.ListNetworkPolicies(
		context.Background(),
		connect.NewRequest(&netpolv1.ListNetworkPoliciesRequest{}),
	)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Nodes, 2, "duplicate node should be deduplicated")
	assert.Len(t, resp.Msg.Edges, 1, "duplicate edge should be deduplicated")
}

func TestListNetworkPolicies_FiltersByPortal(t *testing.T) {
	scheme := newScheme(t)

	nfdMain := makeNFD("nfd-main", "main")
	nfdOther := makeNFD("nfd-other", "other")

	nsMain := makeFlowNodeSet("ns-main", "nfd-main", []sreportalv1alpha1.FlowNode{
		{ID: "svc:core:api", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"},
	})
	nsOther := makeFlowNodeSet("ns-other", "nfd-other", []sreportalv1alpha1.FlowNode{
		{ID: "svc:pay:stripe", Label: "stripe", Namespace: "pay", NodeType: "external", Group: "Pay"},
	})
	esMain := makeFlowEdgeSet("es-main", "nfd-main", []sreportalv1alpha1.FlowEdge{})
	esOther := makeFlowEdgeSet("es-other", "nfd-other", []sreportalv1alpha1.FlowEdge{})

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(nfdMain, nfdOther, nsMain, nsOther, esMain, esOther).
		WithStatusSubresource(nsMain, nsOther, esMain, esOther).
		WithIndex(&sreportalv1alpha1.NetworkFlowDiscovery{}, "spec.portalRef", func(o client.Object) []string {
			nfd := o.(*sreportalv1alpha1.NetworkFlowDiscovery)
			if nfd.Spec.PortalRef == "" {
				return nil
			}
			return []string{nfd.Spec.PortalRef}
		}).
		Build()

	svc := svcgrpc.NewNetworkPolicyService(k8sClient)

	resp, err := svc.ListNetworkPolicies(
		context.Background(),
		connect.NewRequest(&netpolv1.ListNetworkPoliciesRequest{Portal: "main"}),
	)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Nodes, 1)
	assert.Equal(t, "svc:core:api", resp.Msg.Nodes[0].Id)
}

func TestListNetworkPolicies_FiltersByNamespace(t *testing.T) {
	scheme := newScheme(t)

	nodeSet := makeFlowNodeSet("ns1", "nfd-main", []sreportalv1alpha1.FlowNode{
		{ID: "svc:core:api", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"},
		{ID: "svc:pay:stripe", Label: "stripe", Namespace: "pay", NodeType: "external", Group: "Pay"},
	})
	edgeSet := makeFlowEdgeSet("es1", "nfd-main", []sreportalv1alpha1.FlowEdge{
		{From: "svc:core:api", To: "svc:pay:stripe", EdgeType: "cross-pl"},
	})

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(nodeSet, edgeSet).
		WithStatusSubresource(nodeSet, edgeSet).
		Build()

	svc := svcgrpc.NewNetworkPolicyService(k8sClient)

	resp, err := svc.ListNetworkPolicies(
		context.Background(),
		connect.NewRequest(&netpolv1.ListNetworkPoliciesRequest{Namespace: "core"}),
	)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Nodes, 1)
	assert.Equal(t, "svc:core:api", resp.Msg.Nodes[0].Id)
	assert.Empty(t, resp.Msg.Edges, "cross-namespace edge should be pruned")
}

func TestListNetworkPolicies_SearchMatchesLabelGroupAndNamespace(t *testing.T) {
	scheme := newScheme(t)

	nodeSet := makeFlowNodeSet("ns1", "nfd-main", []sreportalv1alpha1.FlowNode{
		{ID: "svc:core:api", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"},
		{ID: "svc:pay:stripe", Label: "stripe", Namespace: "pay", NodeType: "external", Group: "Pay"},
		{ID: "svc:core:web", Label: "web", Namespace: "core", NodeType: "service", Group: "Core"},
	})

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(nodeSet).
		WithStatusSubresource(nodeSet).
		Build()

	svc := svcgrpc.NewNetworkPolicyService(k8sClient)

	cases := []struct {
		name    string
		search  string
		wantIDs []string
	}{
		{
			name:    "match by label",
			search:  "stripe",
			wantIDs: []string{"svc:pay:stripe"},
		},
		{
			name:    "match by group",
			search:  "Pay",
			wantIDs: []string{"svc:pay:stripe"},
		},
		{
			name:    "match by namespace",
			search:  "core",
			wantIDs: []string{"svc:core:api", "svc:core:web"},
		},
		{
			name:    "case insensitive",
			search:  "STRIPE",
			wantIDs: []string{"svc:pay:stripe"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := svc.ListNetworkPolicies(
				context.Background(),
				connect.NewRequest(&netpolv1.ListNetworkPoliciesRequest{Search: tc.search}),
			)
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.wantIDs, nodeIDs(resp.Msg.Nodes))
		})
	}
}

func TestListNetworkPolicies_SearchExpandsToDirectNeighbors(t *testing.T) {
	// Graph: A -> B -> C
	// Search for "api" (matches A only).
	// Expected: A and B (1-hop neighbor), NOT C.
	scheme := newScheme(t)

	nodeSet := makeFlowNodeSet("ns1", "nfd-main", []sreportalv1alpha1.FlowNode{
		{ID: "a", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"},
		{ID: "b", Label: "backend", Namespace: "core", NodeType: "service", Group: "Core"},
		{ID: "c", Label: "cache", Namespace: "core", NodeType: "database", Group: "Core"},
	})
	edgeSet := makeFlowEdgeSet("es1", "nfd-main", []sreportalv1alpha1.FlowEdge{
		{From: "a", To: "b", EdgeType: "internal"},
		{From: "b", To: "c", EdgeType: "internal"},
	})

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(nodeSet, edgeSet).
		WithStatusSubresource(nodeSet, edgeSet).
		Build()

	svc := svcgrpc.NewNetworkPolicyService(k8sClient)

	resp, err := svc.ListNetworkPolicies(
		context.Background(),
		connect.NewRequest(&netpolv1.ListNetworkPoliciesRequest{Search: "api"}),
	)

	require.NoError(t, err)
	ids := nodeIDs(resp.Msg.Nodes)
	assert.Contains(t, ids, "a", "direct match should be included")
	assert.Contains(t, ids, "b", "1-hop neighbor should be included")
	assert.NotContains(t, ids, "c", "2-hop neighbor should NOT be included")
	assert.Len(t, resp.Msg.Edges, 1, "only A->B edge should remain")
	assert.Equal(t, "a", resp.Msg.Edges[0].From)
	assert.Equal(t, "b", resp.Msg.Edges[0].To)
}

func TestListNetworkPolicies_EmptyState_ReturnsEmptyGraph(t *testing.T) {
	scheme := newScheme(t)

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	svc := svcgrpc.NewNetworkPolicyService(k8sClient)

	resp, err := svc.ListNetworkPolicies(
		context.Background(),
		connect.NewRequest(&netpolv1.ListNetworkPoliciesRequest{}),
	)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Nodes)
	assert.Empty(t, resp.Msg.Edges)
}

func TestListNetworkPolicies_SortOrder_IsDeterministic(t *testing.T) {
	scheme := newScheme(t)

	nodeSet := makeFlowNodeSet("ns1", "nfd-main", []sreportalv1alpha1.FlowNode{
		{ID: "svc:pay:z", Label: "z-svc", Namespace: "pay", NodeType: "service", Group: "Pay"},
		{ID: "svc:core:a", Label: "a-svc", Namespace: "core", NodeType: "service", Group: "Core"},
		{ID: "svc:core:b", Label: "b-svc", Namespace: "core", NodeType: "service", Group: "Core"},
	})
	edgeSet := makeFlowEdgeSet("es1", "nfd-main", []sreportalv1alpha1.FlowEdge{
		{From: "svc:pay:z", To: "svc:core:a", EdgeType: "cross-pl"},
		{From: "svc:core:a", To: "svc:core:b", EdgeType: "internal"},
	})

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(nodeSet, edgeSet).
		WithStatusSubresource(nodeSet, edgeSet).
		Build()

	svc := svcgrpc.NewNetworkPolicyService(k8sClient)

	resp, err := svc.ListNetworkPolicies(
		context.Background(),
		connect.NewRequest(&netpolv1.ListNetworkPoliciesRequest{}),
	)

	require.NoError(t, err)

	// Nodes sorted by group, then label: Core/a-svc, Core/b-svc, Pay/z-svc
	require.Len(t, resp.Msg.Nodes, 3)
	assert.Equal(t, "svc:core:a", resp.Msg.Nodes[0].Id)
	assert.Equal(t, "svc:core:b", resp.Msg.Nodes[1].Id)
	assert.Equal(t, "svc:pay:z", resp.Msg.Nodes[2].Id)

	// Edges sorted by from, then to: core:a->core:b, pay:z->core:a
	require.Len(t, resp.Msg.Edges, 2)
	assert.Equal(t, "svc:core:a", resp.Msg.Edges[0].From)
	assert.Equal(t, "svc:core:b", resp.Msg.Edges[0].To)
	assert.Equal(t, "svc:pay:z", resp.Msg.Edges[1].From)
	assert.Equal(t, "svc:core:a", resp.Msg.Edges[1].To)
}
