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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

const (
	lookupTimeout   = 2 * time.Second
	maxConcurrent   = 10
	resolveInterval = 24 * time.Hour
	schedTick       = 1 * time.Minute
	forceDebounce   = 5 * time.Second
)

// Runnable resolves DNSRecord endpoints out-of-band (off the reconcile hot
// path) and writes the result onto each DNSRecord's status (SyncStatus). It is
// the ONLY component that resolves DNS; it does NOT touch the FQDN read store —
// projecting status to the read store stays the DNSRecord reconcile's job, so
// there is a single writer per record (no read-store contention).
type Runnable struct {
	Client   client.Client
	Resolver domaindns.Resolver

	sched   *scheduler
	mu      sync.Mutex
	forced  map[string]struct{}
	forceCh chan struct{}
}

// New creates a Runnable with an initialised scheduler.
func New(c client.Client, resolver domaindns.Resolver) *Runnable {
	return &Runnable{
		Client:   c,
		Resolver: resolver,
		sched:    newScheduler(resolveInterval, time.Now, time.Now().UnixNano()),
		forced:   map[string]struct{}{},
		forceCh:  make(chan struct{}, 1),
	}
}

// Force marks a record ("namespace/name") as immediately due for resolution and
// wakes the loop (debounced). Non-blocking. If the record's endpoints are not
// materialised yet, the request is retained and applied once they appear.
func (r *Runnable) Force(recordKey string) {
	r.mu.Lock()
	r.forced[recordKey] = struct{}{}
	r.mu.Unlock()
	select {
	case r.forceCh <- struct{}{}:
	default:
	}
}

// tick performs one resolution pass: syncs the scheduler with current records,
// applies pending forces (retaining those whose endpoints aren't materialised
// yet), then resolves all due FQDNs.
func (r *Runnable) tick(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("dnsresolve")
	var list v1alpha2.DNSRecordList
	if err := r.Client.List(ctx, &list); err != nil {
		return err
	}
	r.sched.Sync(listKeys(list.Items))

	r.mu.Lock()
	pending := r.forced
	r.forced = map[string]struct{}{}
	r.mu.Unlock()
	var retained []string
	for rk := range pending {
		if r.sched.ForceRecord(rk) == 0 {
			// No tracked keys yet (endpoints not materialised / cache lag):
			// keep the request so a later tick honours it once they appear.
			retained = append(retained, rk)
		}
	}
	if len(retained) > 0 {
		r.mu.Lock()
		for _, rk := range retained {
			r.forced[rk] = struct{}{}
		}
		r.mu.Unlock()
	}

	due := r.sched.Due(time.Now())
	if len(due) == 0 {
		return nil
	}
	byRecord := map[string][]FQDNKey{}
	for _, k := range due {
		byRecord[k.RecordKey] = append(byRecord[k.RecordKey], k)
	}
	for rk, keys := range byRecord {
		rec := recordFromList(list.Items, rk)
		if rec == nil {
			logger.V(1).Info("due key has no matching record; skipping", "record", rk)
			continue
		}
		if err := r.resolveRecord(ctx, rec, keys); err != nil {
			logger.Error(err, "resolve record failed", "record", rk)
			continue // schedule preserved -> retried next tick
		}
		for _, k := range keys {
			r.sched.Reschedule(k)
		}
	}
	return nil
}

// Start implements manager.Runnable: a steady tick (resolveInterval is spread
// per-FQDN by the scheduler) plus a debounced wake on Force.
func (r *Runnable) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("dnsresolve")
	ticker := time.NewTicker(schedTick)
	defer ticker.Stop()
	debounce := time.NewTimer(forceDebounce)
	if !debounce.Stop() {
		<-debounce.C
	}
	defer debounce.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.tick(ctx); err != nil {
				logger.Error(err, "dns resolve tick failed")
			}
		case <-r.forceCh:
			debounce.Reset(forceDebounce) // coalesce bursts of forces into one tick
		case <-debounce.C:
			if err := r.tick(ctx); err != nil {
				logger.Error(err, "dns resolve tick failed")
			}
		}
	}
}

func listKeys(records []v1alpha2.DNSRecord) []FQDNKey {
	var out []FQDNKey
	for i := range records {
		rk := records[i].Namespace + "/" + records[i].Name
		for _, ep := range records[i].Status.Endpoints {
			out = append(out, FQDNKey{RecordKey: rk, DNSName: ep.DNSName, RecordType: ep.RecordType})
		}
	}
	return out
}

func recordFromList(records []v1alpha2.DNSRecord, recordKey string) *v1alpha2.DNSRecord {
	for i := range records {
		if records[i].Namespace+"/"+records[i].Name == recordKey {
			return records[i].DeepCopy()
		}
	}
	return nil
}

var _ manager.Runnable = (*Runnable)(nil)

// resolveRecord resolves the requested keys of rec (in parallel, bounded),
// writes SyncStatus onto rec.Status.Endpoints (matched by DNSName+RecordType),
// and patches the status subresource. A real change in SyncStatus re-triggers
// the DNSRecord reconcile (via the SyncStatus predicate), which re-projects to
// the read store; an unchanged result yields a no-op patch (no reconcile).
func (r *Runnable) resolveRecord(ctx context.Context, rec *v1alpha2.DNSRecord, keys []FQDNKey) error {
	logger := log.FromContext(ctx).WithName("dnsresolve")
	want := make(map[FQDNKey]struct{}, len(keys))
	for _, k := range keys {
		want[k] = struct{}{}
	}
	recordKey := rec.Namespace + "/" + rec.Name

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

	// Resolve in parallel with a bounded worker pool. Each goroutine writes only
	// its own endpoint index, so concurrent writes to the slice are race-free.
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
				if res.Err != nil {
					// NotAvailable collapses timeout/NXDOMAIN/network; the underlying
					// error distinguishes a missing record from a DNS outage.
					logger.V(1).Info("DNS resolution failed",
						"fqdn", ep.DNSName, "recordType", ep.RecordType,
						"status", string(res.Status), "err", res.Err.Error())
				}
			}
		})
	}
	wg.Wait()

	if err := r.Client.Status().Patch(ctx, rec, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch DNSRecord status: %w", err)
	}
	return nil
}
