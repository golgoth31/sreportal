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

package grpc

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"connectrpc.com/connect"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	netpolv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

const (
	fqdnCategoryDatabase  = "database"
	fqdnCategoryMessaging = "messaging"
	fqdnCategoryExternal  = "external"
)

var policySuffixes = []string{
	"-ingress-policy", "-egress-policy", "-fqdn-network-policy",
	"-cron-egress-policy", "-cron-fqdn-network-policy",
}

// NetworkPolicyService implements the NetworkPolicyServiceHandler interface.
type NetworkPolicyService struct {
	sreportalv1connect.UnimplementedNetworkPolicyServiceHandler
	client client.Client
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=list
// +kubebuilder:rbac:groups=networking.gke.io,resources=fqdnnetworkpolicies,verbs=list

// NewNetworkPolicyService creates a new NetworkPolicyService.
func NewNetworkPolicyService(c client.Client) *NetworkPolicyService {
	return &NetworkPolicyService{client: c}
}

type graphBuilder struct {
	nodes   map[string]*netpolv1.NetpolNode
	edges   []*netpolv1.NetpolEdge
	appToNs map[string]string
}

func newGraphBuilder() *graphBuilder {
	return &graphBuilder{
		nodes:   make(map[string]*netpolv1.NetpolNode),
		edges:   make([]*netpolv1.NetpolEdge, 0),
		appToNs: make(map[string]string),
	}
}

func (g *graphBuilder) nodeID(name, ns, ntype string) string {
	return fmt.Sprintf("%s:%s:%s", ntype, ns, name)
}

func (g *graphBuilder) ensureNode(id, label, ns, ntype string) {
	if _, ok := g.nodes[id]; ok {
		return
	}
	g.nodes[id] = &netpolv1.NetpolNode{
		Id:        id,
		Label:     label,
		Namespace: ns,
		NodeType:  ntype,
		Group:     ns,
	}
}

func (g *graphBuilder) addEdge(from, to, edgeType string) {
	g.edges = append(g.edges, &netpolv1.NetpolEdge{
		From:     from,
		To:       to,
		EdgeType: edgeType,
	})
}

// buildAppToNsMap extracts application names from policy naming conventions.
func (g *graphBuilder) buildAppToNsMap(items []networkingv1.NetworkPolicy) {
	for _, np := range items {
		for _, suffix := range policySuffixes {
			if app, ok := strings.CutSuffix(np.Name, suffix); ok {
				g.appToNs[app] = np.Namespace
				break
			}
		}
	}
}

// parseIngressPolicies extracts ingress relations from NetworkPolicies.
func (g *graphBuilder) parseIngressPolicies(items []networkingv1.NetworkPolicy) {
	for _, np := range items {
		targetApp, ok := strings.CutSuffix(np.Name, "-ingress-policy")
		if !ok {
			continue
		}
		if !slices.Contains(np.Spec.PolicyTypes, networkingv1.PolicyTypeIngress) {
			continue
		}

		tgtID := g.nodeID(targetApp, np.Namespace, "service")
		g.ensureNode(tgtID, targetApp, np.Namespace, "service")

		for _, rule := range np.Spec.Ingress {
			g.parseIngressRule(rule, np.Namespace, tgtID)
		}
	}
}

func (g *graphBuilder) parseIngressRule(rule networkingv1.NetworkPolicyIngressRule, targetNs, tgtID string) {
	for _, from := range rule.From {
		if from.PodSelector == nil {
			continue
		}
		for _, expr := range from.PodSelector.MatchExpressions {
			switch {
			case expr.Key == "app.kubernetes.io/name" && expr.Operator == "In":
				for _, src := range expr.Values {
					srcNs := g.appToNs[src]
					if srcNs == "" {
						srcNs = "unknown"
					}
					srcID := g.nodeID(src, srcNs, "service")
					g.ensureNode(srcID, src, srcNs, "service")

					edgeType := "internal"
					if srcNs != targetNs {
						edgeType = "cross-ns"
					}
					g.addEdge(srcID, tgtID, edgeType)
				}
			case expr.Key == "basename" && expr.Operator == "In":
				for _, src := range expr.Values {
					srcID := g.nodeID(src, targetNs, "cron")
					g.ensureNode(srcID, src, targetNs, "cron")
					g.addEdge(srcID, tgtID, "cron")
				}
			}
		}
	}
}

// parseFQDNPolicies extracts FQDN egress relations from GKE FQDNNetworkPolicies.
func (g *graphBuilder) parseFQDNPolicies(items []unstructured.Unstructured) {
	for _, item := range items {
		ns := item.GetNamespace()
		name := item.GetName()
		isCron := strings.Contains(name, "-cron-fqdn-")
		appName := strings.TrimSuffix(strings.TrimSuffix(name, "-cron-fqdn-network-policy"), "-fqdn-network-policy")

		srcType := "service"
		if isCron {
			srcType = "cron"
		}
		srcID := g.nodeID(appName, ns, srcType)
		g.ensureNode(srcID, appName, ns, srcType)

		egressRules, _, _ := unstructured.NestedSlice(item.Object, "spec", "egress")
		for _, ruleRaw := range egressRules {
			rule, ok := ruleRaw.(map[string]any)
			if !ok {
				continue
			}
			g.parseFQDNEgressRule(rule, ns, srcID)
		}
	}
}

func (g *graphBuilder) parseFQDNEgressRule(rule map[string]any, ns, srcID string) {
	ports, _, _ := unstructured.NestedSlice(rule, "ports")
	portNums := extractPorts(ports)

	toRules, _, _ := unstructured.NestedSlice(rule, "to")
	for _, toRaw := range toRules {
		to, ok := toRaw.(map[string]any)
		if !ok {
			continue
		}
		fqdns, _, _ := unstructured.NestedStringSlice(to, "fqdns")
		for _, fqdn := range fqdns {
			cat, label := classifyFQDN(fqdn, portNums)
			dbID := g.nodeID(label, ns, cat)
			g.ensureNode(dbID, label, ns, cat)
			g.addEdge(srcID, dbID, cat)
		}
	}
}

func (g *graphBuilder) applySearch(search string) {
	matchedNodes := make(map[string]bool)
	for id, n := range g.nodes {
		if strings.Contains(strings.ToLower(n.Label), search) ||
			strings.Contains(strings.ToLower(n.Group), search) ||
			strings.Contains(strings.ToLower(n.Namespace), search) {
			matchedNodes[id] = true
		}
	}
	for _, e := range g.edges {
		if matchedNodes[e.From] {
			matchedNodes[e.To] = true
		}
		if matchedNodes[e.To] {
			matchedNodes[e.From] = true
		}
	}

	filteredNodes := make(map[string]*netpolv1.NetpolNode)
	for id, n := range g.nodes {
		if matchedNodes[id] {
			filteredNodes[id] = n
		}
	}
	g.nodes = filteredNodes

	filteredEdges := make([]*netpolv1.NetpolEdge, 0)
	for _, e := range g.edges {
		if matchedNodes[e.From] && matchedNodes[e.To] {
			filteredEdges = append(filteredEdges, e)
		}
	}
	g.edges = filteredEdges
}

func (g *graphBuilder) deduplicateEdges() []*netpolv1.NetpolEdge {
	edgeSet := make(map[string]bool)
	deduped := make([]*netpolv1.NetpolEdge, 0, len(g.edges))
	for _, e := range g.edges {
		key := e.From + "|" + e.To + "|" + e.EdgeType
		if !edgeSet[key] {
			edgeSet[key] = true
			deduped = append(deduped, e)
		}
	}
	return deduped
}

// ListNetworkPolicies returns all network policies as a graph of nodes and edges.
func (s *NetworkPolicyService) ListNetworkPolicies(
	ctx context.Context,
	req *connect.Request[netpolv1.ListNetworkPoliciesRequest],
) (*connect.Response[netpolv1.ListNetworkPoliciesResponse], error) {
	gb := newGraphBuilder()

	var npList networkingv1.NetworkPolicyList
	listOpts := []client.ListOption{}
	if req.Msg.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(req.Msg.Namespace))
	}

	if err := s.client.List(ctx, &npList, listOpts...); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	gb.buildAppToNsMap(npList.Items)
	gb.parseIngressPolicies(npList.Items)

	// Fetch FQDNNetworkPolicies (GKE CRD — silently skipped if unavailable)
	var fqdnList unstructured.UnstructuredList
	fqdnList.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "networking.gke.io", Version: "v1alpha3", Kind: "FQDNNetworkPolicyList",
	})
	if err := s.client.List(ctx, &fqdnList, listOpts...); err == nil {
		gb.parseFQDNPolicies(fqdnList.Items)
	}

	if search := strings.ToLower(req.Msg.Search); search != "" {
		gb.applySearch(search)
	}

	dedupedEdges := gb.deduplicateEdges()

	nodes := make([]*netpolv1.NetpolNode, 0, len(gb.nodes))
	for _, n := range gb.nodes {
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Group == nodes[j].Group {
			return nodes[i].Label < nodes[j].Label
		}
		return nodes[i].Group < nodes[j].Group
	})
	sort.Slice(dedupedEdges, func(i, j int) bool {
		if dedupedEdges[i].From == dedupedEdges[j].From {
			return dedupedEdges[i].To < dedupedEdges[j].To
		}
		return dedupedEdges[i].From < dedupedEdges[j].From
	})

	return connect.NewResponse(&netpolv1.ListNetworkPoliciesResponse{
		Nodes: nodes,
		Edges: dedupedEdges,
	}), nil
}

func extractPorts(ports []any) []int64 {
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

func classifyFQDN(fqdn string, ports []int64) (category, label string) {
	for _, p := range ports {
		switch p {
		case 5432, 1433, 3306:
			return fqdnCategoryDatabase, fqdn
		case 5672, 5671:
			return fqdnCategoryMessaging, fqdn
		}
	}
	return fqdnCategoryExternal, fqdn
}
