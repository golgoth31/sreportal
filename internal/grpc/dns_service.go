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
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	dnsv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// DNSService implements the DNSServiceHandler interface
type DNSService struct {
	sreportalv1connect.UnimplementedDNSServiceHandler
	client client.Client
}

// NewDNSService creates a new DNSService
func NewDNSService(c client.Client) *DNSService {
	return &DNSService{client: c}
}

// ListFQDNs returns all aggregated FQDNs from DNS resources
func (s *DNSService) ListFQDNs(
	ctx context.Context,
	req *connect.Request[dnsv1.ListFQDNsRequest],
) (*connect.Response[dnsv1.ListFQDNsResponse], error) {
	// List all DNS resources
	var dnsList sreportalv1alpha1.DNSList
	listOpts := []client.ListOption{}

	if req.Msg.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(req.Msg.Namespace))
	}

	// Apply portal filter if specified
	if req.Msg.Portal != "" {
		listOpts = append(listOpts, client.MatchingFields{"spec.portalRef": req.Msg.Portal})
	}

	if err := s.client.List(ctx, &dnsList, listOpts...); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Aggregate FQDNs from all DNS resources (flatten groups)
	// Use a map to accumulate groups for FQDNs that appear in multiple groups
	seen := make(map[string]*dnsv1.FQDN)

	for _, dns := range dnsList.Items {
		for _, group := range dns.Status.Groups {
			// Apply source filter
			if req.Msg.Source != "" && group.Source != req.Msg.Source {
				continue
			}

			for _, fqdnStatus := range group.FQDNs {
				// Apply search filter
				if req.Msg.Search != "" && !strings.Contains(
					strings.ToLower(fqdnStatus.FQDN),
					strings.ToLower(req.Msg.Search),
				) {
					continue
				}

				if existing, ok := seen[fqdnStatus.FQDN]; ok {
					// FQDN already seen, append group name if not already present
					if !slices.Contains(existing.Groups, group.Name) {
						existing.Groups = append(existing.Groups, group.Name)
					}
				} else {
					f := &dnsv1.FQDN{
						Name:                 fqdnStatus.FQDN,
						Source:               group.Source,
						Groups:               []string{group.Name},
						Description:          fqdnStatus.Description,
						RecordType:           fqdnStatus.RecordType,
						Targets:              fqdnStatus.Targets,
						LastSeen:             timestamppb.New(fqdnStatus.LastSeen.Time),
						DnsResourceName:      dns.Name,
						DnsResourceNamespace: dns.Namespace,
						OriginRef:            toProtoOriginRef(fqdnStatus.OriginRef),
						SyncStatus:           fqdnStatus.SyncStatus,
					}
					seen[fqdnStatus.FQDN] = f
				}
			}
		}
	}

	// Collect all FQDNs from the map
	fqdns := make([]*dnsv1.FQDN, 0, len(seen))
	for _, f := range seen {
		fqdns = append(fqdns, f)
	}

	return connect.NewResponse(&dnsv1.ListFQDNsResponse{
		Fqdns: fqdns,
	}), nil
}

// StreamFQDNs streams FQDN updates in real-time
func (s *DNSService) StreamFQDNs(
	ctx context.Context,
	req *connect.Request[dnsv1.StreamFQDNsRequest],
	stream *connect.ServerStream[dnsv1.StreamFQDNsResponse],
) error {
	// Initial send of all current FQDNs
	listReq := connect.NewRequest(&dnsv1.ListFQDNsRequest{
		Namespace: req.Msg.Namespace,
	})
	listResp, err := s.ListFQDNs(ctx, listReq)
	if err != nil {
		return err
	}

	// Send initial state
	for _, fqdn := range listResp.Msg.Fqdns {
		if err := stream.Send(&dnsv1.StreamFQDNsResponse{
			Type: dnsv1.UpdateType_UPDATE_TYPE_ADDED,
			Fqdn: fqdn,
		}); err != nil {
			return err
		}
	}

	// Keep streaming updates by polling every 5 seconds.
	//
	// Known limitation: each open stream issues a full ListFQDNs call on every
	// tick, so cost scales as O(streams × portals × groups). For a small number
	// of concurrent streams this is acceptable. To eliminate polling entirely,
	// replace this with a K8s informer/watch on DNS resources and push updates
	// only when the underlying data changes.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	previousFQDNs := make(map[string]*dnsv1.FQDN)
	for _, fqdn := range listResp.Msg.Fqdns {
		previousFQDNs[fqdn.Name] = fqdn
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			listResp, err := s.ListFQDNs(ctx, listReq)
			if err != nil {
				continue // Log and continue on errors
			}

			currentFQDNs := make(map[string]*dnsv1.FQDN)
			for _, fqdn := range listResp.Msg.Fqdns {
				currentFQDNs[fqdn.Name] = fqdn

				prev, exists := previousFQDNs[fqdn.Name]
				if !exists {
					// New FQDN
					if err := stream.Send(&dnsv1.StreamFQDNsResponse{
						Type: dnsv1.UpdateType_UPDATE_TYPE_ADDED,
						Fqdn: fqdn,
					}); err != nil {
						return err
					}
				} else if !fqdnEqual(prev, fqdn) {
					// Modified FQDN
					if err := stream.Send(&dnsv1.StreamFQDNsResponse{
						Type: dnsv1.UpdateType_UPDATE_TYPE_MODIFIED,
						Fqdn: fqdn,
					}); err != nil {
						return err
					}
				}
			}

			// Check for deleted FQDNs
			for name, fqdn := range previousFQDNs {
				if _, exists := currentFQDNs[name]; !exists {
					if err := stream.Send(&dnsv1.StreamFQDNsResponse{
						Type: dnsv1.UpdateType_UPDATE_TYPE_DELETED,
						Fqdn: fqdn,
					}); err != nil {
						return err
					}
				}
			}

			previousFQDNs = currentFQDNs
		}
	}
}

// fqdnEqual compares two FQDNs for equality (excluding LastSeen)
func fqdnEqual(a, b *dnsv1.FQDN) bool {
	if a.Name != b.Name || a.Source != b.Source || a.Description != b.Description {
		return false
	}
	if a.RecordType != b.RecordType || a.SyncStatus != b.SyncStatus {
		return false
	}
	if len(a.Groups) != len(b.Groups) {
		return false
	}
	for i, g := range a.Groups {
		if g != b.Groups[i] {
			return false
		}
	}
	if len(a.Targets) != len(b.Targets) {
		return false
	}
	for i, t := range a.Targets {
		if t != b.Targets[i] {
			return false
		}
	}
	return true
}

// toProtoOriginRef converts a K8s API OriginResourceRef to its proto representation.
// Returns nil when the input is nil (manual entries, or external-dns records without a resource label).
func toProtoOriginRef(ref *sreportalv1alpha1.OriginResourceRef) *dnsv1.OriginResourceRef {
	if ref == nil {
		return nil
	}
	return &dnsv1.OriginResourceRef{
		Kind:      ref.Kind,
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
}
