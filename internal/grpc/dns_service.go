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
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	dnsv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// streamPollInterval is how often the shared FQDN cache is refreshed and
// stream subscribers are notified.
const streamPollInterval = 5 * time.Second

// DNSService implements the DNSServiceHandler interface.
//
// StreamFQDNs efficiency: a single background goroutine (Start) refreshes an
// in-memory snapshot of all FQDNs every streamPollInterval. Concurrent stream
// subscribers read from this shared cache and apply their own filters locally,
// so the K8s API call count is O(1 per tick) regardless of stream count.
type DNSService struct {
	sreportalv1connect.UnimplementedDNSServiceHandler
	client client.Client

	// cacheMu guards cacheAll.
	cacheMu  sync.RWMutex
	cacheAll []*dnsv1.FQDN // full unfiltered sorted snapshot; nil until first fetch

	// cacheReady is closed once after the first successful cache fetch.
	// StreamFQDNs goroutines block on it before sending the initial state.
	cacheReady chan struct{}

	// cacheUpdateMu guards cacheUpdate.
	// On each cache refresh, the current channel is closed (broadcasting to all
	// waiting goroutines) and replaced with a new open channel.
	cacheUpdateMu sync.Mutex
	cacheUpdate   chan struct{}
}

// NewDNSService creates a new DNSService.
func NewDNSService(c client.Client) *DNSService {
	return &DNSService{
		client:      c,
		cacheReady:  make(chan struct{}),
		cacheUpdate: make(chan struct{}),
	}
}

// Start implements manager.Runnable. It runs a cache-refresh loop that fetches
// all FQDNs from K8s every streamPollInterval and notifies waiting StreamFQDNs
// goroutines. It exits when ctx is cancelled.
func (s *DNSService) Start(ctx context.Context) error {
	readyOnce := sync.Once{}
	ticker := time.NewTicker(streamPollInterval)
	defer ticker.Stop()

	// Fetch immediately so the first stream doesn't wait a full interval.
	s.refreshCache(ctx, &readyOnce)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			s.refreshCache(ctx, &readyOnce)
		}
	}
}

// refreshCache fetches all DNS resources, builds the FQDN snapshot, and
// broadcasts to waiting StreamFQDNs goroutines.
func (s *DNSService) refreshCache(ctx context.Context, readyOnce *sync.Once) {
	log := logf.FromContext(ctx).WithName("dns-cache")
	fqdns, err := s.fetchAllFQDNs(ctx)
	if err != nil {
		log.Error(err, "failed to refresh FQDN cache, retaining previous snapshot")
		return
	}

	// Swap the snapshot under write-lock.
	s.cacheMu.Lock()
	s.cacheAll = fqdns
	s.cacheMu.Unlock()

	// Signal all waiting stream goroutines by closing the current update channel
	// and replacing it with a fresh one.
	s.cacheUpdateMu.Lock()
	old := s.cacheUpdate
	s.cacheUpdate = make(chan struct{})
	s.cacheUpdateMu.Unlock()
	close(old)

	// Close cacheReady exactly once so StreamFQDNs goroutines unblock.
	readyOnce.Do(func() { close(s.cacheReady) })
}

