package netpol

import "context"

// FlowGraphWriter provides write access to the network flow graph projection.
type FlowGraphWriter interface {
	ReplaceNodes(ctx context.Context, key, portalRef string, nodes []FlowNode) error
	ReplaceEdges(ctx context.Context, key, portalRef string, edges []FlowEdge) error
	Delete(ctx context.Context, key string) error
}
