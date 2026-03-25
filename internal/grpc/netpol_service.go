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
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	netpolv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// NetworkPolicyService implements the NetworkPolicyServiceHandler interface.
// It reads pre-computed flow graphs from FlowNodeSet and FlowEdgeSet CRDs.
type NetworkPolicyService struct {
	sreportalv1connect.UnimplementedNetworkPolicyServiceHandler
	client client.Client
}

// NewNetworkPolicyService creates a new NetworkPolicyService.
func NewNetworkPolicyService(c client.Client) *NetworkPolicyService {
	return &NetworkPolicyService{client: c}
}

// ListNetworkPolicies returns the pre-computed flow graph from FlowNodeSet and FlowEdgeSet CRDs.
// When a portal filter is provided, only FlowNodeSet/FlowEdgeSet whose discoveryRef
// matches a NetworkFlowDiscovery linked to that portal are included.
func (s *NetworkPolicyService) ListNetworkPolicies(
	ctx context.Context,
	req *connect.Request[netpolv1.ListNetworkPoliciesRequest],
) (*connect.Response[netpolv1.ListNetworkPoliciesResponse], error) {
	allowedDiscoveries, err := s.resolveAllowedDiscoveries(ctx, req.Msg.Portal)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var nodeSetList sreportalv1alpha1.FlowNodeSetList
	if err := s.client.List(ctx, &nodeSetList); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var edgeSetList sreportalv1alpha1.FlowEdgeSetList
	if err := s.client.List(ctx, &edgeSetList); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	nodeMap := mergeNodes(nodeSetList.Items, allowedDiscoveries)
	edgeMap := mergeEdges(edgeSetList.Items, allowedDiscoveries)

	filterByNamespace(nodeMap, edgeMap, req.Msg.Namespace)
	filterBySearch(nodeMap, edgeMap, req.Msg.Search)

	return connect.NewResponse(&netpolv1.ListNetworkPoliciesResponse{
		Nodes: sortedNodes(nodeMap),
		Edges: sortedEdges(edgeMap),
	}), nil
}

// resolveAllowedDiscoveries returns the set of NetworkFlowDiscovery names linked to the
// given portal. Returns nil when no portal filter is specified (all discoveries allowed).
func (s *NetworkPolicyService) resolveAllowedDiscoveries(ctx context.Context, portal string) (map[string]struct{}, error) {
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

// mergeNodes deduplicates nodes from multiple FlowNodeSets, filtering by allowed discoveries.
func mergeNodes(nodeSets []sreportalv1alpha1.FlowNodeSet, allowed map[string]struct{}) map[string]*netpolv1.NetpolNode {
	nodeMap := make(map[string]*netpolv1.NetpolNode)

	for _, ns := range nodeSets {
		if allowed != nil {
			if _, ok := allowed[ns.Spec.DiscoveryRef]; !ok {
				continue
			}
		}

		for _, n := range ns.Status.Nodes {
			if _, ok := nodeMap[n.ID]; !ok {
				nodeMap[n.ID] = &netpolv1.NetpolNode{
					Id: n.ID, Label: n.Label, Namespace: n.Namespace, NodeType: n.NodeType, Group: n.Group,
				}
			}
		}
	}

	return nodeMap
}

// mergeEdges deduplicates edges from multiple FlowEdgeSets, filtering by allowed discoveries.
func mergeEdges(edgeSets []sreportalv1alpha1.FlowEdgeSet, allowed map[string]struct{}) map[string]*netpolv1.NetpolEdge {
	edgeMap := make(map[string]*netpolv1.NetpolEdge)

	for _, es := range edgeSets {
		if allowed != nil {
			if _, ok := allowed[es.Spec.DiscoveryRef]; !ok {
				continue
			}
		}

		for _, e := range es.Status.Edges {
			key := e.From + "|" + e.To + "|" + e.EdgeType
			if _, ok := edgeMap[key]; !ok {
				edgeMap[key] = &netpolv1.NetpolEdge{From: e.From, To: e.To, EdgeType: e.EdgeType}
			}
		}
	}

	return edgeMap
}

// filterByNamespace removes nodes not in the given namespace and edges referencing removed nodes.
// No-op when namespace is empty.
func filterByNamespace(nodeMap map[string]*netpolv1.NetpolNode, edgeMap map[string]*netpolv1.NetpolEdge, namespace string) {
	if namespace == "" {
		return
	}

	for id, n := range nodeMap {
		if n.Namespace != namespace {
			delete(nodeMap, id)
		}
	}

	pruneOrphanEdges(nodeMap, edgeMap)
}

// filterBySearch keeps only nodes matching the search term and their direct neighbors (1 hop).
// The neighbor expansion is deterministic: only nodes directly connected to a search-matched
// node are included, regardless of map iteration order.
// No-op when search is empty.
func filterBySearch(nodeMap map[string]*netpolv1.NetpolNode, edgeMap map[string]*netpolv1.NetpolEdge, search string) {
	search = strings.ToLower(search)
	if search == "" {
		return
	}

	// First pass: mark nodes that directly match the search term.
	directMatch := make(map[string]bool)
	for id, n := range nodeMap {
		if strings.Contains(strings.ToLower(n.Label), search) ||
			strings.Contains(strings.ToLower(n.Group), search) ||
			strings.Contains(strings.ToLower(n.Namespace), search) {
			directMatch[id] = true
		}
	}

	// Second pass: expand to 1-hop neighbors using only the direct match set
	// so the result is independent of map iteration order.
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

	pruneOrphanEdges(nodeMap, edgeMap)
}

// pruneOrphanEdges removes edges whose source or target node is no longer in nodeMap.
func pruneOrphanEdges(nodeMap map[string]*netpolv1.NetpolNode, edgeMap map[string]*netpolv1.NetpolEdge) {
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

// sortedNodes converts a node map to a deterministically sorted slice (by group, then label).
func sortedNodes(nodeMap map[string]*netpolv1.NetpolNode) []*netpolv1.NetpolNode {
	nodes := make([]*netpolv1.NetpolNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}

	slices.SortFunc(nodes, func(a, b *netpolv1.NetpolNode) int {
		if c := cmp.Compare(a.Group, b.Group); c != 0 {
			return c
		}

		return cmp.Compare(a.Label, b.Label)
	})

	return nodes
}

// sortedEdges converts an edge map to a deterministically sorted slice (by from, then to).
func sortedEdges(edgeMap map[string]*netpolv1.NetpolEdge) []*netpolv1.NetpolEdge {
	edges := make([]*netpolv1.NetpolEdge, 0, len(edgeMap))
	for _, e := range edgeMap {
		edges = append(edges, e)
	}

	slices.SortFunc(edges, func(a, b *netpolv1.NetpolEdge) int {
		if c := cmp.Compare(a.From, b.From); c != 0 {
			return c
		}

		return cmp.Compare(a.To, b.To)
	})

	return edges
}
