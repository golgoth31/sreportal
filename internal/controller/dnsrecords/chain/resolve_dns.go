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
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	maxDNSRecordLookups    = 10
	dnsRecordLookupTimeout = 5 * time.Second
)

// ResolveDNSHandler resolves each endpoint against DNS and persists the resulting
// SyncStatus values on the DNSRecord status. No-op when the resolver is nil,
// resolution is disabled (via ChainData.DisableDNSCheck), or the record has no
// endpoints.
type ResolveDNSHandler struct {
	client   client.Client
	resolver domaindns.Resolver
}

// NewResolveDNSHandler creates a new ResolveDNSHandler. The disableDNSCheck
// toggle is read per-reconciliation from rc.Data.DisableDNSCheck (populated by
// LoadDNSConfigHandler from the referenced DNS CR).
func NewResolveDNSHandler(c client.Client, resolver domaindns.Resolver) *ResolveDNSHandler {
	return &ResolveDNSHandler{client: c, resolver: resolver}
}

// Handle implements reconciler.Handler.
func (h *ResolveDNSHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNSRecord, ChainData]) error {
	record := rc.Resource
	if rc.Data.DisableDNSCheck || h.resolver == nil || len(record.Status.Endpoints) == 0 {
		return nil
	}

	base := record.DeepCopy()
	h.resolveEndpoints(ctx, record.Status.Endpoints)

	if !syncStatusChanged(base.Status.Endpoints, record.Status.Endpoints) {
		return nil
	}
	if err := h.client.Status().Patch(ctx, record, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch DNSRecord status: %w", err)
	}
	return nil
}

// syncStatusChanged reports whether any endpoint's SyncStatus differs between
// before and after slices. Used to avoid spurious status patches that would
// re-trigger the DNSRecord watch and cause a reconciliation loop.
func syncStatusChanged(before, after []v1alpha2.EndpointStatus) bool {
	if len(before) != len(after) {
		return true
	}
	for i := range after {
		if before[i].SyncStatus != after[i].SyncStatus {
			return true
		}
	}
	return false
}

// resolveEndpoints resolves DNS for each endpoint in-place, setting SyncStatus.
// Resolution errors (timeout, NXDOMAIN, network, ctx cancellation) collapse to
// NotAvailable in the status — but the underlying error is logged at V(1) so
// operators can distinguish a missing record from a DNS outage.
func (h *ResolveDNSHandler) resolveEndpoints(ctx context.Context, endpoints []v1alpha2.EndpointStatus) {
	logger := log.FromContext(ctx)
	workers := min(maxDNSRecordLookups, len(endpoints))
	ch := make(chan int, len(endpoints))
	for i := range endpoints {
		ch <- i
	}
	close(ch)

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for idx := range ch {
				ep := &endpoints[idx]
				lookupCtx, cancel := context.WithTimeout(ctx, dnsRecordLookupTimeout)
				result := domaindns.CheckFQDN(lookupCtx, h.resolver, ep.DNSName, ep.RecordType, ep.Targets)
				ep.SyncStatus = v1alpha2.SyncStatus(result.Status)
				if result.Err != nil {
					logger.V(1).Info("DNS resolution failed",
						"fqdn", ep.DNSName,
						"recordType", ep.RecordType,
						"status", string(result.Status),
						"err", result.Err.Error(),
					)
				}
				cancel()
			}
		})
	}
	wg.Wait()
}
