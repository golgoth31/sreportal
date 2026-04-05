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

package mcp

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NetpolServer wraps the MCP server for network policy analysis.
// It reads pre-computed flow graphs from the in-memory FlowGraphReader.
// Mount at /mcp/netpol for Streamable HTTP.
type NetpolServer struct {
	mcpServer *server.MCPServer
	reader    domainnetpol.FlowGraphReader
}

// NewNetpolServer creates a new MCP server instance for network policies.
func NewNetpolServer(reader domainnetpol.FlowGraphReader) *NetpolServer {
	s := &NetpolServer{reader: reader}

	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-netpol")
		logger.Info("client session registered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("netpol").Inc()
	})
	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-netpol")
		logger.Info("client session unregistered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("netpol").Dec()
	})
	hooks.AddAfterInitialize(func(ctx context.Context, _ any, message *mcp.InitializeRequest, _ *mcp.InitializeResult) {
		logger := log.FromContext(ctx).WithName("mcp-netpol")
		logger.Info("client initialized",
			"clientName", message.Params.ClientInfo.Name,
			"clientVersion", message.Params.ClientInfo.Version,
			"protocolVersion", message.Params.ProtocolVersion,
		)
	})

	s.mcpServer = server.NewMCPServer(
		"sreportal-netpol",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithHooks(hooks),
	)

	s.registerNetpolTools()

	return s
}

// registerNetpolTools registers network-policy-related MCP tools.
func (s *NetpolServer) registerNetpolTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("list_network_flows",
			mcp.WithDescription("List all network flows between services, databases, crons, and external endpoints "+
				"derived from Kubernetes NetworkPolicies and FQDNNetworkPolicies. "+
				"Returns nodes (services, databases, crons, externals) and edges (directional flows)."),
			mcp.WithString("portal",
				mcp.Description("Filter by portal name (empty for all)"),
			),
			mcp.WithString("namespace",
				mcp.Description("Filter nodes by Kubernetes namespace"),
			),
			mcp.WithString("search",
				mcp.Description("Search by service name, group, or namespace (substring match). "+
					"Also includes direct neighbors (1 hop) of matching nodes."),
			),
		),
		withToolMetrics("netpol", "list_network_flows", s.handleListFlows),
	)

	s.mcpServer.AddTool(
		mcp.NewTool("get_service_flows",
			mcp.WithDescription("Get all incoming and outgoing flows for a specific service. "+
				"Shows which services call it and which services/databases/externals it calls."),
			mcp.WithString("service",
				mcp.Required(),
				mcp.Description("Service name to look up"),
			),
			mcp.WithString("portal",
				mcp.Description("Filter by portal name (empty for all)"),
			),
		),
		withToolMetrics("netpol", "get_service_flows", s.handleGetServiceFlows),
	)

}

// netpolResult types for JSON serialization.
type netpolNodeResult struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Namespace string `json:"namespace"`
	NodeType  string `json:"node_type"`
	Group     string `json:"group"`
}

type netpolEdgeResult struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	EdgeType string  `json:"edge_type"`
	LastSeen *string `json:"last_seen,omitempty"`
}

func flowNodeToMCPResult(n domainnetpol.FlowNode) netpolNodeResult {
	return netpolNodeResult{
		ID: n.ID, Label: n.Label, Namespace: n.Namespace, NodeType: n.NodeType, Group: n.Group,
	}
}

func flowEdgeToMCPResult(e domainnetpol.FlowEdge) netpolEdgeResult {
	r := netpolEdgeResult{From: e.From, To: e.To, EdgeType: e.EdgeType}
	if e.LastSeen != nil {
		s := e.LastSeen.Format(time.RFC3339)
		r.LastSeen = &s
	}

	return r
}

// loadGraph reads the full graph from the store for the given portal, returning maps for local processing.
func (s *NetpolServer) loadGraph(ctx context.Context, portal string) (map[string]netpolNodeResult, map[string]netpolEdgeResult, error) {
	filters := domainnetpol.FlowGraphFilters{Portal: portal}

	nodes, err := s.reader.ListNodes(ctx, filters)
	if err != nil {
		return nil, nil, fmt.Errorf("list nodes: %w", err)
	}

	edges, err := s.reader.ListEdges(ctx, filters)
	if err != nil {
		return nil, nil, fmt.Errorf("list edges: %w", err)
	}

	nodeMap := make(map[string]netpolNodeResult, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = flowNodeToMCPResult(n)
	}

	edgeMap := make(map[string]netpolEdgeResult, len(edges))
	for _, e := range edges {
		key := e.From + "|" + e.To + "|" + e.EdgeType
		edgeMap[key] = flowEdgeToMCPResult(e)
	}

	return nodeMap, edgeMap, nil
}