// fetchAllFQDNs lists all DNS resources and returns the full FQDN set sorted
// deterministically by (name, record_type). No filters are applied here;
// filtering is done per-stream in StreamFQDNs.
func (s *DNSService) fetchAllFQDNs(ctx context.Context) ([]*dnsv1.FQDN, error) {
	var dnsList sreportalv1alpha1.DNSList
	if err := s.client.List(ctx, &dnsList); err != nil {
		return nil, err
	}

	seen := make(map[string]*dnsv1.FQDN)
	for _, dns := range dnsList.Items {
		for _, group := range dns.Status.Groups {
			for _, fqdnStatus := range group.FQDNs {
				key := fqdnStatus.FQDN + "/" + fqdnStatus.RecordType
				if existing, ok := seen[key]; ok {
					if !slices.Contains(existing.Groups, group.Name) {
						existing.Groups = append(existing.Groups, group.Name)
					}
				} else {
					seen[key] = &dnsv1.FQDN{
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
				}
			}
		}
	}

	fqdns := make([]*dnsv1.FQDN, 0, len(seen))
	for _, f := range seen {
		fqdns = append(fqdns, f)
	}
	sort.Slice(fqdns, func(i, j int) bool {
		if fqdns[i].Name == fqdns[j].Name {
			return fqdns[i].RecordType < fqdns[j].RecordType
		}
		return fqdns[i].Name < fqdns[j].Name
	})
	return fqdns, nil
}

// currentUpdate returns the current update channel under the update mutex.
// Callers should select on the returned channel to know when a new snapshot
// is available, then call snapshotCache to read it.
func (s *DNSService) currentUpdate() chan struct{} {
	s.cacheUpdateMu.Lock()
	defer s.cacheUpdateMu.Unlock()
	return s.cacheUpdate
}

// snapshotCache returns a copy of the cached FQDN slice under read-lock.
func (s *DNSService) snapshotCache() []*dnsv1.FQDN {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	out := make([]*dnsv1.FQDN, len(s.cacheAll))
	copy(out, s.cacheAll)
	return out
}

// applyStreamFilters filters fqdns in-place (returns a new slice) according
// to the fields in a StreamFQDNsRequest.
func applyStreamFilters(fqdns []*dnsv1.FQDN, req *dnsv1.StreamFQDNsRequest) []*dnsv1.FQDN {
	if req.Namespace == "" && req.Portal == "" && req.Source == "" && req.Search == "" {
		return fqdns
	}
	out := fqdns[:0:0] // fresh slice, zero alloc when all pass
	searchLower := strings.ToLower(req.Search)
	for _, f := range fqdns {
		if req.Namespace != "" && f.DnsResourceNamespace != req.Namespace {
			continue
		}
		if req.Source != "" && f.Source != req.Source {
			continue
		}
		if req.Search != "" && !strings.Contains(strings.ToLower(f.Name), searchLower) {
			continue
		}
		// Portal filter: the stream request carries a portal name; we match it
		// against DnsResourceName (which is the DNS CR name == portal name).
		if req.Portal != "" && f.DnsResourceName != req.Portal {
			continue
		}
		out = append(out, f)
	}
	return out
}

// ListFQDNs returns all aggregated FQDNs from DNS resources, with optional
// filters and cursor-based pagination.
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

	// Aggregate FQDNs from all DNS resources (flatten groups).
	// Deduplication key is DNSName+RecordType: the same hostname can have both
	// an A and a CNAME record and both must be preserved as separate entries.
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

				key := fqdnStatus.FQDN + "/" + fqdnStatus.RecordType
				if existing, ok := seen[key]; ok {
					// Same FQDN+RecordType already seen: append group name if not present
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
					seen[key] = f
				}
			}
		}
	}

	// Collect FQDNs from map and sort for deterministic response ordering
	fqdns := make([]*dnsv1.FQDN, 0, len(seen))
	for _, f := range seen {
		fqdns = append(fqdns, f)
	}
	sort.Slice(fqdns, func(i, j int) bool {
		if fqdns[i].Name == fqdns[j].Name {
			return fqdns[i].RecordType < fqdns[j].RecordType
		}
		return fqdns[i].Name < fqdns[j].Name
	})

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

// StreamFQDNs streams FQDN updates in real-time.
//
// It reads from the shared FQDN cache maintained by Start() rather than issuing
// its own K8s List calls, so the cost is O(1 K8s call per tick) regardless of the
// number of concurrent streams.
func (s *DNSService) StreamFQDNs(
	ctx context.Context,
	req *connect.Request[dnsv1.StreamFQDNsRequest],
	stream *connect.ServerStream[dnsv1.StreamFQDNsResponse],
) error {
	// Block until the first cache fetch has completed.
	select {
	case <-ctx.Done():
		return nil
	case <-s.cacheReady:
	}

	// Send initial state from the current cache snapshot.
	snapshot := applyStreamFilters(s.snapshotCache(), req.Msg)
	for _, fqdn := range snapshot {
		if err := stream.Send(&dnsv1.StreamFQDNsResponse{
			Type: dnsv1.UpdateType_UPDATE_TYPE_ADDED,
			Fqdn: fqdn,
		}); err != nil {
			return err
		}
	}

	// Build the previous-state map for diffing.
	previousFQDNs := make(map[string]*dnsv1.FQDN, len(snapshot))
	for _, fqdn := range snapshot {
		previousFQDNs[fqdn.Name+"/"+fqdn.RecordType] = fqdn
	}

	// Poll the cache on every update signal instead of issuing a K8s call.
	for {
		// Grab the current update channel before blocking so we don't miss a
		// refresh that happens between the select and the snapshot read.
		updateCh := s.currentUpdate()

		select {
		case <-ctx.Done():
			return nil
		case <-updateCh:
		}

		filtered := applyStreamFilters(s.snapshotCache(), req.Msg)

		currentFQDNs := make(map[string]*dnsv1.FQDN, len(filtered))
		for _, fqdn := range filtered {
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
