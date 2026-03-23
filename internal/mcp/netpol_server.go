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
	"slices"
	"sort"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NetpolServer wraps the MCP server for network policy analysis.
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
			mcp.WithString("namespace",
				mcp.Description("Filter by Kubernetes namespace"),
			),
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
			mcp.WithString("namespace",
				mcp.Description("Namespace of the service (optional, searches all namespaces if empty)"),
			),
		),
		withToolMetrics("netpol", "get_service_flows", s.handleGetServiceFlows),
	)

	s.mcpServer.AddTool(
		mcp.NewTool("impact_analysis",
			mcp.WithDescription("Analyze the blast radius of a resource going down. "+
				"Given a resource (database, service, external), returns all services impacted, level by level. "+
				"Level 1 = direct dependents, Level 2 = services that call Level 1, etc."),
			mcp.WithString("resource",
				mcp.Required(),
				mcp.Description("Resource name (database, service, or external endpoint)"),
			),
			mcp.WithString("namespace",
				mcp.Description("Namespace to narrow the search"),
			),
			mcp.WithNumber("max_depth",
				mcp.Description("Maximum depth for impact propagation (default 4)"),
			),
		),
		withToolMetrics("netpol", "impact_analysis", s.handleImpactAnalysis),
	)
}

// netpolGraph is the internal graph structure shared by all handlers.
type netpolGraph struct {
	nodes map[string]netpolNode
	edges []netpolEdge
}

type netpolNode struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Namespace string `json:"namespace"`
	NodeType  string `json:"node_type"`
	Group     string `json:"group"`
}

type netpolEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	EdgeType string `json:"edge_type"`
}

var mcpPolicySuffixes = []string{
	"-ingress-policy", "-egress-policy", "-fqdn-network-policy",
	"-cron-egress-policy", "-cron-fqdn-network-policy",
}

func (s *NetpolServer) buildGraph(ctx context.Context, namespace string) (*netpolGraph, error) {
	g := &netpolGraph{nodes: make(map[string]netpolNode)}
	appToNs := make(map[string]string)

	listOpts := []client.ListOption{}
	if namespace != "" {
		listOpts = append(listOpts, client.InNamespace(namespace))
	}

	var npList networkingv1.NetworkPolicyList
	if err := s.client.List(ctx, &npList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list NetworkPolicies: %w", err)
	}

	for _, np := range npList.Items {
		for _, suffix := range mcpPolicySuffixes {
			if app, ok := strings.CutSuffix(np.Name, suffix); ok {
				appToNs[app] = np.Namespace
				break
			}
		}
	}

	nid := func(name, ns, ntype string) string { return fmt.Sprintf("%s:%s:%s", ntype, ns, name) }
	ensure := func(id, label, ns, ntype string) {
		if _, ok := g.nodes[id]; !ok {
			g.nodes[id] = netpolNode{ID: id, Label: label, Namespace: ns, NodeType: ntype, Group: ns}
		}
	}

	g.parseIngressPolicies(npList.Items, appToNs, nid, ensure)
	g.parseFQDNPolicies(ctx, s.client, listOpts, nid, ensure)
	g.deduplicateEdges()

	return g, nil
}

