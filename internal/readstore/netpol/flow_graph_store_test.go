package netpol

import (
	"context"
	"testing"

	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlowGraphStore_ReplaceAndListNodes(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	nodes := []domainnetpol.FlowNode{
		{ID: tNodeSvcNs1A, Label: "a", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1},
		{ID: tNodeSvcNs1B, Label: "b", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1},
	}
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "portal-1", nodes))

	got, err := store.ListNodes(ctx, domainnetpol.FlowGraphFilters{})
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestFlowGraphStore_ReplaceAndListEdges(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "portal-1", []domainnetpol.FlowNode{
		{ID: tNodeSvcNs1A, Label: "a", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1},
		{ID: tNodeSvcNs1B, Label: "b", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "portal-1", []domainnetpol.FlowEdge{
		{From: tNodeSvcNs1A, To: tNodeSvcNs1B, EdgeType: tEdgeInternal},
	}))

	got, err := store.ListEdges(ctx, domainnetpol.FlowGraphFilters{})
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestFlowGraphStore_PortalFiltering(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "portal-1", []domainnetpol.FlowNode{
		{ID: tNodeSvcNs1A, Label: "a", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1},
	}))
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-2", "portal-2", []domainnetpol.FlowNode{
		{ID: tNodeSvcNs2B, Label: "b", Namespace: tNs2, NodeType: tNodeTypeService, Group: tNs2},
	}))
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "portal-1", []domainnetpol.FlowNode{
		{ID: tNodeSvcNs1A, Label: "a", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1},
		{ID: "svc:ns1:c", Label: "c", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1},
	}))
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-2", "portal-2", []domainnetpol.FlowNode{
		{ID: tNodeSvcNs2B, Label: "b", Namespace: tNs2, NodeType: tNodeTypeService, Group: tNs2},
		{ID: "svc:ns2:d", Label: "d", Namespace: tNs2, NodeType: tNodeTypeService, Group: tNs2},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "portal-1", []domainnetpol.FlowEdge{
		{From: tNodeSvcNs1A, To: "svc:ns1:c", EdgeType: tEdgeInternal},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-2", "portal-2", []domainnetpol.FlowEdge{
		{From: tNodeSvcNs2B, To: "svc:ns2:d", EdgeType: tEdgeInternal},
	}))

	nodes, err := store.ListNodes(ctx, domainnetpol.FlowGraphFilters{Portal: "portal-1"})
	require.NoError(t, err)
	assert.Len(t, nodes, 2) // a and c from nfd-1
	nodeIDs := make(map[string]bool)
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}
	assert.True(t, nodeIDs[tNodeSvcNs1A])
	assert.True(t, nodeIDs["svc:ns1:c"])

	edges, err := store.ListEdges(ctx, domainnetpol.FlowGraphFilters{Portal: "portal-1"})
	require.NoError(t, err)
	assert.Len(t, edges, 1)
	assert.Equal(t, tNodeSvcNs1A, edges[0].From)
}

func TestFlowGraphStore_NamespaceFiltering(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "", []domainnetpol.FlowNode{
		{ID: tNodeSvcNs1A, Label: "a", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1},
		{ID: tNodeSvcNs2B, Label: "b", Namespace: tNs2, NodeType: tNodeTypeService, Group: tNs2},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "", []domainnetpol.FlowEdge{
		{From: tNodeSvcNs1A, To: tNodeSvcNs2B, EdgeType: "cross-ns"},
	}))

	nodes, err := store.ListNodes(ctx, domainnetpol.FlowGraphFilters{Namespace: tNs1})
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, tNs1, nodes[0].Namespace)

	edges, err := store.ListEdges(ctx, domainnetpol.FlowGraphFilters{Namespace: tNs1})
	require.NoError(t, err)
	assert.Empty(t, edges) // edge pruned because ns2:b is not in filtered nodes
}

