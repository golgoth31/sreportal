package netpol

import "context"

// FlowGraphReader provides read access to the network flow graph projection.
type FlowGraphReader interface {
	ListNodes(ctx context.Context, filters FlowGraphFilters) ([]FlowNode, error)
	ListEdges(ctx context.Context, filters FlowGraphFilters) ([]FlowEdge, error)
	Subscribe() <-chan struct{}
}
