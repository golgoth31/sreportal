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

package dnsresolve

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnschain "github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

const (
	lookupTimeout = 2 * time.Second
	maxConcurrent = 10
)

// Runnable resolves DNSRecord endpoints out-of-band and keeps both the
// DNSRecord status and the FQDN read store in sync.
type Runnable struct {
	Client     client.Client
	Resolver   domaindns.Resolver
	FQDNWriter domaindns.FQDNWriter
}

// resolveRecord resolves the requested keys of rec (in parallel, bounded),
// writes SyncStatus onto rec.Status.Endpoints (matched by DNSName+RecordType),
// patches the status subresource, then refreshes the read store. groupMapping
// may be nil.
func (r *Runnable) resolveRecord(
	ctx context.Context,
	rec *v1alpha2.DNSRecord,
	groupMapping *v1alpha2.GroupMappingSpec,
	keys []FQDNKey,
) error {
	want := make(map[FQDNKey]struct{}, len(keys))
	for _, k := range keys {
		want[k] = struct{}{}
	}
	recordKey := rec.Namespace + "/" + rec.Name

	// Collect indices of endpoints to resolve.
	var indices []int
	for i := range rec.Status.Endpoints {
		ep := rec.Status.Endpoints[i]
		if _, ok := want[FQDNKey{RecordKey: recordKey, DNSName: ep.DNSName, RecordType: ep.RecordType}]; ok {
			indices = append(indices, i)
		}
	}
	if len(indices) == 0 {
		return nil
	}

	base := rec.DeepCopy()

	// Resolve in parallel with a bounded worker pool.
	idxCh := make(chan int, len(indices))
	for _, i := range indices {
		idxCh <- i
	}
	close(idxCh)
	workers := min(maxConcurrent, len(indices))
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range idxCh {
				ep := &rec.Status.Endpoints[i]
				lc, cancel := context.WithTimeout(ctx, lookupTimeout)
				res := domaindns.CheckFQDN(lc, r.Resolver, ep.DNSName, ep.RecordType, ep.Targets)
				cancel()
				ep.SyncStatus = v1alpha2.SyncStatus(res.Status)
			}
		})
	}
	wg.Wait()

	if err := r.Client.Status().Patch(ctx, rec, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch DNSRecord status: %w", err)
	}
	if r.FQDNWriter != nil {
		views := dnschain.DNSRecordToFQDNViews(rec, groupMapping)
		if err := r.FQDNWriter.Replace(ctx, recordKey, rec.Spec.PortalRef, views); err != nil {
			return fmt.Errorf("refresh read store: %w", err)
		}
	}
	return nil
}
