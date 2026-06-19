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
	dnschain "github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

const (
	lookupTimeout   = 2 * time.Second
	maxConcurrent   = 10
	resolveInterval = 24 * time.Hour
	schedTick       = 1 * time.Minute
	forceDebounce   = 5 * time.Second
)

// Runnable resolves DNSRecord endpoints out-of-band and keeps both the
// DNSRecord status and the FQDN read store in sync.
type Runnable struct {
	Client     client.Client
	Resolver   domaindns.Resolver
	FQDNWriter domaindns.FQDNWriter
	sched      *scheduler
	mu         sync.Mutex
	forced     map[string]struct{}
}

// New creates a Runnable with an initialised scheduler.
func New(c client.Client, resolver domaindns.Resolver, w domaindns.FQDNWriter) *Runnable {
	return &Runnable{
		Client:     c,
		Resolver:   resolver,
		FQDNWriter: w,
		sched:      newScheduler(resolveInterval, time.Now, time.Now().UnixNano()),
		forced:     map[string]struct{}{},
	}
}

// Force marks a record as immediately due for resolution on the next tick.
func (r *Runnable) Force(recordKey string) {
	r.mu.Lock()
	r.forced[recordKey] = struct{}{}
	r.mu.Unlock()
}

// tick performs one resolution pass: syncs the scheduler, flushes forced keys,
// then resolves all due FQDNs.
func (r *Runnable) tick(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("dnsresolve")
	var list v1alpha2.DNSRecordList
	if err := r.Client.List(ctx, &list); err != nil {
		return err
	}
	r.sched.Sync(listKeys(list.Items))

	r.mu.Lock()
	for rk := range r.forced {
		r.sched.ForceRecord(rk)
		delete(r.forced, rk)
	}
	r.mu.Unlock()

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
			continue
		}
		gm := r.groupMappingFor(ctx, rec)
		if err := r.resolveRecord(ctx, rec, gm, keys); err != nil {
			logger.Error(err, "resolve record failed", "record", rk)
			continue
		}
		for _, k := range keys {
			r.sched.Reschedule(k)
		}
	}
	return nil
}

// Start implements manager.Runnable. It ticks on a regular interval and also
// fires a debounced tick shortly after Force is called.
func (r *Runnable) Start(ctx context.Context) error {
	ticker := time.NewTicker(schedTick)
	defer ticker.Stop()
	debounce := time.NewTimer(forceDebounce)
	defer debounce.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_ = r.tick(ctx)
		case <-debounce.C:
			_ = r.tick(ctx)
			debounce.Reset(forceDebounce)
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

func (r *Runnable) groupMappingFor(ctx context.Context, rec *v1alpha2.DNSRecord) *v1alpha2.GroupMappingSpec {
	for _, o := range rec.GetOwnerReferences() {
		if o.Kind != "DNS" {
			continue
		}
		var dns v1alpha2.DNS
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: rec.Namespace, Name: o.Name}, &dns); err != nil {
			return nil
		}
		return &dns.Spec.GroupMapping
	}
	return nil
}

var _ manager.Runnable = (*Runnable)(nil)

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
