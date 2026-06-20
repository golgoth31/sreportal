# Async DNS Resolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move DNSRecord SyncStatus resolution out of the reconcile hot path into a dedicated background Runnable that checks each FQDN once per ~24h (spread out), with immediate re-check forced on entry changes.

**Architecture:** A new `manager.Runnable` (`internal/controller/dnsresolve`) owns all DNS resolution. It keeps an in-memory per-FQDN schedule (jittered across 24h), resolves due FQDNs at a low steady rate, patches `DNSRecord.status.Endpoints[].SyncStatus`, and refreshes the FQDN read store (UI) via the existing `dnschain.DNSRecordToFQDNViews` + `FQDNWriter.Replace`. The DNSRecord reconcile drops `ResolveDNSHandler` (becomes ~ms) and instead enqueues the record into the Runnable's debounced force channel on every reconcile (which only fires on spec/generation change). Resolution is therefore fully decoupled from reconcile; no SyncStatus-driven reconcile churn.

**Tech Stack:** Go, controller-runtime (`manager.Runnable`), existing `domaindns.Resolver`/`CheckFQDN`, `domaindns.FQDNWriter`.

---

## Key design decision (D0) — how the UI gets async-resolved SyncStatus

The UI reads SyncStatus from the FQDN read store, populated during reconcile by `ProjectStoreHandler` (`fqdnWriter.Replace`, views built by `dnschain.DNSRecordToFQDNViews`). Async status patches do NOT trigger a reconcile (controller uses `GenerationChangedPredicate`). **Chosen approach (this plan): the Runnable updates BOTH the DNSRecord status AND the read store** (reusing `DNSRecordToFQDNViews` + `Replace`) — full decoupling, no reconcile churn.

Alternative (NOT taken): make a SyncStatus-aware predicate re-trigger a cheap reconcile that re-projects; requires `MaterialiseEntriesHandler` to preserve SyncStatus and adds reconcile churn. If the reviewer prefers this, revisit before Task 4.

## Constants (no configuration for now)

- Resolve interval per FQDN: `24h`.
- Scheduler tick: `1m`.
- Per-FQDN lookup timeout: `2s` (was 5s inline; off the hot path now).
- Force debounce window: `5s` (coalesce repeated enqueues of the same record).
- Max concurrent lookups: `10`.

## File Structure

- Create `internal/controller/dnsresolve/scheduler.go` — pure in-memory per-FQDN schedule (jitter, Due, Reschedule, Sync, ForceRecord). No k8s/DNS deps → fully unit-testable.
- Create `internal/controller/dnsresolve/scheduler_test.go`.
- Create `internal/controller/dnsresolve/runnable.go` — the `manager.Runnable`: ticker + force channel + debounce; resolves due/forced FQDNs, patches status, refreshes read store.
- Create `internal/controller/dnsresolve/runnable_test.go`.
- Modify `internal/controller/dnsrecords/dnsrecord_controller.go` — remove `ResolveDNSHandler` from the chain; add a `Forcer` hook called at end of `Reconcile`.
- Modify `internal/controller/dnsrecords/chain/resolve_dns.go` — delete (logic moves to the Runnable) OR keep `domaindns.CheckFQDN` usage in the Runnable. Delete the handler file + its test.
- Modify `cmd/main.go` — construct and `mgr.Add` the Runnable; wire the forcer into the DNSRecord reconciler.

---

## Task 1: Per-FQDN scheduler (pure, unit-tested)

**Files:**
- Create: `internal/controller/dnsresolve/scheduler.go`
- Test: `internal/controller/dnsresolve/scheduler_test.go`

- [ ] **Step 1: Write the failing test**

