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
	"slices"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	netpolv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// NetworkPolicyService implements the NetworkPolicyServiceHandler interface.
// It reads pre-computed flow graphs from the in-memory FlowGraphReader.
type NetworkPolicyService struct {
	sreportalv1connect.UnimplementedNetworkPolicyServiceHandler
	reader       domainnetpol.FlowGraphReader
	portalReader domainportal.PortalReader
}

// NewNetworkPolicyService creates a new NetworkPolicyService.
func NewNetworkPolicyService(reader domainnetpol.FlowGraphReader, portalReader domainportal.PortalReader) *NetworkPolicyService {
	return &NetworkPolicyService{reader: reader, portalReader: portalReader}
}

// ListNetworkPolicies returns the flow graph filtered by portal, namespace, and search.
func (s *NetworkPolicyService) ListNetworkPolicies(
	ctx context.Context,
	req *connect.Request[netpolv1.ListNetworkPoliciesRequest],
) (*connect.Response[netpolv1.ListNetworkPoliciesResponse], error) {
	if enabled, err := IsFeatureEnabled(ctx, s.portalReader, req.Msg.Portal, CheckNetworkPolicy); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if !enabled {
		return connect.NewResponse(&netpolv1.ListNetworkPoliciesResponse{}), nil
	}

	filters := domainnetpol.FlowGraphFilters{
		Portal:    req.Msg.Portal,
		Namespace: req.Msg.Namespace,
		Search:    req.Msg.Search,
	}

	nodes, err := s.reader.ListNodes(ctx, filters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	edges, err := s.reader.ListEdges(ctx, filters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&netpolv1.ListNetworkPoliciesResponse{
		Nodes: sortedProtoNodes(nodes),
		Edges: sortedProtoEdges(edges),
	}), nil
}

func flowNodeToProto(n domainnetpol.FlowNode) *netpolv1.NetpolNode {
	return &netpolv1.NetpolNode{
		Id:        n.ID,
		Label:     n.Label,
		Namespace: n.Namespace,
		NodeType:  n.NodeType,
		Group:     n.Group,
	}
}

func flowEdgeToProto(e domainnetpol.FlowEdge) *netpolv1.NetpolEdge {
	proto := &netpolv1.NetpolEdge{
		From:     e.From,
		To:       e.To,
		EdgeType: e.EdgeType,
	}

	if e.LastSeen != nil {
		proto.LastSeen = timestamppb.New(*e.LastSeen)
	}

	return proto
}

func sortedProtoNodes(nodes []domainnetpol.FlowNode) []*netpolv1.NetpolNode {
	protos := make([]*netpolv1.NetpolNode, len(nodes))
	for i, n := range nodes {
		protos[i] = flowNodeToProto(n)
	}

	slices.SortFunc(protos, func(a, b *netpolv1.NetpolNode) int {
		if c := cmp.Compare(a.Group, b.Group); c != 0 {
			return c
		}

		return cmp.Compare(a.Label, b.Label)
	})

	return protos
}

func sortedProtoEdges(edges []domainnetpol.FlowEdge) []*netpolv1.NetpolEdge {
	protos := make([]*netpolv1.NetpolEdge, len(edges))
	for i, e := range edges {
		protos[i] = flowEdgeToProto(e)
	}

	slices.SortFunc(protos, func(a, b *netpolv1.NetpolEdge) int {
		if c := cmp.Compare(a.From, b.From); c != 0 {
			return c
		}

		return cmp.Compare(a.To, b.To)
	})

	return protos
}
