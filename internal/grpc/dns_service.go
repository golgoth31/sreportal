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
	"encoding/base64"
	"strconv"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	dnsv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// DNSService implements the DNSServiceHandler interface.
// It reads pre-aggregated FQDNViews from a FQDNReader (ReadStore) instead of
// querying K8s directly. StreamFQDNs uses the reader's Subscribe() channel
// for event-driven notifications instead of polling.
type DNSService struct {
	sreportalv1connect.UnimplementedDNSServiceHandler
	reader domaindns.FQDNReader
}

// NewDNSService creates a new DNSService backed by a FQDNReader.
func NewDNSService(reader domaindns.FQDNReader) *DNSService {
	return &DNSService{reader: reader}
}

// ListFQDNs returns all aggregated FQDNs with optional filters and cursor-based pagination.
func (s *DNSService) ListFQDNs(
	ctx context.Context,
	req *connect.Request[dnsv1.ListFQDNsRequest],
) (*connect.Response[dnsv1.ListFQDNsResponse], error) {
	filters := domaindns.FQDNFilters{
		Portal:    req.Msg.Portal,
		Namespace: req.Msg.Namespace,
		Source:    req.Msg.Source,
		Search:    req.Msg.Search,
	}

	views, err := s.reader.List(ctx, filters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	fqdns := make([]*dnsv1.FQDN, 0, len(views))
	for _, v := range views {
		fqdns = append(fqdns, fqdnViewToProto(v))
	}

	// Pagination: page_size=0 means return all (backward-compatible default).
	totalSize := int32(len(fqdns))
	var nextPageToken string
	if req.Msg.PageSize > 0 {
		offset := decodePageToken(req.Msg.PageToken)
		if offset < 0 || offset > len(fqdns) {
			offset = 0
		}
		end := offset + int(req.Msg.PageSize)
		if end < len(fqdns) {
			nextPageToken = encodePageToken(end)
			fqdns = fqdns[offset:end]
		} else {
			fqdns = fqdns[offset:]
		}
	}

	return connect.NewResponse(&dnsv1.ListFQDNsResponse{
		Fqdns:         fqdns,
		NextPageToken: nextPageToken,
		TotalSize:     totalSize,
	}), nil
}

// StreamFQDNs streams FQDN updates in real-time using the ReadStore's
// Subscribe() notification channel instead of polling.
func (s *DNSService) StreamFQDNs(
	ctx context.Context,
	req *connect.Request[dnsv1.StreamFQDNsRequest],
	stream *connect.ServerStream[dnsv1.StreamFQDNsResponse],
) error {
	filters := domaindns.FQDNFilters{
		Portal:    req.Msg.Portal,
		Namespace: req.Msg.Namespace,
		Source:    req.Msg.Source,
		Search:    req.Msg.Search,
	}

	// Send initial state.
	views, err := s.reader.List(ctx, filters)
	if err != nil {
		return err
	}
	for _, v := range views {
		if err := stream.Send(&dnsv1.StreamFQDNsResponse{
			Type: dnsv1.UpdateType_UPDATE_TYPE_ADDED,
			Fqdn: fqdnViewToProto(v),
		}); err != nil {
			return err
		}
	}

	// Build previous-state map for diffing.
	previousFQDNs := make(map[string]*dnsv1.FQDN, len(views))
	for _, v := range views {
		proto := fqdnViewToProto(v)
		previousFQDNs[proto.Name+"/"+proto.RecordType] = proto
	}

	// Wait for store notifications and diff.
	for {
		updateCh := s.reader.Subscribe()

		select {
		case <-ctx.Done():
			return nil
		case <-updateCh:
		}

		views, err = s.reader.List(ctx, filters)
		if err != nil {
			return err
		}

		currentFQDNs := make(map[string]*dnsv1.FQDN, len(views))
		for _, v := range views {
			fqdn := fqdnViewToProto(v)
			key := fqdn.Name + "/" + fqdn.RecordType
			currentFQDNs[key] = fqdn

			prev, exists := previousFQDNs[key]
			if !exists {
				if err := stream.Send(&dnsv1.StreamFQDNsResponse{
					Type: dnsv1.UpdateType_UPDATE_TYPE_ADDED,
					Fqdn: fqdn,
				}); err != nil {
					return err
				}
			} else if !fqdnEqual(prev, fqdn) {
				if err := stream.Send(&dnsv1.StreamFQDNsResponse{
					Type: dnsv1.UpdateType_UPDATE_TYPE_MODIFIED,
					Fqdn: fqdn,
				}); err != nil {
					return err
				}
			}
		}

		for key, fqdn := range previousFQDNs {
			if _, exists := currentFQDNs[key]; !exists {
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

// fqdnViewToProto converts a domain FQDNView to its proto representation.
func fqdnViewToProto(v domaindns.FQDNView) *dnsv1.FQDN {
	f := &dnsv1.FQDN{
		Name:                 v.Name,
		Source:               string(v.Source),
		Groups:               v.Groups,
		Description:          v.Description,
		RecordType:           v.RecordType,
		Targets:              v.Targets,
		LastSeen:             timestamppb.New(v.LastSeen),
		DnsResourceName:      v.PortalName,
		DnsResourceNamespace: v.Namespace,
		SyncStatus:           v.SyncStatus,
	}
	if v.OriginRef != nil {
		f.OriginRef = &dnsv1.OriginResourceRef{
			Kind:      v.OriginRef.Kind(),
			Namespace: v.OriginRef.Namespace(),
			Name:      v.OriginRef.Name(),
		}
	}
	return f
}

// fqdnEqual compares two FQDNs for equality (excluding LastSeen).
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

// decodePageToken decodes an opaque page cursor back to an integer offset.
// Returns 0 on any error (empty string, invalid base64, invalid integer).
func decodePageToken(token string) int {
	if token == "" {
		return 0
	}
	b, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(string(b))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// encodePageToken encodes an integer offset as an opaque page cursor.
func encodePageToken(offset int) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}