```go
package dnsresolve

import (
	"testing"
	"time"
)

func tk(record, fqdn, rt string) FQDNKey { return FQDNKey{RecordKey: record, DNSName: fqdn, RecordType: rt} }

func TestScheduler_SyncSpreadsAcrossInterval(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	s := newScheduler(24*time.Hour, func() time.Time { return base }, 1)
	keys := make([]FQDNKey, 200)
	for i := range keys {
		keys[i] = tk("ns/r", "h"+string(rune('a'+i%26)), "A")
	}
	// give them distinct names
	for i := range keys {
		keys[i].DNSName = keys[i].DNSName + "-" + time.Duration(i).String()
	}
	s.Sync(keys)
	// Nothing is due at base (all scheduled in (base, base+24h]).
	if due := s.Due(base); len(due) != 0 {
		t.Fatalf("expected 0 due at base, got %d", len(due))
	}
	// By base+24h everything is due.
	if due := s.Due(base.Add(24 * time.Hour)); len(due) != 200 {
		t.Fatalf("expected 200 due by +24h, got %d", len(due))
	}
}

func TestScheduler_RescheduleMovesNextOut(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	s := newScheduler(24*time.Hour, func() time.Time { return base }, 1)
	k := tk("ns/r", "a.example.com", "A")
	s.Sync([]FQDNKey{k})
	s.Reschedule(k)
	if due := s.Due(base.Add(24 * time.Hour)); len(due) != 0 {
		t.Fatalf("rescheduled key must not be due before base+interval, got %d", len(due))
	}
	if due := s.Due(base.Add(48 * time.Hour)); len(due) != 1 {
		t.Fatalf("rescheduled key must be due by base+2*interval, got %d", len(due))
	}
}

func TestScheduler_ForceRecordMakesAllRecordKeysDue(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	s := newScheduler(24*time.Hour, func() time.Time { return base }, 1)
	a := tk("ns/r", "a", "A")
	b := tk("ns/r", "b", "A")
	other := tk("ns/other", "c", "A")
	s.Sync([]FQDNKey{a, b, other})
	s.ForceRecord("ns/r")
	due := s.Due(base)
	if len(due) != 2 {
		t.Fatalf("force must make the record's 2 keys due, got %d", len(due))
	}
}

func TestScheduler_SyncDropsRemovedKeys(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	s := newScheduler(24*time.Hour, func() time.Time { return base }, 1)
	a := tk("ns/r", "a", "A")
	b := tk("ns/r", "b", "A")
	s.Sync([]FQDNKey{a, b})
	s.Sync([]FQDNKey{a}) // b removed
	s.ForceRecord("ns/r")
	if due := s.Due(base); len(due) != 1 {
		t.Fatalf("expected only surviving key due, got %d", len(due))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/dnsresolve/ -run TestScheduler -v`
Expected: FAIL (package/symbols undefined).

- [ ] **Step 3: Write the implementation**

```go
package dnsresolve

import (
	"math/rand"
	"sync"
	"time"
)

// FQDNKey identifies one endpoint of a DNSRecord. RecordKey is "namespace/name".
type FQDNKey struct {
	RecordKey  string
	DNSName    string
	RecordType string
}

// scheduler tracks, per FQDN, when it is next due for a DNS check. New keys are
// spread uniformly across [now, now+interval) so the steady-state check rate is
// ~len(keys)/interval with no thundering herd (including after a restart).
type scheduler struct {
	mu       sync.Mutex
	interval time.Duration
	now      func() time.Time
	rng      *rand.Rand
	next     map[FQDNKey]time.Time
}

func newScheduler(interval time.Duration, now func() time.Time, seed int64) *scheduler {
	return &scheduler{
		interval: interval,
		now:      now,
		rng:      rand.New(rand.NewSource(seed)),
		next:     map[FQDNKey]time.Time{},
	}
}

// Sync reconciles the tracked key set with the current desired set: new keys get
// a jittered nextCheck in (now, now+interval]; keys no longer present are dropped.
func (s *scheduler) Sync(keys []FQDNKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	desired := make(map[FQDNKey]struct{}, len(keys))
	now := s.now()
	for _, k := range keys {
		desired[k] = struct{}{}
		if _, ok := s.next[k]; !ok {
			jitter := time.Duration(s.rng.Int63n(int64(s.interval)))
			s.next[k] = now.Add(jitter)
		}
	}
	for k := range s.next {
		if _, ok := desired[k]; !ok {
			delete(s.next, k)
		}
	}
}

// Due returns the keys whose nextCheck is at or before t.
func (s *scheduler) Due(t time.Time) []FQDNKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []FQDNKey
	for k, n := range s.next {
		if !n.After(t) {
			out = append(out, k)
		}
	}
	return out
}

// Reschedule pushes a key's next check to now+interval.
func (s *scheduler) Reschedule(k FQDNKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.next[k]; ok {
		s.next[k] = s.now().Add(s.interval)
	}
}

// ForceRecord makes every tracked key of a record immediately due.
func (s *scheduler) ForceRecord(recordKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for k := range s.next {
		if k.RecordKey == recordKey {
			s.next[k] = now.Add(-time.Nanosecond)
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/controller/dnsresolve/ -run TestScheduler -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/controller/dnsresolve/scheduler.go internal/controller/dnsresolve/scheduler_test.go
git commit -m "feat(dnsresolve): per-FQDN resolution scheduler with 24h jittered spread"
```

