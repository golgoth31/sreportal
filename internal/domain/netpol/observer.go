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
