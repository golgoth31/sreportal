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
	"sort"
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
func (s *NetworkPolicyService) ListNetworkPolicies(
	ctx context.Context,
	req *connect.Request[netpolv1.ListNetworkPoliciesRequest],
) (*connect.Response[netpolv1.ListNetworkPoliciesResponse], error) {
	// Read nodes from FlowNodeSets
	var nodeSetList sreportalv1alpha1.FlowNodeSetList
	if err := s.client.List(ctx, &nodeSetList); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Read edges from FlowEdgeSets
	var edgeSetList sreportalv1alpha1.FlowEdgeSetList
	if err := s.client.List(ctx, &edgeSetList); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Merge all nodes and edges
	nodeMap := make(map[string]*netpolv1.NetpolNode)
	for _, ns := range nodeSetList.Items {
		for _, n := range ns.Status.Nodes {
			if _, ok := nodeMap[n.ID]; !ok {
				nodeMap[n.ID] = &netpolv1.NetpolNode{
					Id: n.ID, Label: n.Label, Namespace: n.Namespace, NodeType: n.NodeType, Group: n.Group,
				}
			}
		}
	}

	edgeSet := make(map[string]*netpolv1.NetpolEdge)
	for _, es := range edgeSetList.Items {
		for _, e := range es.Status.Edges {
			key := e.From + "|" + e.To + "|" + e.EdgeType
			if _, ok := edgeSet[key]; !ok {
				edgeSet[key] = &netpolv1.NetpolEdge{From: e.From, To: e.To, EdgeType: e.EdgeType}
			}
		}
	}

	// Apply search filter
	search := strings.ToLower(req.Msg.Search)
	if search != "" {
		matched := make(map[string]bool)
		for id, n := range nodeMap {
			if strings.Contains(strings.ToLower(n.Label), search) ||
				strings.Contains(strings.ToLower(n.Group), search) ||
				strings.Contains(strings.ToLower(n.Namespace), search) {
				matched[id] = true
			}
		}
		for _, e := range edgeSet {
			if matched[e.From] {
				matched[e.To] = true
			}
			if matched[e.To] {
				matched[e.From] = true
			}
		}
		for id := range nodeMap {
			if !matched[id] {
				delete(nodeMap, id)
			}
		}
		for key, e := range edgeSet {
			if !matched[e.From] || !matched[e.To] {
				delete(edgeSet, key)
			}
		}
	}

	nodes := make([]*netpolv1.NetpolNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Group == nodes[j].Group {
			return nodes[i].Label < nodes[j].Label
		}
		return nodes[i].Group < nodes[j].Group
	})

	edges := make([]*netpolv1.NetpolEdge, 0, len(edgeSet))
	for _, e := range edgeSet {
		edges = append(edges, e)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})

	return connect.NewResponse(&netpolv1.ListNetworkPoliciesResponse{
		Nodes: nodes,
		Edges: edges,
	}), nil
}
