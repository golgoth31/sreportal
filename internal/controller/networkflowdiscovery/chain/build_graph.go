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

package chain

import (
	"context"
	"fmt"
	"slices"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	nodeTypeService = "service"
	nodeTypeCron    = "cron"
)

var policySuffixes = []string{
	"-ingress-policy", "-egress-policy", "-fqdn-network-policy",
	"-cron-egress-policy", "-cron-fqdn-network-policy",
}

// BuildGraphHandler lists NetworkPolicies and FQDNNetworkPolicies, then builds
// the flow graph and stores it in rc.Data.
type BuildGraphHandler struct {
	client client.Client
}

// NewBuildGraphHandler creates a new BuildGraphHandler.
func NewBuildGraphHandler(c client.Client) *BuildGraphHandler {
	return &BuildGraphHandler{client: c}
}

// Handle implements reconciler.Handler.
// This handler is a no-op for remote resources (data is populated by FetchRemoteGraphHandler).
func (h *BuildGraphHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, ChainData]) error {
	if rc.Resource.Spec.IsRemote {
		return nil
	}

	logger := log.FromContext(ctx).WithName("build-graph")

	gb := &graphBuilder{
		nodes:   make(map[string]sreportalv1alpha1.FlowNode),
		appToNs: make(map[string]string),
	}

	listOpts := h.buildListOpts(rc.Resource.Spec.Namespaces)

	// Fetch standard NetworkPolicies
	var npList networkingv1.NetworkPolicyList
	if err := h.client.List(ctx, &npList, listOpts...); err != nil {
		return fmt.Errorf("list NetworkPolicies: %w", err)
	}

	gb.buildAppToNsMap(npList.Items)
	gb.parseIngressPolicies(npList.Items)

	// Fetch FQDNNetworkPolicies (GKE CRD — silently skipped if unavailable)
	var fqdnList unstructured.UnstructuredList
	fqdnList.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "networking.gke.io", Version: "v1alpha3", Kind: "FQDNNetworkPolicyList",
	})
	if err := h.client.List(ctx, &fqdnList, listOpts...); err == nil {
		gb.parseFQDNPolicies(fqdnList.Items)
	}

	gb.deduplicateEdges()

	rc.Data.Nodes = gb.sortedNodes()
	rc.Data.Edges = gb.sortedEdges()

	logger.V(1).Info("graph built", "nodes", len(rc.Data.Nodes), "edges", len(rc.Data.Edges))

	return nil
}

func (h *BuildGraphHandler) buildListOpts(namespaces []string) []client.ListOption {
	if len(namespaces) == 1 {
		return []client.ListOption{client.InNamespace(namespaces[0])}
	}
	return nil
}

// graphBuilder accumulates nodes and edges during parsing.
type graphBuilder struct {
	nodes   map[string]sreportalv1alpha1.FlowNode
	edges   []sreportalv1alpha1.FlowEdge
	appToNs map[string]string
}

func (g *graphBuilder) nodeID(name, ns, ntype string) string {
	return fmt.Sprintf("%s:%s:%s", ntype, ns, name)
}

func (g *graphBuilder) ensureNode(id, label, ns, ntype string) {
	if _, ok := g.nodes[id]; ok {
		return
	}
	g.nodes[id] = sreportalv1alpha1.FlowNode{
		ID: id, Label: label, Namespace: ns, NodeType: ntype, Group: ns,
	}
}

func (g *graphBuilder) addEdge(from, to, edgeType string) {
	g.edges = append(g.edges, sreportalv1alpha1.FlowEdge{From: from, To: to, EdgeType: edgeType})
}

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

func (g *graphBuilder) parseIngressPolicies(items []networkingv1.NetworkPolicy) {
	for _, np := range items {
		targetApp, ok := strings.CutSuffix(np.Name, "-ingress-policy")
		if !ok || !slices.Contains(np.Spec.PolicyTypes, networkingv1.PolicyTypeIngress) {
			continue
		}

		tgtID := g.nodeID(targetApp, np.Namespace, nodeTypeService)
		g.ensureNode(tgtID, targetApp, np.Namespace, nodeTypeService)

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
					srcID := g.nodeID(src, srcNs, nodeTypeService)
					g.ensureNode(srcID, src, srcNs, nodeTypeService)
					edgeType := "internal"
					if srcNs != targetNs {
						edgeType = "cross-ns"
					}
					g.addEdge(srcID, tgtID, edgeType)
				}
			case expr.Key == "basename" && expr.Operator == "In":
				for _, src := range expr.Values {
					srcID := g.nodeID(src, targetNs, nodeTypeCron)
					g.ensureNode(srcID, src, targetNs, nodeTypeCron)
					g.addEdge(srcID, tgtID, nodeTypeCron)
				}
			}
		}
	}
}

func (g *graphBuilder) parseFQDNPolicies(items []unstructured.Unstructured) {
	for _, item := range items {
		ns := item.GetNamespace()
		name := item.GetName()
		isCron := strings.Contains(name, "-cron-fqdn-")
		appName := strings.TrimSuffix(strings.TrimSuffix(name, "-cron-fqdn-network-policy"), "-fqdn-network-policy")

		srcType := nodeTypeService
		if isCron {
			srcType = nodeTypeCron
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

func (g *graphBuilder) deduplicateEdges() {
	edgeSet := make(map[string]bool)
	deduped := make([]sreportalv1alpha1.FlowEdge, 0, len(g.edges))
	for _, e := range g.edges {
		key := e.From + "|" + e.To + "|" + e.EdgeType
		if !edgeSet[key] {
			edgeSet[key] = true
			deduped = append(deduped, e)
		}
	}
	g.edges = deduped
}

func (g *graphBuilder) sortedNodes() []sreportalv1alpha1.FlowNode {
	nodes := make([]sreportalv1alpha1.FlowNode, 0, len(g.nodes))
	for _, n := range g.nodes {
		nodes = append(nodes, n)
	}
	slices.SortFunc(nodes, func(a, b sreportalv1alpha1.FlowNode) int {
		if c := strings.Compare(a.Group, b.Group); c != 0 {
			return c
		}
		return strings.Compare(a.Label, b.Label)
	})
	return nodes
}

func (g *graphBuilder) sortedEdges() []sreportalv1alpha1.FlowEdge {
	slices.SortFunc(g.edges, func(a, b sreportalv1alpha1.FlowEdge) int {
		if c := strings.Compare(a.From, b.From); c != 0 {
			return c
		}
		return strings.Compare(a.To, b.To)
	})
	return g.edges
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
			return "database", fqdn
		case 5672, 5671:
			return "messaging", fqdn
		}
	}
	return "external", fqdn
}
