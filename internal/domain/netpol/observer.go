package netpol

import (
	"context"
	"time"
)

// FlowObserver detects real traffic on network edges.
// Implementations include Hubble gRPC, Prometheus queries, etc.
type FlowObserver interface {
	// Available reports whether this observer's data source is reachable.
	Available(ctx context.Context) (bool, error)

	// LastSeen returns the most recent traffic timestamp for each edge.
	// The map is keyed by "from|to|edgeType". Missing keys mean no data.
	LastSeen(ctx context.Context, edges []FlowEdge) (map[string]time.Time, error)
}

// EdgeKey returns the dedup key for an edge: "from|to|edgeType".
func EdgeKey(e FlowEdge) string {
	return e.From + "|" + e.To + "|" + e.EdgeType
}
