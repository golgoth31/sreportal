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

	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	netpolv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	netpolreadstore "github.com/golgoth31/sreportal/internal/readstore/netpol"
)

// --- helpers ---

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

func setupStore(t *testing.T, nodes []domainnetpol.FlowNode, edges []domainnetpol.FlowEdge) *netpolreadstore.FlowGraphStore {
	t.Helper()
	store := netpolreadstore.NewFlowGraphStore()
	ctx := context.Background()
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-main", "", nodes))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-main", "", edges))

	return store
}

// --- tests ---

func TestListNetworkPolicies_ReturnsAllNodesAndEdges_WhenNoFilter(t *testing.T) {
	store := setupStore(t,
		[]domainnetpol.FlowNode{
			{ID: "svc:core:api", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"},
			{ID: "svc:core:web", Label: "web", Namespace: "core", NodeType: "service", Group: "Core"},
		},
		[]domainnetpol.FlowEdge{
			{From: "svc:core:web", To: "svc:core:api", EdgeType: "internal"},
		},
	)

	svc := svcgrpc.NewNetworkPolicyService(store)

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
	store := netpolreadstore.NewFlowGraphStore()
	ctx := context.Background()

	sharedNode := domainnetpol.FlowNode{ID: "svc:core:api", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"}
	webNode := domainnetpol.FlowNode{ID: "svc:core:web", Label: "web", Namespace: "core", NodeType: "service", Group: "Core"}
	sharedEdge := domainnetpol.FlowEdge{From: "svc:core:web", To: "svc:core:api", EdgeType: "internal"}

	require.NoError(t, store.ReplaceNodes(ctx, "nfd-a", "", []domainnetpol.FlowNode{sharedNode}))
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-b", "", []domainnetpol.FlowNode{sharedNode, webNode}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-a", "", []domainnetpol.FlowEdge{sharedEdge}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-b", "", []domainnetpol.FlowEdge{sharedEdge}))

	svc := svcgrpc.NewNetworkPolicyService(store)

	resp, err := svc.ListNetworkPolicies(
		context.Background(),
		connect.NewRequest(&netpolv1.ListNetworkPoliciesRequest{}),
	)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Nodes, 2, "duplicate node should be deduplicated")
	assert.Len(t, resp.Msg.Edges, 1, "duplicate edge should be deduplicated")
}

func TestListNetworkPolicies_FiltersByPortal(t *testing.T) {
	store := netpolreadstore.NewFlowGraphStore()
	ctx := context.Background()

	require.NoError(t, store.ReplaceNodes(ctx, "nfd-main", "main", []domainnetpol.FlowNode{
		{ID: "svc:core:api", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"},
	}))
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-other", "other", []domainnetpol.FlowNode{
		{ID: "svc:pay:stripe", Label: "stripe", Namespace: "pay", NodeType: "external", Group: "Pay"},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-main", "main", nil))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-other", "other", nil))

	svc := svcgrpc.NewNetworkPolicyService(store)

	resp, err := svc.ListNetworkPolicies(
		context.Background(),
		connect.NewRequest(&netpolv1.ListNetworkPoliciesRequest{Portal: "main"}),
	)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Nodes, 1)
	assert.Equal(t, "svc:core:api", resp.Msg.Nodes[0].Id)
}

func TestListNetworkPolicies_FiltersByNamespace(t *testing.T) {
	store := setupStore(t,
		[]domainnetpol.FlowNode{
			{ID: "svc:core:api", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"},
			{ID: "svc:pay:stripe", Label: "stripe", Namespace: "pay", NodeType: "external", Group: "Pay"},
		},
		[]domainnetpol.FlowEdge{
			{From: "svc:core:api", To: "svc:pay:stripe", EdgeType: "cross-pl"},
		},
	)

	svc := svcgrpc.NewNetworkPolicyService(store)

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
	store := setupStore(t,
		[]domainnetpol.FlowNode{
			{ID: "svc:core:api", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"},
			{ID: "svc:pay:stripe", Label: "stripe", Namespace: "pay", NodeType: "external", Group: "Pay"},
			{ID: "svc:core:web", Label: "web", Namespace: "core", NodeType: "service", Group: "Core"},
		},
		nil,
	)

	svc := svcgrpc.NewNetworkPolicyService(store)

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
	store := setupStore(t,
		[]domainnetpol.FlowNode{
			{ID: "a", Label: "api", Namespace: "core", NodeType: "service", Group: "Core"},
			{ID: "b", Label: "backend", Namespace: "core", NodeType: "service", Group: "Core"},
			{ID: "c", Label: "cache", Namespace: "core", NodeType: "database", Group: "Core"},
		},
		[]domainnetpol.FlowEdge{
			{From: "a", To: "b", EdgeType: "internal"},
			{From: "b", To: "c", EdgeType: "internal"},
		},
	)

	svc := svcgrpc.NewNetworkPolicyService(store)

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
	store := netpolreadstore.NewFlowGraphStore()

	svc := svcgrpc.NewNetworkPolicyService(store)

	resp, err := svc.ListNetworkPolicies(
		context.Background(),
		connect.NewRequest(&netpolv1.ListNetworkPoliciesRequest{}),
	)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Nodes)
	assert.Empty(t, resp.Msg.Edges)
}

func TestListNetworkPolicies_SortOrder_IsDeterministic(t *testing.T) {
	store := setupStore(t,
		[]domainnetpol.FlowNode{
			{ID: "svc:pay:z", Label: "z-svc", Namespace: "pay", NodeType: "service", Group: "Pay"},
			{ID: "svc:core:a", Label: "a-svc", Namespace: "core", NodeType: "service", Group: "Core"},
			{ID: "svc:core:b", Label: "b-svc", Namespace: "core", NodeType: "service", Group: "Core"},
		},
		[]domainnetpol.FlowEdge{
			{From: "svc:pay:z", To: "svc:core:a", EdgeType: "cross-pl"},
			{From: "svc:core:a", To: "svc:core:b", EdgeType: "internal"},
		},
	)

	svc := svcgrpc.NewNetworkPolicyService(store)

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