---

## Task 2: Resolution + projection step (one record), unit-tested

**Files:**
- Create: `internal/controller/dnsresolve/runnable.go` (partial — the `resolveRecord` method + types)
- Test: `internal/controller/dnsresolve/runnable_test.go`

Design notes (from the existing code, do not change these contracts):
- `domaindns.CheckFQDN(ctx, resolver, dnsName, recordType, targets) *domaindns.CheckResult` → `.Status` is a `domaindns.SyncStatus`.
- `v1alpha2.EndpointStatus` fields: `DNSName`, `RecordType`, `Targets`, `SyncStatus v1alpha2.SyncStatus`, `LastSeen metav1.Time`.
- Read-store refresh: `dnschain.DNSRecordToFQDNViews(record, groupMapping)` → `[]domaindns.FQDNView`, then `fqdnWriter.Replace(ctx, recordKey, record.Spec.PortalRef, views)`.
- GroupMapping comes from the owning DNS CR (controller ownerRef of kind `DNS`); read it and use `&dns.Spec.GroupMapping`.

- [ ] **Step 1: Write the failing test** (resolves the record's endpoints, sets SyncStatus, patches status, replaces read-store views)

```go
package dnsresolve

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

// stubResolver returns a fixed host result.
type stubResolver struct{ addrs []string }

func (s stubResolver) LookupHost(_ context.Context, _ string) ([]string, error) { return s.addrs, nil }
func (s stubResolver) LookupCNAME(_ context.Context, _ string) (string, error)  { return "", nil }

// capWriter captures Replace calls.
type capWriter struct{ views []domaindns.FQDNView }

func (w *capWriter) Replace(_ context.Context, _ , _ string, fqdns []domaindns.FQDNView) error {
	w.views = fqdns
	return nil
}
func (w *capWriter) Delete(context.Context, string) error            { return nil }
func (w *capWriter) AnnotateOwner(string, string, string)            {}

func TestResolveRecord_SetsSyncStatusAndRefreshesStore(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha2.AddToScheme(scheme))

	rec := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "ns"},
		Spec:       v1alpha2.DNSRecordSpec{PortalRef: "p", SourceType: "ingress"},
		Status: v1alpha2.DNSRecordStatus{Endpoints: []v1alpha2.EndpointStatus{
			{DNSName: "a.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}, LastSeen: metav1.Now()},
		}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&v1alpha2.DNSRecord{}).WithObjects(rec).Build()
	w := &capWriter{}

	r := &Runnable{Client: c, Resolver: stubResolver{addrs: []string{"1.2.3.4"}}, FQDNWriter: w}
	require.NoError(t, r.resolveRecord(context.Background(), rec, nil, []FQDNKey{
		{RecordKey: "ns/r", DNSName: "a.example.com", RecordType: "A"},
	}))

	// status patched to sync
	var got v1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(rec), &got))
	require.Equal(t, v1alpha2.SyncStatus(domaindns.SyncStatusSync), got.Status.Endpoints[0].SyncStatus)
	// read store refreshed with the synced view
	require.Len(t, w.views, 1)
	require.Equal(t, string(domaindns.SyncStatusSync), w.views[0].SyncStatus)
}
```

(Add imports `"sigs.k8s.io/controller-runtime/pkg/client"` as needed.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/dnsresolve/ -run TestResolveRecord -v`
Expected: FAIL (`Runnable`/`resolveRecord` undefined).

- [ ] **Step 3: Implement `Runnable` skeleton + `resolveRecord`** in `runnable.go`

```go
package dnsresolve

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnschain "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

const lookupTimeout = 2 * time.Second

// Runnable resolves DNSRecord endpoints out-of-band and keeps both the
// DNSRecord status and the FQDN read store in sync.
type Runnable struct {
	Client     client.Client
	Resolver   domaindns.Resolver
	FQDNWriter domaindns.FQDNWriter
}

// resolveRecord resolves the given keys belonging to rec, writes SyncStatus onto
// rec.Status (matched by DNSName+RecordType), patches the status subresource, and
// refreshes the read store. groupMapping may be nil.
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
	base := rec.DeepCopy()
	for i := range rec.Status.Endpoints {
		ep := &rec.Status.Endpoints[i]
		k := FQDNKey{RecordKey: rec.Namespace + "/" + rec.Name, DNSName: ep.DNSName, RecordType: ep.RecordType}
		if _, ok := want[k]; !ok {
			continue
		}
		lc, cancel := context.WithTimeout(ctx, lookupTimeout)
		res := domaindns.CheckFQDN(lc, r.Resolver, ep.DNSName, ep.RecordType, ep.Targets)
		cancel()
		ep.SyncStatus = v1alpha2.SyncStatus(res.Status)
	}
	if err := r.Client.Status().Patch(ctx, rec, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch DNSRecord status: %w", err)
	}
	if r.FQDNWriter != nil {
		views := dnschain.DNSRecordToFQDNViews(rec, groupMapping)
		if err := r.FQDNWriter.Replace(ctx, rec.Namespace+"/"+rec.Name, rec.Spec.PortalRef, views); err != nil {
			return fmt.Errorf("refresh read store: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/controller/dnsresolve/ -run TestResolveRecord -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/dnsresolve/runnable.go internal/controller/dnsresolve/runnable_test.go
git commit -m "feat(dnsresolve): per-record resolve+status-patch+read-store refresh"
```

---

## Task 3: Runnable loop (ticker + debounced force) + schedule sync

**Files:**
- Modify: `internal/controller/dnsresolve/runnable.go`
- Test: `internal/controller/dnsresolve/runnable_test.go`

- [ ] **Step 1: Write the failing test** — `Force(recordKey)` enqueues; a manual `tick()` resolves forced keys and refreshes the store.

```go
func TestRunnable_ForceThenTickResolves(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha2.AddToScheme(scheme))
	rec := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "ns"},
		Spec:       v1alpha2.DNSRecordSpec{PortalRef: "p"},
		Status: v1alpha2.DNSRecordStatus{Endpoints: []v1alpha2.EndpointStatus{
			{DNSName: "a.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}, LastSeen: metav1.Now()},
		}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&v1alpha2.DNSRecord{}).WithObjects(rec).Build()
	w := &capWriter{}
	r := New(c, stubResolver{addrs: []string{"1.2.3.4"}}, w)

	r.Force("ns/r")        // simulate reconcile-driven force
	require.NoError(t, r.tick(context.Background())) // syncs schedule + resolves due/forced

	var got v1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(rec), &got))
	require.Equal(t, v1alpha2.SyncStatus(domaindns.SyncStatusSync), got.Status.Endpoints[0].SyncStatus)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/dnsresolve/ -run TestRunnable_Force -v`
Expected: FAIL (`New`/`Force`/`tick` undefined).

- [ ] **Step 3: Implement `New`, `Force`, `tick`, `listKeys`, `groupMappingFor`, and `Start`** (append to `runnable.go`)

```go
const (
	resolveInterval = 24 * time.Hour
	schedTick       = 1 * time.Minute
	forceDebounce   = 5 * time.Second
	maxConcurrent   = 10
)

// New builds a Runnable with the production scheduler (seeded from the clock).
func New(c client.Client, resolver domaindns.Resolver, w domaindns.FQDNWriter) *Runnable {
	r := &Runnable{Client: c, Resolver: resolver, FQDNWriter: w, forced: map[string]struct{}{}}
	r.sched = newScheduler(resolveInterval, time.Now, time.Now().UnixNano())
	return r
}

// Force requests an immediate re-resolution of recordKey ("namespace/name").
// Non-blocking; coalesced until the next tick (debounced).
func (r *Runnable) Force(recordKey string) {
	r.mu.Lock()
	r.forced[recordKey] = struct{}{}
	r.mu.Unlock()
}

// tick syncs the schedule with current DNSRecords, applies pending forces, and
// resolves all due records. Exposed for tests.
func (r *Runnable) tick(ctx context.Context) error {
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
			// preserve schedule on error -> retried next tick; log and continue.
			logf(ctx, rk, err)
			continue
		}
		for _, k := range keys {
			r.sched.Reschedule(k)
		}
	}
	return nil
}

// Start runs the loop until ctx is cancelled (manager.Runnable).
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
```

Add the supporting fields/helpers:

```go
// add to the Runnable struct:
//   sched  *scheduler
//   mu     sync.Mutex
//   forced map[string]struct{}

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

// groupMappingFor loads the owning DNS CR's group mapping (nil if not found).
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
```

Add a small `logf` helper using `sigs.k8s.io/controller-runtime/pkg/log` (`log.FromContext(ctx).Error(err, "resolve record failed", "record", rk)`), plus the `sync` and `log` imports. Required struct imports already present.

> Note for the implementer: confirm `v1alpha2.DNSSpec` exposes `GroupMapping GroupMappingSpec` (it is referenced by `LoadDNSConfigHandler`). If the field path differs, mirror exactly what `internal/controller/dnsrecords/chain/load_dns_config.go` reads.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/controller/dnsresolve/ -v`
Expected: PASS (all).

- [ ] **Step 5: Commit**

```bash
git add internal/controller/dnsresolve/
git commit -m "feat(dnsresolve): background runnable (24h tick + debounced force)"
```

---

## Task 4: Remove ResolveDNSHandler from the reconcile; add force hook

**Files:**
- Modify: `internal/controller/dnsrecords/dnsrecord_controller.go`
- Delete: `internal/controller/dnsrecords/chain/resolve_dns.go` and `resolve_dns_test.go`

- [ ] **Step 1: Add a `Forcer` interface + field on the reconciler**

In `dnsrecord_controller.go`, add:

```go
// Forcer requests an out-of-band DNS re-resolution for a record key.
type Forcer interface{ Force(recordKey string) }
```

Add field `forcer Forcer` to `DNSRecordReconciler` and a setter:

```go
func (r *DNSRecordReconciler) SetForcer(f Forcer) { r.forcer = f }
```

- [ ] **Step 2: Drop the handler from the chain**

In `rebuildChain()`, remove the `dnsrecordchain.NewResolveDNSHandler(...)` line so the chain is:

```go
r.chain = reconciler.NewChain(
	"dnsrecord",
	dnsrecordchain.NewLoadDNSConfigHandler(r.Client),
	dnsrecordchain.NewMaterialiseEntriesHandler(r.Client),
	dnsrecordchain.NewProjectStoreHandler(r.fqdnWriter),
)
```

Remove the now-unused `resolver` field/param from `DNSRecordReconciler`/`NewDNSRecordReconciler` (it moves to the Runnable). Update `cmd/main.go` call site in Task 5.

- [ ] **Step 3: Enqueue a force at the end of `Reconcile`**

In `Reconcile`, just before returning success (where `RequeueAfter` is set), add:

```go
if r.forcer != nil {
	r.forcer.Force(req.Namespace + "/" + req.Name)
}
```

Since the controller uses `GenerationChangedPredicate`, Reconcile only fires on spec/generation changes → this forces a fresh resolution exactly on entry changes.

- [ ] **Step 4: Delete the inline handler + test**

```bash
git rm internal/controller/dnsrecords/chain/resolve_dns.go internal/controller/dnsrecords/chain/resolve_dns_test.go
```

- [ ] **Step 5: Build + run package tests**

Run: `go build ./... && go test ./internal/controller/dnsrecords/...`
Expected: PASS (update any dnsrecord_controller_test.go references to the removed resolver — pass `nil`/drop the arg).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(dnsrecords): move DNS resolution off the reconcile path"
```

---

## Task 5: Wire the Runnable in cmd/main.go

**Files:**
- Modify: `cmd/main.go`

- [ ] **Step 1: Construct + wire**

Near the DNSRecord reconciler wiring (`cmd/main.go:579-585`), after `dnsRecordReconciler.SetFQDNWriter(fqdnStore)`:

```go
dnsResolver := dnsresolve.New(mgr.GetClient(), dnschain.NewNetResolver(), fqdnStore)
dnsRecordReconciler.SetForcer(dnsResolver)
if err := mgr.Add(dnsResolver); err != nil {
	setupLog.Error(err, "unable to add DNS resolve runnable")
	os.Exit(1)
}
```

Update `NewDNSRecordReconciler(...)` call to drop the resolver arg (per Task 4). Add the import `dnsresolve "github.com/golgoth31/sreportal/internal/controller/dnsresolve"`.

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: success (ignore the `web/dist` embed error if the UI isn't built).

- [ ] **Step 3: Commit**

```bash
git add cmd/main.go
git commit -m "feat(dnsresolve): wire background DNS resolver runnable in main"
```

---

## Task 6: Verify + generate + lint

- [ ] **Step 1:** `go test ./internal/controller/dnsresolve/... ./internal/controller/dnsrecords/...` → PASS
- [ ] **Step 2:** `make manifests && make helm` → check `git diff helm/values.yaml` is pure churn and `git checkout helm/values.yaml`; revert screenshot churn from `make doc` if run.
- [ ] **Step 3:** `make lint` → 0 issues (build `web/dist` stub first if needed).
- [ ] **Step 4:** Run the PR review panel (execution-tracer, security-reviewer, silent-failure-hunter, code-reviewer) on the diff, post synthesis to Slack DM `D037XBNQQKB`, then open the PR (base `main`).

---

## Self-Review notes

- **Spec coverage:** 24h spread (Task 1 jitter), force-on-change (Task 3 `Force` + Task 4 hook), no config (constants in Task 3), debounce (Task 3 `forceDebounce`), burst acceptable (per-tick batching + `maxConcurrent` — see follow-up below), reconcile no longer blocks on DNS (Task 4).
- **Open item (not blocking the plan, decide during Task 3):** `maxConcurrent` is declared but the resolve loop above is sequential per record. For a force on the 194-FQDN record, resolution runs sequentially (≈194×fast lookups; ~seconds, off the hot path). If that's too slow on force, parallelise `resolveRecord`'s per-endpoint loop with a bounded worker pool (mirror the old `resolveEndpoints` semaphore). Left sequential by default for simplicity (YAGNI) — revisit if force latency matters.
- **No reconcile churn:** the Runnable patches status + read store directly; `GenerationChangedPredicate` means these status patches never re-trigger reconcile.
- **Restart behaviour:** schedule is in-memory; on restart it re-seeds jittered across 24h, so SyncStatus is refreshed over the next 24h while the persisted CR status keeps the UI populated meanwhile. A spec change still forces an immediate check.
