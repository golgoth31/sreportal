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
		{ID: "svc:ns1:a", Label: "a", Namespace: "ns1", NodeType: "service", Group: "ns1"},
		{ID: "svc:ns1:b", Label: "b", Namespace: "ns1", NodeType: "service", Group: "ns1"},
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
		{ID: "svc:ns1:a", Label: "a", Namespace: "ns1", NodeType: "service", Group: "ns1"},
		{ID: "svc:ns1:b", Label: "b", Namespace: "ns1", NodeType: "service", Group: "ns1"},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "portal-1", []domainnetpol.FlowEdge{
		{From: "svc:ns1:a", To: "svc:ns1:b", EdgeType: "internal"},
	}))

	got, err := store.ListEdges(ctx, domainnetpol.FlowGraphFilters{})
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestFlowGraphStore_PortalFiltering(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "portal-1", []domainnetpol.FlowNode{
		{ID: "svc:ns1:a", Label: "a", Namespace: "ns1", NodeType: "service", Group: "ns1"},
	}))
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-2", "portal-2", []domainnetpol.FlowNode{
		{ID: "svc:ns2:b", Label: "b", Namespace: "ns2", NodeType: "service", Group: "ns2"},
	}))
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "portal-1", []domainnetpol.FlowNode{
		{ID: "svc:ns1:a", Label: "a", Namespace: "ns1", NodeType: "service", Group: "ns1"},
		{ID: "svc:ns1:c", Label: "c", Namespace: "ns1", NodeType: "service", Group: "ns1"},
	}))
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-2", "portal-2", []domainnetpol.FlowNode{
		{ID: "svc:ns2:b", Label: "b", Namespace: "ns2", NodeType: "service", Group: "ns2"},
		{ID: "svc:ns2:d", Label: "d", Namespace: "ns2", NodeType: "service", Group: "ns2"},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "portal-1", []domainnetpol.FlowEdge{
		{From: "svc:ns1:a", To: "svc:ns1:c", EdgeType: "internal"},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-2", "portal-2", []domainnetpol.FlowEdge{
		{From: "svc:ns2:b", To: "svc:ns2:d", EdgeType: "internal"},
	}))

	nodes, err := store.ListNodes(ctx, domainnetpol.FlowGraphFilters{Portal: "portal-1"})
	require.NoError(t, err)
	assert.Len(t, nodes, 2) // a and c from nfd-1
	nodeIDs := make(map[string]bool)
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}
	assert.True(t, nodeIDs["svc:ns1:a"])
	assert.True(t, nodeIDs["svc:ns1:c"])

	edges, err := store.ListEdges(ctx, domainnetpol.FlowGraphFilters{Portal: "portal-1"})
	require.NoError(t, err)
	assert.Len(t, edges, 1)
	assert.Equal(t, "svc:ns1:a", edges[0].From)
}

func TestFlowGraphStore_NamespaceFiltering(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "", []domainnetpol.FlowNode{
		{ID: "svc:ns1:a", Label: "a", Namespace: "ns1", NodeType: "service", Group: "ns1"},
		{ID: "svc:ns2:b", Label: "b", Namespace: "ns2", NodeType: "service", Group: "ns2"},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "", []domainnetpol.FlowEdge{
		{From: "svc:ns1:a", To: "svc:ns2:b", EdgeType: "cross-ns"},
	}))

	nodes, err := store.ListNodes(ctx, domainnetpol.FlowGraphFilters{Namespace: "ns1"})
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "ns1", nodes[0].Namespace)

	edges, err := store.ListEdges(ctx, domainnetpol.FlowGraphFilters{Namespace: "ns1"})
	require.NoError(t, err)
	assert.Empty(t, edges) // edge pruned because ns2:b is not in filtered nodes
}

func TestFlowGraphStore_SearchWithNeighborExpansion(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "", []domainnetpol.FlowNode{
		{ID: "svc:ns1:api", Label: "api-gateway", Namespace: "ns1", NodeType: "service", Group: "ns1"},
		{ID: "svc:ns1:db", Label: "postgres", Namespace: "ns1", NodeType: "database", Group: "ns1"},
		{ID: "svc:ns1:other", Label: "unrelated", Namespace: "ns1", NodeType: "service", Group: "ns1"},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "", []domainnetpol.FlowEdge{
		{From: "svc:ns1:api", To: "svc:ns1:db", EdgeType: "database"},
		{From: "svc:ns1:other", To: "svc:ns1:api", EdgeType: "internal"},
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
		{ID: "svc:ns1:a", Label: "a", Namespace: "ns1", NodeType: "service", Group: "ns1"},
	}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "portal-1", []domainnetpol.FlowEdge{
		{From: "svc:ns1:a", To: "svc:ns1:b", EdgeType: "internal"},
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
		{ID: "svc:ns1:a", Label: "a", Namespace: "ns1", NodeType: "service", Group: "ns1"},
	}))

	select {
	case <-ch:
		// expected
	default:
		t.Fatal("expected notification after ReplaceNodes")
	}
}

func TestFlowGraphStore_DeduplicationAcrossKeys(t *testing.T) {
	store := NewFlowGraphStore()
	ctx := context.Background()

	node := domainnetpol.FlowNode{ID: "svc:ns1:shared", Label: "shared", Namespace: "ns1", NodeType: "service", Group: "ns1"}
	other := domainnetpol.FlowNode{ID: "svc:ns1:other", Label: "other", Namespace: "ns1", NodeType: "service", Group: "ns1"}
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-1", "", []domainnetpol.FlowNode{node, other}))
	require.NoError(t, store.ReplaceNodes(ctx, "nfd-2", "", []domainnetpol.FlowNode{node, other}))

	edge := domainnetpol.FlowEdge{From: "svc:ns1:shared", To: "svc:ns1:other", EdgeType: "internal"}
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-1", "", []domainnetpol.FlowEdge{edge}))
	require.NoError(t, store.ReplaceEdges(ctx, "nfd-2", "", []domainnetpol.FlowEdge{edge}))

	nodes, err := store.ListNodes(ctx, domainnetpol.FlowGraphFilters{})
	require.NoError(t, err)
	assert.Len(t, nodes, 2) // shared + other, deduplicated across nfd-1 and nfd-2

	edges, err := store.ListEdges(ctx, domainnetpol.FlowGraphFilters{})
	require.NoError(t, err)
	assert.Len(t, edges, 1) // same edge deduplicated
}