func (g *netpolGraph) parseIngressPolicies(
	items []networkingv1.NetworkPolicy,
	appToNs map[string]string,
	nid func(string, string, string) string,
	ensure func(string, string, string, string),
) {
	for _, np := range items {
		targetApp, ok := strings.CutSuffix(np.Name, "-ingress-policy")
		if !ok || !slices.Contains(np.Spec.PolicyTypes, networkingv1.PolicyTypeIngress) {
			continue
		}

		tgtID := nid(targetApp, np.Namespace, "service")
		ensure(tgtID, targetApp, np.Namespace, "service")

		for _, rule := range np.Spec.Ingress {
			for _, from := range rule.From {
				if from.PodSelector == nil {
					continue
				}
				for _, expr := range from.PodSelector.MatchExpressions {
					switch {
					case expr.Key == "app.kubernetes.io/name" && expr.Operator == "In":
						for _, src := range expr.Values {
							srcNs := appToNs[src]
							if srcNs == "" {
								srcNs = "unknown"
							}
							srcID := nid(src, srcNs, "service")
							ensure(srcID, src, srcNs, "service")
							eType := "internal"
							if srcNs != np.Namespace {
								eType = "cross-ns"
							}
							g.edges = append(g.edges, netpolEdge{From: srcID, To: tgtID, EdgeType: eType})
						}
					case expr.Key == "basename" && expr.Operator == "In":
						for _, src := range expr.Values {
							srcID := nid(src, np.Namespace, "cron")
							ensure(srcID, src, np.Namespace, "cron")
							g.edges = append(g.edges, netpolEdge{From: srcID, To: tgtID, EdgeType: "cron"})
						}
					}
				}
			}
		}
	}
}

func (g *netpolGraph) parseFQDNPolicies(
	ctx context.Context,
	k8sClient client.Client,
	listOpts []client.ListOption,
	nid func(string, string, string) string,
	ensure func(string, string, string, string),
) {
	var fqdnList unstructured.UnstructuredList
	fqdnList.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "networking.gke.io", Version: "v1alpha3", Kind: "FQDNNetworkPolicyList",
	})
	if err := k8sClient.List(ctx, &fqdnList, listOpts...); err != nil {
		return
	}

	for _, item := range fqdnList.Items {
		ns := item.GetNamespace()
		name := item.GetName()
		isCron := strings.Contains(name, "-cron-fqdn-")
		appName := strings.TrimSuffix(strings.TrimSuffix(name, "-cron-fqdn-network-policy"), "-fqdn-network-policy")

		srcType := "service"
		if isCron {
			srcType = "cron"
		}
		srcID := nid(appName, ns, srcType)
		ensure(srcID, appName, ns, srcType)

		egressRules, _, _ := unstructured.NestedSlice(item.Object, "spec", "egress")
		for _, ruleRaw := range egressRules {
			rule, ok := ruleRaw.(map[string]any)
			if !ok {
				continue
			}
			ports, _, _ := unstructured.NestedSlice(rule, "ports")
			portNums := mcpExtractPorts(ports)

			toRules, _, _ := unstructured.NestedSlice(rule, "to")
			for _, toRaw := range toRules {
				to, ok := toRaw.(map[string]any)
				if !ok {
					continue
				}
				fqdns, _, _ := unstructured.NestedStringSlice(to, "fqdns")
				for _, fqdn := range fqdns {
					cat, label := mcpClassifyFQDN(fqdn, portNums)
					dbID := nid(label, ns, cat)
					ensure(dbID, label, ns, cat)
					g.edges = append(g.edges, netpolEdge{From: srcID, To: dbID, EdgeType: cat})
				}
			}
		}
	}
}

func (g *netpolGraph) deduplicateEdges() {
	edgeSet := make(map[string]bool)
	deduped := make([]netpolEdge, 0, len(g.edges))
	for _, e := range g.edges {
		key := e.From + "|" + e.To + "|" + e.EdgeType
		if !edgeSet[key] {
			edgeSet[key] = true
			deduped = append(deduped, e)
		}
	}
	g.edges = deduped
}