func TestFlowGraphStore_SearchWithNeighborExpansion(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "", []domainnetpol.FlowNode{
		{ID: tNodeSvcNs1API, Label: "api-gateway", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1},
		{ID: "svc:ns1:db", Label: "postgres", Namespace: tNs1, NodeType: "database", Group: tNs1},
		{ID: tNodeSvcNs1Other, Label: "unrelated", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "", []domainnetpol.FlowEdge{
		{From: tNodeSvcNs1API, To: "svc:ns1:db", EdgeType: "database"},
		{From: tNodeSvcNs1Other, To: tNodeSvcNs1API, EdgeType: tEdgeInternal},
	}))

	// Search for "api" should include api-gateway + 1-hop neighbors (postgres, unrelated)
	nodes, err := store.ListNodes(ctx, domainnetpol.FlowGraphFilters{Search: "api"})
	require.NoError(t, err)
	assert.Len(t, nodes, 3)

	edges, err := store.ListEdges(ctx, domainnetpol.FlowGraphFilters{Search: "api"})
	require.NoError(t, err)
	assert.Len(t, edges, 2)
}

func TestFlowGraphStore_Delete(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "portal-1", []domainnetpol.FlowNode{
		{ID: tNodeSvcNs1A, Label: "a", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "portal-1", []domainnetpol.FlowEdge{
		{From: tNodeSvcNs1A, To: tNodeSvcNs1B, EdgeType: tEdgeInternal},
	}))

	require.NoError(t, store.Delete(ctx, "nfd-1"))

	nodes, err := store.ListNodes(ctx, domainnetpol.FlowGraphFilters{})
	require.NoError(t, err)
	assert.Empty(t, nodes)

	edges, err := store.ListEdges(ctx, domainnetpol.FlowGraphFilters{})
	require.NoError(t, err)
	assert.Empty(t, edges)
}

func TestFlowGraphStore_Subscribe(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	ch := store.Subscribe()

	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "", []domainnetpol.FlowNode{
		{ID: tNodeSvcNs1A, Label: "a", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1},
	}))

	select {
	case <-ch:
		// expected
	default:
		t.Fatal("expected notification after ReplaceNodes")
	}
}

func TestFlowGraphStore_MergeEvaluatedAndUsedAcrossKeys(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	node := domainnetpol.FlowNode{ID: tNodeSvcNs1A, Label: "a", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1}
	other := domainnetpol.FlowNode{ID: tNodeSvcNs1B, Label: "b", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1}
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "", []domainnetpol.FlowNode{node, other}))
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-2", "", []domainnetpol.FlowNode{node, other}))

	// nfd-1 has the edge as evaluated+used, nfd-2 has it as not evaluated.
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "", []domainnetpol.FlowEdge{
		{From: tNodeSvcNs1A, To: tNodeSvcNs1B, EdgeType: tEdgeInternal, Used: true, Evaluated: true},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-2", "", []domainnetpol.FlowEdge{
		{From: tNodeSvcNs1A, To: tNodeSvcNs1B, EdgeType: tEdgeInternal, Used: false, Evaluated: false},
	}))

	edges, err := store.ListEdges(ctx, domainnetpol.FlowGraphFilters{})
	require.NoError(t, err)
	require.Len(t, edges, 1)
	assert.True(t, edges[0].Used, "Used should be true (OR merge)")
	assert.True(t, edges[0].Evaluated, "Evaluated should be true (OR merge)")
}

func TestFlowGraphStore_DeduplicationAcrossKeys(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	node := domainnetpol.FlowNode{ID: "svc:ns1:shared", Label: "shared", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1}
	other := domainnetpol.FlowNode{ID: tNodeSvcNs1Other, Label: "other", Namespace: tNs1, NodeType: tNodeTypeService, Group: tNs1}
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "", []domainnetpol.FlowNode{node, other}))
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-2", "", []domainnetpol.FlowNode{node, other}))

	edge := domainnetpol.FlowEdge{From: "svc:ns1:shared", To: tNodeSvcNs1Other, EdgeType: tEdgeInternal}
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "", []domainnetpol.FlowEdge{edge}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-2", "", []domainnetpol.FlowEdge{edge}))

	nodes, err := store.ListNodes(ctx, domainnetpol.FlowGraphFilters{})
	require.NoError(t, err)
	assert.Len(t, nodes, 2) // shared + other, deduplicated across nfd-1 and nfd-2

	edges, err := store.ListEdges(ctx, domainnetpol.FlowGraphFilters{})
	require.NoError(t, err)
	assert.Len(t, edges, 1) // same edge deduplicated
}
