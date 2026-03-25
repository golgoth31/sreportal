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

	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NetpolServer wraps the MCP server for network policy analysis.
// It reads pre-computed flow graphs from FlowNodeSet and FlowEdgeSet CRDs.
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

	s.mcpServer.AddTool(
		mcp.NewTool("impact_analysis",
			mcp.WithDescription("Analyze the blast radius of a resource going down. "+
				"Given a resource (database, service, external), returns all services impacted, level by level."),
			mcp.WithString("resource",
				mcp.Required(),
				mcp.Description("Resource name (database, service, or external endpoint)"),
			),
			mcp.WithString("portal",
				mcp.Description("Filter by portal name (empty for all)"),
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

// resolveAllowedDiscoveries returns the set of NetworkFlowDiscovery names linked to the
// given portal. Returns nil when no portal filter is specified (all discoveries allowed).
func (s *NetpolServer) resolveAllowedDiscoveries(ctx context.Context, portal string) (map[string]struct{}, error) {
	if portal == "" {
		return nil, nil
	}

	var nfdList sreportalv1alpha1.NetworkFlowDiscoveryList
	if err := s.client.List(ctx, &nfdList, client.MatchingFields{"spec.portalRef": portal}); err != nil {
		return nil, fmt.Errorf("resolve allowed discoveries for portal %q: %w", portal, err)
	}

	allowed := make(map[string]struct{}, len(nfdList.Items))
	for _, nfd := range nfdList.Items {
		allowed[nfd.Name] = struct{}{}
	}

	return allowed, nil
}

// loadGraph reads the pre-computed graph from FlowNodeSet and FlowEdgeSet CRDs,
// filtering by allowed discoveries when portal filtering is active.
func (s *NetpolServer) loadGraph(ctx context.Context, allowed map[string]struct{}) (map[string]netpolNodeResult, map[string]netpolEdgeResult, error) {
	var nodeSetList sreportalv1alpha1.FlowNodeSetList
	if err := s.client.List(ctx, &nodeSetList); err != nil {
		return nil, nil, fmt.Errorf("list FlowNodeSet resources: %w", err)
	}

	var edgeSetList sreportalv1alpha1.FlowEdgeSetList
	if err := s.client.List(ctx, &edgeSetList); err != nil {
		return nil, nil, fmt.Errorf("list FlowEdgeSet resources: %w", err)
	}

	nodeMap := make(map[string]netpolNodeResult)
	for _, ns := range nodeSetList.Items {
		if allowed != nil {
			if _, ok := allowed[ns.Spec.DiscoveryRef]; !ok {
				continue
			}
		}

		for _, n := range ns.Status.Nodes {
			if _, ok := nodeMap[n.ID]; !ok {
				nodeMap[n.ID] = netpolNodeResult{
					ID: n.ID, Label: n.Label, Namespace: n.Namespace, NodeType: n.NodeType, Group: n.Group,
				}
			}
		}
	}

	edgeMap := make(map[string]netpolEdgeResult)
	for _, es := range edgeSetList.Items {
		if allowed != nil {
			if _, ok := allowed[es.Spec.DiscoveryRef]; !ok {
				continue
			}
		}

		for _, e := range es.Status.Edges {
			key := e.From + "|" + e.To + "|" + e.EdgeType
			if _, ok := edgeMap[key]; !ok {
				edgeMap[key] = netpolEdgeResult{From: e.From, To: e.To, EdgeType: e.EdgeType}
			}
		}
	}

	return nodeMap, edgeMap, nil
}

// mcpFilterByNamespace removes nodes not in the given namespace and prunes orphan edges.
func mcpFilterByNamespace(nodeMap map[string]netpolNodeResult, edgeMap map[string]netpolEdgeResult, namespace string) {
	if namespace == "" {
		return
	}

	for id, n := range nodeMap {
		if n.Namespace != namespace {
			delete(nodeMap, id)
		}
	}

	mcpPruneOrphanEdges(nodeMap, edgeMap)
}

// mcpFilterBySearch keeps only nodes matching the search term and their direct neighbors (1 hop).
// Matches on label, group, and namespace (case-insensitive).
func mcpFilterBySearch(nodeMap map[string]netpolNodeResult, edgeMap map[string]netpolEdgeResult, search string) {
	search = strings.ToLower(search)
	if search == "" {
		return
	}

	directMatch := make(map[string]bool)
	for id, n := range nodeMap {
		if strings.Contains(strings.ToLower(n.Label), search) ||
			strings.Contains(strings.ToLower(n.Group), search) ||
			strings.Contains(strings.ToLower(n.Namespace), search) {
			directMatch[id] = true
		}
	}

	matched := make(map[string]bool, len(directMatch))
	for id := range directMatch {
		matched[id] = true
	}

	for _, e := range edgeMap {
		if directMatch[e.From] {
			matched[e.To] = true
		}

		if directMatch[e.To] {
			matched[e.From] = true
		}
	}

	for id := range nodeMap {
		if !matched[id] {
			delete(nodeMap, id)
		}
	}

	mcpPruneOrphanEdges(nodeMap, edgeMap)
}

// mcpPruneOrphanEdges removes edges whose source or target node is no longer in nodeMap.
func mcpPruneOrphanEdges(nodeMap map[string]netpolNodeResult, edgeMap map[string]netpolEdgeResult) {
	for key, e := range edgeMap {
		if _, ok := nodeMap[e.From]; !ok {
			delete(edgeMap, key)
			continue
		}

		if _, ok := nodeMap[e.To]; !ok {
			delete(edgeMap, key)
		}
	}
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

	allowed, err := s.resolveAllowedDiscoveries(ctx, portal)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodeMap, edgeMap, err := s.loadGraph(ctx, allowed)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	mcpFilterByNamespace(nodeMap, edgeMap, namespace)
	mcpFilterBySearch(nodeMap, edgeMap, search)

	nodes := mcpSortedNodes(nodeMap)
	edges := mcpSortedEdges(edgeMap)

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
	portal := request.GetString("portal", "")

	allowed, err := s.resolveAllowedDiscoveries(ctx, portal)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodeMap, edgeMap, err := s.loadGraph(ctx, allowed)
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
				flows.CallsTo = append(flows.CallsTo, FlowTarget{Node: tgt, EdgeType: e.EdgeType})
			}
		}

		if e.To == found.ID {
			if src, ok := nodeMap[e.From]; ok {
				flows.CalledBy = append(flows.CalledBy, FlowTarget{Node: src, EdgeType: e.EdgeType})
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
	portal := request.GetString("portal", "")
	maxDepth := int(request.GetFloat("max_depth", 4))

	allowed, err := s.resolveAllowedDiscoveries(ctx, portal)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodeMap, edgeMap, err := s.loadGraph(ctx, allowed)
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

	// Build reverse adjacency from edge map.
	callsFrom := make(map[string][]netpolEdgeResult)
	for _, e := range edgeMap {
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

		slices.SortFunc(nextNodes, func(a, b ImpactNode) int { return cmp.Compare(a.Node.Label, b.Node.Label) })
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