// handleListFlows returns the full graph of nodes and edges.
func (s *NetpolServer) handleListFlows(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	namespace := request.GetString("namespace", "")
	search := strings.ToLower(request.GetString("search", ""))

	g, err := s.buildGraph(ctx, namespace)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodes := make([]netpolNode, 0, len(g.nodes))
	for _, n := range g.nodes {
		if search != "" && !strings.Contains(strings.ToLower(n.Label), search) &&
			!strings.Contains(strings.ToLower(n.Group), search) {
			continue
		}
		nodes = append(nodes, n)
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	result := map[string]any{
		"total_nodes": len(nodes),
		"total_edges": len(g.edges),
		"nodes":       nodes,
		"edges":       g.edges,
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// ServiceFlows represents the incoming and outgoing flows for a service.
type ServiceFlows struct {
	Service  netpolNode   `json:"service"`
	CallsTo  []FlowTarget `json:"calls_to"`
	CalledBy []FlowTarget `json:"called_from"`
}

// FlowTarget is a target/source in a service flow.
type FlowTarget struct {
	Node     netpolNode `json:"node"`
	EdgeType string     `json:"edge_type"`
}

func (s *NetpolServer) handleGetServiceFlows(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svcName := request.GetString("service", "")
	namespace := request.GetString("namespace", "")

	g, err := s.buildGraph(ctx, namespace)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Find the node
	var found *netpolNode
	for _, n := range g.nodes {
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
	for _, e := range g.edges {
		if e.From == found.ID {
			if tgt, ok := g.nodes[e.To]; ok {
				flows.CallsTo = append(flows.CallsTo, FlowTarget{Node: tgt, EdgeType: e.EdgeType})
			}
		}
		if e.To == found.ID {
			if src, ok := g.nodes[e.From]; ok {
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

// ImpactNode is a node in an impact level with the path that caused the impact.
type ImpactNode struct {
	Node netpolNode `json:"node"`
	Via  string     `json:"via"`
}

func (s *NetpolServer) handleImpactAnalysis(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceName := request.GetString("resource", "")
	namespace := request.GetString("namespace", "")
	maxDepth := int(request.GetFloat("max_depth", 4))

	g, err := s.buildGraph(ctx, namespace)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Find the target node
	var targetID string
	for id, n := range g.nodes {
		if strings.EqualFold(n.Label, resourceName) {
			targetID = id
			break
		}
	}
	if targetID == "" {
		return mcp.NewToolResultText(fmt.Sprintf("Resource %q not found.", resourceName)), nil
	}

	// Build reverse adjacency: who calls X?
	callsFrom := make(map[string][]netpolEdge)
	for _, e := range g.edges {
		callsFrom[e.To] = append(callsFrom[e.To], e)
	}

	// BFS
	visited := map[string]bool{targetID: true}
	currentLevel := map[string]bool{targetID: true}

	levels := []ImpactLevel{{Depth: 0, Nodes: []ImpactNode{{Node: g.nodes[targetID]}}}}

	for depth := 1; depth <= maxDepth; depth++ {
		var nextNodes []ImpactNode
		nextLevel := make(map[string]bool)

		for nid := range currentLevel {
			for _, e := range callsFrom[nid] {
				if !visited[e.From] {
					visited[e.From] = true
					nextLevel[e.From] = true
					via := ""
					if n, ok := g.nodes[nid]; ok {
						via = n.Label
					}
					nextNodes = append(nextNodes, ImpactNode{Node: g.nodes[e.From], Via: via})
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
		"resource":     g.nodes[targetID],
		"blast_radius": len(visited) - 1,
		"levels":       levels,
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

const (
	mcpFqdnCategoryDatabase  = "database"
	mcpFqdnCategoryMessaging = "messaging"
	mcpFqdnCategoryExternal  = "external"
)

func mcpExtractPorts(ports []any) []int64 {
	var result []int64
	for _, pRaw := range ports {
		p, ok := pRaw.(map[string]any)
		if !ok {
			continue
		}
		port, ok := p["port"]
		if !ok {
			continue
		}
		switch v := port.(type) {
		case int64:
			result = append(result, v)
		case float64:
			result = append(result, int64(v))
		}
	}
	return result
}

func mcpClassifyFQDN(fqdn string, ports []int64) (string, string) {
	for _, p := range ports {
		switch p {
		case 5432, 1433, 3306:
			return mcpFqdnCategoryDatabase, fqdn
		case 5672, 5671:
			return mcpFqdnCategoryMessaging, fqdn
		}
	}
	return mcpFqdnCategoryExternal, fqdn
}

// Handler returns an http.Handler for the MCP Streamable HTTP transport.
// Mount at /mcp/netpol.
func (s *NetpolServer) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}
