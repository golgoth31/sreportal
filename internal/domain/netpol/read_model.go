package netpol

// FlowNode is the read model for a node in the network flow graph.
type FlowNode struct {
	ID        string
	Label     string
	Namespace string
	NodeType  string
	Group     string
}

// FlowEdge is the read model for a directional edge in the network flow graph.
type FlowEdge struct {
	From     string
	To       string
	EdgeType string
	Used     bool // true if traffic was observed on this edge
}

// FlowGraphFilters specifies optional filters for querying the flow graph.
type FlowGraphFilters struct {
	Portal    string
	Namespace string
	Search    string
}