func mcpSortedNodes(nodeMap map[string]netpolNodeResult) []netpolNodeResult {
	nodes := make([]netpolNodeResult, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}

	slices.SortFunc(nodes, func(a, b netpolNodeResult) int {
		if c := cmp.Compare(a.Group, b.Group); c != 0 {
			return c
		}

		return cmp.Compare(a.Label, b.Label)
	})

	return nodes
}

func mcpSortedEdges(edgeMap map[string]netpolEdgeResult) []netpolEdgeResult {
	edges := make([]netpolEdgeResult, 0, len(edgeMap))
	for _, e := range edgeMap {
		edges = append(edges, e)
	}

	slices.SortFunc(edges, func(a, b netpolEdgeResult) int {
		if c := cmp.Compare(a.From, b.From); c != 0 {
			return c
		}

		return cmp.Compare(a.To, b.To)
	})

	return edges
}

func (s *NetpolServer) handleListFlows(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	portal := request.GetString("portal", "")
	namespace := request.GetString("namespace", "")
	search := request.GetString("search", "")

	filters := domainnetpol.FlowGraphFilters{
		Portal:    portal,
		Namespace: namespace,
		Search:    search,
	}

	nodes, err := s.reader.ListNodes(ctx, filters)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	edges, err := s.reader.ListEdges(ctx, filters)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodeMap := make(map[string]netpolNodeResult, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = flowNodeToMCPResult(n)
	}

	edgeMap := make(map[string]netpolEdgeResult, len(edges))
	for _, e := range edges {
		key := e.From + "|" + e.To + "|" + e.EdgeType
		edgeMap[key] = flowEdgeToMCPResult(e)
	}

	sortedNodes := mcpSortedNodes(nodeMap)
	sortedEdges := mcpSortedEdges(edgeMap)

	result := map[string]any{
		"total_nodes": len(sortedNodes),
		"total_edges": len(sortedEdges),
		"nodes":       sortedNodes,
		"edges":       sortedEdges,
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// FlowTarget is a target/source in a service flow.
type FlowTarget struct {
	Node     netpolNodeResult `json:"node"`
	EdgeType string           `json:"edge_type"`
	LastSeen *string          `json:"last_seen,omitempty"`
}

// ServiceFlows represents the incoming and outgoing flows for a service.
type ServiceFlows struct {
	Service  netpolNodeResult `json:"service"`
	CallsTo  []FlowTarget     `json:"calls_to"`
	CalledBy []FlowTarget     `json:"called_from"`
}

func (s *NetpolServer) handleGetServiceFlows(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svcName := request.GetString("service", "")
	portal := request.GetString("portal", "")

	nodeMap, edgeMap, err := s.loadGraph(ctx, portal)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var found *netpolNodeResult
	for _, n := range nodeMap {
		if strings.EqualFold(n.Label, svcName) {
			n := n
			found = &n

			break
		}
	}

	if found == nil {
		return mcp.NewToolResultText(fmt.Sprintf("Service %q not found.", svcName)), nil
	}

	flows := ServiceFlows{Service: *found}
	for _, e := range edgeMap {
		if e.From == found.ID {
			if tgt, ok := nodeMap[e.To]; ok {
				flows.CallsTo = append(flows.CallsTo, FlowTarget{Node: tgt, EdgeType: e.EdgeType, LastSeen: e.LastSeen})
			}
		}

		if e.To == found.ID {
			if src, ok := nodeMap[e.From]; ok {
				flows.CalledBy = append(flows.CalledBy, FlowTarget{Node: src, EdgeType: e.EdgeType, LastSeen: e.LastSeen})
			}
		}
	}

	slices.SortFunc(flows.CallsTo, func(a, b FlowTarget) int { return cmp.Compare(a.Node.Label, b.Node.Label) })
	slices.SortFunc(flows.CalledBy, func(a, b FlowTarget) int { return cmp.Compare(a.Node.Label, b.Node.Label) })

	jsonBytes, err := json.MarshalIndent(flows, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// Handler returns an http.Handler for the MCP Streamable HTTP transport.
// Mount at /mcp/netpol.
func (s *NetpolServer) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}
