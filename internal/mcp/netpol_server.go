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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NetpolServer wraps the MCP server for network policy analysis.
// It reads pre-computed flow graphs from NetworkFlowDiscovery CRD status.
// Mount at /mcp/netpol for Streamable HTTP.
type NetpolServer struct {
	mcpServer *server.MCPServer
	client    client.Client
}

// NewNetpolServer creates a new MCP server instance for network policies.
func NewNetpolServer(k8sClient client.Client) *NetpolServer {
	s := &NetpolServer{client: k8sClient}

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

	s.registerTools()

	return s
}

func (s *NetpolServer) registerTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("list_network_flows",
			mcp.WithDescription("List all network flows between services, databases, crons, and external endpoints "+
				"derived from Kubernetes NetworkPolicies and FQDNNetworkPolicies. "+
				"Returns nodes (services, databases, crons, externals) and edges (directional flows)."),
			mcp.WithString("search",
				mcp.Description("Search by service or resource name (substring match)"),
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
		),
		withToolMetrics("netpol", "get_service_flows", s.handleGetServiceFlows),
	)

	s.mcpServer.AddTool(
		mcp.NewTool("impact_analysis",
			mcp.WithDescription("Analyze the blast radius of a resource going down. "+
				"Given a resource (database, service, external), returns all services impacted, level by level."),
			mcp.WithString("resource",
				mcp.Required(),
				mcp.Description("Resource name (database, service, or external endpoint)"),
			),
			mcp.WithNumber("max_depth",
				mcp.Description("Maximum depth for impact propagation (default 4)"),
			),
		),
		withToolMetrics("netpol", "impact_analysis", s.handleImpactAnalysis),
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
	From     string `json:"from"`
	To       string `json:"to"`
	EdgeType string `json:"edge_type"`
}

// loadGraph reads the pre-computed graph from FlowNodeSet and FlowEdgeSet CRDs.
func (s *NetpolServer) loadGraph(ctx context.Context) (map[string]netpolNodeResult, []netpolEdgeResult, error) {
	var nodeSetList sreportalv1alpha1.FlowNodeSetList
	if err := s.client.List(ctx, &nodeSetList); err != nil {
		return nil, nil, fmt.Errorf("failed to list FlowNodeSet resources: %w", err)
	}

	var edgeSetList sreportalv1alpha1.FlowEdgeSetList
	if err := s.client.List(ctx, &edgeSetList); err != nil {
		return nil, nil, fmt.Errorf("failed to list FlowEdgeSet resources: %w", err)
	}

	nodeMap := make(map[string]netpolNodeResult)
	for _, ns := range nodeSetList.Items {
		for _, n := range ns.Status.Nodes {
			if _, ok := nodeMap[n.ID]; !ok {
				nodeMap[n.ID] = netpolNodeResult{
					ID: n.ID, Label: n.Label, Namespace: n.Namespace, NodeType: n.NodeType, Group: n.Group,
				}
			}
		}
	}

	edgeSet := make(map[string]netpolEdgeResult)
	for _, es := range edgeSetList.Items {
		for _, e := range es.Status.Edges {
			key := e.From + "|" + e.To + "|" + e.EdgeType
			if _, ok := edgeSet[key]; !ok {
				edgeSet[key] = netpolEdgeResult{From: e.From, To: e.To, EdgeType: e.EdgeType}
			}
		}
	}

	edges := make([]netpolEdgeResult, 0, len(edgeSet))
	for _, e := range edgeSet {
		edges = append(edges, e)
	}

	return nodeMap, edges, nil
}

func (s *NetpolServer) handleListFlows(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	search := strings.ToLower(request.GetString("search", ""))

	nodeMap, edges, err := s.loadGraph(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodes := make([]netpolNodeResult, 0, len(nodeMap))
	for _, n := range nodeMap {
		if search != "" && !strings.Contains(strings.ToLower(n.Label), search) &&
			!strings.Contains(strings.ToLower(n.Group), search) {
			continue
		}
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	result := map[string]any{
		"total_nodes": len(nodes),
		"total_edges": len(edges),
		"nodes":       nodes,
		"edges":       edges,
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
}

// ServiceFlows represents the incoming and outgoing flows for a service.
type ServiceFlows struct {
	Service  netpolNodeResult `json:"service"`
	CallsTo  []FlowTarget     `json:"calls_to"`
	CalledBy []FlowTarget     `json:"called_from"`
}

func (s *NetpolServer) handleGetServiceFlows(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svcName := request.GetString("service", "")

	nodeMap, edges, err := s.loadGraph(ctx)
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
	for _, e := range edges {
		if e.From == found.ID {
			if tgt, ok := nodeMap[e.To]; ok {
				flows.CallsTo = append(flows.CallsTo, FlowTarget{Node: tgt, EdgeType: e.EdgeType})
			}
		}
		if e.To == found.ID {
			if src, ok := nodeMap[e.From]; ok {
				flows.CalledBy = append(flows.CalledBy, FlowTarget{Node: src, EdgeType: e.EdgeType})
			}
		}
	}

	jsonBytes, err := json.MarshalIndent(flows, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// ImpactLevel represents one level in the impact analysis.
type ImpactLevel struct {
	Depth int          `json:"depth"`
	Nodes []ImpactNode `json:"nodes"`
}

// ImpactNode is a node in an impact level.
type ImpactNode struct {
	Node netpolNodeResult `json:"node"`
	Via  string           `json:"via"`
}

func (s *NetpolServer) handleImpactAnalysis(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceName := request.GetString("resource", "")
	maxDepth := int(request.GetFloat("max_depth", 4))

	nodeMap, edges, err := s.loadGraph(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var targetID string
	for id, n := range nodeMap {
		if strings.EqualFold(n.Label, resourceName) {
			targetID = id
			break
		}
	}
	if targetID == "" {
		return mcp.NewToolResultText(fmt.Sprintf("Resource %q not found.", resourceName)), nil
	}

	// Build reverse adjacency
	callsFrom := make(map[string][]netpolEdgeResult)
	for _, e := range edges {
		callsFrom[e.To] = append(callsFrom[e.To], e)
	}

	// BFS
	visited := map[string]bool{targetID: true}
	currentLevel := map[string]bool{targetID: true}
	levels := []ImpactLevel{{Depth: 0, Nodes: []ImpactNode{{Node: nodeMap[targetID]}}}}

	for depth := 1; depth <= maxDepth; depth++ {
		var nextNodes []ImpactNode
		nextLevel := make(map[string]bool)

		for nid := range currentLevel {
			for _, e := range callsFrom[nid] {
				if !visited[e.From] {
					visited[e.From] = true
					nextLevel[e.From] = true
					via := nodeMap[nid].Label
					nextNodes = append(nextNodes, ImpactNode{Node: nodeMap[e.From], Via: via})
				}
			}
		}

		if len(nextNodes) == 0 {
			break
		}

		sort.Slice(nextNodes, func(i, j int) bool { return nextNodes[i].Node.Label < nextNodes[j].Node.Label })
		levels = append(levels, ImpactLevel{Depth: depth, Nodes: nextNodes})
		currentLevel = nextLevel
	}

	result := map[string]any{
		"resource":     nodeMap[targetID],
		"blast_radius": len(visited) - 1,
		"levels":       levels,
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// Handler returns an http.Handler for the MCP Streamable HTTP transport.
func (s *NetpolServer) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}
