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

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	maxDNSRecordLookups    = 10
	dnsRecordLookupTimeout = 5 * time.Second
)

// ResolveDNSHandler resolves each endpoint against DNS and persists the resulting
// SyncStatus values on the DNSRecord status. No-op when the resolver is nil,
// resolution is disabled, or the record has no endpoints.
type ResolveDNSHandler struct {
	client          client.Client
	resolver        domaindns.Resolver
	disableDNSCheck bool
}

// NewResolveDNSHandler creates a new ResolveDNSHandler.
func NewResolveDNSHandler(c client.Client, resolver domaindns.Resolver, disableDNSCheck bool) *ResolveDNSHandler {
	return &ResolveDNSHandler{client: c, resolver: resolver, disableDNSCheck: disableDNSCheck}
}

// Handle implements reconciler.Handler.
func (h *ResolveDNSHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DNSRecord, ChainData]) error {
	record := rc.Resource
	if h.disableDNSCheck || h.resolver == nil || len(record.Status.Endpoints) == 0 {
		return nil
	}

	base := record.DeepCopy()
	h.resolveEndpoints(ctx, record.Status.Endpoints)
	if err := h.client.Status().Patch(ctx, record, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch DNSRecord status: %w", err)
	}
	return nil
}

// resolveEndpoints resolves DNS for each endpoint in-place, setting SyncStatus.
func (h *ResolveDNSHandler) resolveEndpoints(ctx context.Context, endpoints []sreportalv1alpha1.EndpointStatus) {
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
				ep.SyncStatus = string(result.Status)
				cancel()
			}
		})
	}
	wg.Wait()
}
