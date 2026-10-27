# DNS multi-CR per Portal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow N `DNS` CRs per `Portal`, owned-records cascade, portal-absent fast-out, and a deduplicated read store with a portal index.

**Architecture:** v1alpha2 spec gains `Defaults`; DNSRecord stays linked via a single controller `ownerReference` to its DNS; ReadStore is rewritten to deduplicate `(fqdn, recordType)` and index by portal; Source controller switches to a single global informer per kind with in-memory filtering; per-DNS source priority applied at DNSRecord generation; inter-DNS conflicts surfaced via `TargetsConflict` condition.

**Tech Stack:** Go 1.26, controller-runtime v0.23, Kubebuilder, Ginkgo+Gomega+envtest, Connect/Buf proto, React 19+Vite.

**Spec:** `docs/superpowers/specs/2026-05-15-dns-multi-cr-per-portal-design.md`

**Conventions reminder (CLAUDE.md):**
- After CRD changes: `make helm && make doc`.
- After proto changes: `make proto && go build ./... && (cd web && npx tsc -b)`.
- Conventional commits in English (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`).
- Never edit generated files (`zz_generated.*.go`, `config/crd/bases/*`, `internal/grpc/gen/*`, `web/src/gen/*`).
- TDD: red → green → commit. Frequent commits.

---

## Phase 1 — API types (v1alpha2)

### Task 1.1: Add `SourceFilterDefaults` and trim `DNSStatus`

**Files:**
- Modify: `api/v1alpha2/dns_types.go`

- [ ] **Step 1.1.1: Add `SourceFilterDefaults` type and `Defaults` field**

Edit `api/v1alpha2/dns_types.go`. Add new type after `SourcesSpec`:

```go
// SourceFilterDefaults defines fallback filter values applied to every source
// in this DNS CR. A source's own Namespace/LabelFilter, when non-empty,
// overrides the corresponding default.
type SourceFilterDefaults struct {
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +optional
	LabelFilter string `json:"labelFilter,omitempty"`
}
```

Add `Defaults` field to `DNSSpec` before `Sources`:

```go
// +optional
Defaults SourceFilterDefaults `json:"defaults,omitempty"`
```

- [ ] **Step 1.1.2: Remove `Groups` from `DNSStatus`**

In `DNSStatus`, delete the `Groups []FQDNGroupStatus` field. Keep `Conditions`, `LastReconcileTime`, `ObservedGeneration`, `ActiveSources`, `NextReconcileTime`.

- [ ] **Step 1.1.3: Add condition type constants**

Append to `api/v1alpha2/common_source_spec.go` (or create `api/v1alpha2/conditions.go` if you prefer):

```go
// DNS condition types.
const (
	ConditionReady            = "Ready"
	ConditionSourcesReady     = "SourcesReady"
	ConditionTargetsConflict  = "TargetsConflict"
)
```

- [ ] **Step 1.1.4: Regenerate deepcopy + CRDs**

Run:
```
make helm
```

Expected: `zz_generated.deepcopy.go` updated; `config/crd/bases/sreportal.io_dns.yaml` reflects new `spec.defaults` and trimmed status.

- [ ] **Step 1.1.5: Build**

Run:
```
go build ./...
```

Expected: passes. If references to `Status.Groups` remain (e.g. dns_controller GroupsToFQDNViews), they'll fail to compile — note them for Phase 9.

If compilation fails on `Status.Groups`, **comment out** the offending block in `internal/controller/dns/dns_controller.go` (the `GroupsToFQDNViews` function and its callers) with `// TODO(Phase 9): replaced by readstore`. We will revisit in Phase 9.

- [ ] **Step 1.1.6: Commit**

```
git add api/v1alpha2/ config/crd/bases/ internal/controller/dns/ docs/api/
git commit -m "feat(api): add DNS.spec.defaults, trim DNSStatus.Groups"
```

### Task 1.2: Update DNS conversion test fixtures (v1alpha1↔v1alpha2)

**Files:**
- Modify: `api/v1alpha2/dns_conversion_test.go` (if it references `Groups`)
- Modify: `api/v1alpha1/dns_conversion.go` (if it copies `Groups`)

- [ ] **Step 1.2.1: Adapt conversion code**

Grep for `Status.Groups`:
```
grep -rn "Status.Groups\|Status\.Groups" api/ internal/ | grep -v "_test.go"
```

For each hit in v1alpha1↔v1alpha2 conversion, drop the `Groups` copy (the read store is now the source of truth; v1alpha1 status may keep its `Groups` for backward serving).

- [ ] **Step 1.2.2: Run conversion tests**

Run:
```
go test ./api/...
```

Expected: PASS.

- [ ] **Step 1.2.3: Commit**

```
git add api/
git commit -m "refactor(api): drop Groups copy in DNS conversion (readstore is source of truth)"
```

---

## Phase 2 — DNS webhook

### Task 2.1: Drop `name == portalRef` check

**Files:**
- Modify: `internal/webhook/v1alpha2/dns_webhook.go`
- Modify: `internal/webhook/v1alpha2/dns_webhook_test.go`

- [ ] **Step 2.1.1: Update test expectations**

Open `internal/webhook/v1alpha2/dns_webhook_test.go`. Identify tests that assert `name must equal portalRef` rejection on create/update — invert them: the same payloads must now be accepted.

Add a new test case: create a DNS with `metadata.name="team-a-dns"` and `spec.portalRef="my-portal"` → expect NO error.

- [ ] **Step 2.1.2: Run tests (should fail)**

Run:
```
go test ./internal/webhook/v1alpha2/...
```

Expected: FAIL — old assertions still hit the constraint.

- [ ] **Step 2.1.3: Remove the constraint in webhook**

In `dns_webhook.go`, delete both `if obj.Name != obj.Spec.PortalRef` checks (in `ValidateCreate` and `ValidateUpdate`). Keep `portalRef` immutability check on update.

- [ ] **Step 2.1.4: Run tests (should pass)**

Run:
```
go test ./internal/webhook/v1alpha2/...
```

Expected: PASS.

- [ ] **Step 2.1.5: Commit**

```
git add internal/webhook/v1alpha2/dns_webhook.go internal/webhook/v1alpha2/dns_webhook_test.go
git commit -m "feat(webhook): allow N DNS CRs per portal (drop name==portalRef)"
```

### Task 2.2: Validate `labelFilter` and `priority` references

**Files:**
- Modify: `internal/webhook/v1alpha2/dns_webhook.go`
- Modify: `internal/webhook/v1alpha2/dns_webhook_test.go`

- [ ] **Step 2.2.1: Add failing tests for invalid `labelFilter`**

In `dns_webhook_test.go`, add:
- DNS with `spec.defaults.labelFilter = "not a valid selector!!"` → must reject.
- DNS with `spec.sources.service.labelFilter = "app=foo,!="` → must reject.
- DNS with `spec.sources.priority = ["service", "unknown-source"]` → must reject.
- DNS with `spec.sources.priority = ["ingress"]` but `ingress.enabled = false` → must reject.

- [ ] **Step 2.2.2: Run tests (should fail)**

```
go test ./internal/webhook/v1alpha2/...
```

Expected: FAIL.

- [ ] **Step 2.2.3: Implement validation**

In `dns_webhook.go`, factor a `validate(dns)` helper called from both `ValidateCreate` and `ValidateUpdate`:

```go
import (
	"k8s.io/apimachinery/pkg/labels"
)

func (v *DNSCustomValidator) validate(obj *sreportalv1alpha2.DNS) error {
	if obj.Spec.Defaults.LabelFilter != "" {
		if _, err := labels.Parse(obj.Spec.Defaults.LabelFilter); err != nil {
			return fmt.Errorf("spec.defaults.labelFilter: %w", err)
		}
	}
	// Check each source's labelFilter via the CommonSourceSpec helper.
	for kind, lf := range collectLabelFilters(&obj.Spec.Sources) {
		if lf == "" {
			continue
		}
		if _, err := labels.Parse(lf); err != nil {
			return fmt.Errorf("spec.sources.%s.labelFilter: %w", kind, err)
		}
	}
	// Validate priority: every entry must be enabled in this spec.
	enabled := enabledSourceTypes(&obj.Spec.Sources)
	for _, p := range obj.Spec.Sources.Priority {
		if _, ok := enabled[p]; !ok {
			return fmt.Errorf("spec.sources.priority entry %q is not an enabled source in this DNS", p)
		}
	}
	return nil
}
```

Implement `collectLabelFilters` and `enabledSourceTypes` in the same file. `collectLabelFilters` walks every non-nil `Sources.<Kind>` pointer and returns `map[string]string` (e.g. `{"service": "app=foo"}`). `enabledSourceTypes` returns `map[SourceType]struct{}` for sources with `enabled=true`.

- [ ] **Step 2.2.4: Run tests (should pass)**

```
go test ./internal/webhook/v1alpha2/...
```

Expected: PASS.

- [ ] **Step 2.2.5: Commit**

```
git add internal/webhook/v1alpha2/
git commit -m "feat(webhook): validate labelFilter parses and priority refs enabled sources"
```

---

## Phase 3 — DNSRecord webhook (ownerRef + portalRef coherence)

### Task 3.1: Require single controller ownerReference to a DNS

**Files:**
- Modify: `internal/webhook/v1alpha2/dnsrecord_webhook.go`
- Modify: `internal/webhook/v1alpha2/dnsrecord_webhook_test.go`

- [ ] **Step 3.1.1: Add failing tests**

In `dnsrecord_webhook_test.go`, add scenarios (Ginkgo `It` blocks):

```go
It("rejects DNSRecord with no ownerReferences", func() {
	r := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "rec", Namespace: "default"},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin: sreportalv1alpha2.DNSRecordOriginManual,
			PortalRef: "my-portal",
			Entries: []sreportalv1alpha2.DNSRecordEntry{{FQDN: "foo.example.com.", RecordType: "A"}},
		},
	}
	_, err := v.ValidateCreate(ctx, r)
	Expect(err).To(MatchError(ContainSubstring("ownerReferences")))
})

It("rejects DNSRecord with two controller ownerRefs", func() { /* ... */ })
It("rejects DNSRecord ownerRef pointing to wrong Kind", func() { /* ... */ })
It("rejects DNSRecord whose portalRef differs from owner DNS portalRef", func() { /* ... */ })
It("rejects update that mutates ownerReference controller", func() { /* ... */ })
It("accepts DNSRecord with valid DNS ownerReference and matching portalRef", func() { /* ... */ })
```

Provide envtest-backed DNS objects so the validator can look up owner portalRef.

- [ ] **Step 3.1.2: Inject client.Client into validator**

In `dnsrecord_webhook.go`, change validator struct:

```go
type DNSRecordCustomValidator struct {
	client       client.Client
	controllerSA string
}

func NewDNSRecordCustomValidator(c client.Client, controllerSA string) *DNSRecordCustomValidator {
	return &DNSRecordCustomValidator{client: c, controllerSA: controllerSA}
}

func SetupDNSRecordWebhookWithManager(mgr ctrl.Manager, controllerSA string) error {
	return ctrl.NewWebhookManagedBy(mgr, &sreportalv1alpha2.DNSRecord{}).
		WithValidator(&DNSRecordCustomValidator{client: mgr.GetClient(), controllerSA: controllerSA}).
		Complete()
}
```

Update `cmd/main.go` callsite of `SetupDNSRecordWebhookWithManager` if signature changed (it didn't — only the constructor changed).

- [ ] **Step 3.1.3: Implement ownerRef validation**

In `validate()` (existing helper), append:

```go
// Exactly one controller ownerReference of Kind=DNS.
ctrlRefs := 0
var ownerName string
for _, or := range r.OwnerReferences {
	if or.Controller != nil && *or.Controller {
		ctrlRefs++
		if or.Kind != "DNS" || or.APIVersion != sreportalv1alpha2.GroupVersion.String() {
			return fmt.Errorf("controller ownerReference must be a DNS (got %s/%s)", or.APIVersion, or.Kind)
		}
		ownerName = or.Name
	}
}
if ctrlRefs != 1 {
	return fmt.Errorf("DNSRecord requires exactly one controller ownerReference to a DNS (got %d)", ctrlRefs)
}

// Fetch owner DNS and compare portalRef.
var owner sreportalv1alpha2.DNS
if err := v.client.Get(ctx, types.NamespacedName{Namespace: r.Namespace, Name: ownerName}, &owner); err != nil {
	return fmt.Errorf("owner DNS %q not found in namespace %q: %w", ownerName, r.Namespace, err)
}
if owner.Spec.PortalRef != r.Spec.PortalRef {
	return fmt.Errorf("spec.portalRef=%q does not match owner DNS.spec.portalRef=%q", r.Spec.PortalRef, owner.Spec.PortalRef)
}

// On update: ownerRef immutable.
if old != nil {
	if oldRef, newRef := controllerRef(old.OwnerReferences), controllerRef(r.OwnerReferences); oldRef != newRef {
		return fmt.Errorf("controller ownerReference is immutable")
	}
}
```

Add helper:

```go
func controllerRef(refs []metav1.OwnerReference) string {
	for _, or := range refs {
		if or.Controller != nil && *or.Controller {
			return string(or.UID)
		}
	}
	return ""
}
```

- [ ] **Step 3.1.4: Run tests**

```
go test ./internal/webhook/v1alpha2/...
```

Expected: PASS.

- [ ] **Step 3.1.5: Commit**

```
git add internal/webhook/v1alpha2/
git commit -m "feat(webhook): require single DNS controller ownerRef + portalRef coherence on DNSRecord"
```

### Task 3.2: Update DNSRecord controller-runtime creation paths to set ownerRef

**DEFERRED — folded into Task 6.5.**

Status check at the time Task 3.1 landed: no production code path creates a v1alpha2 DNSRecord. The only DNSRecord creator is the v1alpha1 source controller in `internal/controller/source/chain/reconcile_dnsrecords.go`, which creates v1alpha1 DNSRecords and goes through the v1alpha1 webhook (no ownerRef enforcement). Retrofitting that path to set v1alpha2 DNS ownerRefs would require it to look up or create the owning DNS CR — work that overlaps with Task 6.5 (Source — new controller using global informer + per-DNS production), where the source controller is rebuilt against v1alpha2 anyway.

Decision: keep this task as a no-op marker and fold the ownerRef-setting requirement into Task 6.5. Task 6.5 must call `controllerutil.SetControllerReference(&ownerDNS, &record, scheme)` before every v1alpha2 DNSRecord `Create()` it issues, and any test fixtures it introduces must include valid controller ownerRefs.

---

## Phase 4 — ReadStore refactor (dedup + portal index + refcount)

The biggest, riskiest piece. TDD-first. Each step adds tests covering one operation.

### Task 4.1: Define new types

**Files:**
- Modify: `internal/domain/dns/read_model.go`

- [ ] **Step 4.1.1: Replace `PortalName string` with `Portals []string`**

```go
type FQDNView struct {
	Name        string
	Source      Source
	SourceType  string
	Groups      []string
	Description string
	RecordType  string
	Targets     []string
	LastSeen    time.Time
	Portals     []string // multiple portals possible after dedup
	Namespace   string
	OriginRef   *ResourceRef
	SyncStatus  string
}
```

- [ ] **Step 4.1.2: Fix immediate compile errors**

Run `go build ./...`. For every `PortalName` reference, replace with `Portals[0]` *temporarily* if reading a single value, or with the new list semantics if writing. Most sites are the `ProjectStoreHandler.DNSRecordToFQDNViews` and the grpc adapter — keep changes minimal here, just enough to compile.

- [ ] **Step 4.1.3: Commit**

```
git add internal/domain/dns/ internal/
git commit -m "refactor(domain/dns): FQDNView.PortalName -> Portals []string"
```

### Task 4.2: New `FQDNWriter.Replace` signature

**Files:**
- Modify: `internal/domain/dns/writer.go`

- [ ] **Step 4.2.1: Update interface**

```go
type FQDNWriter interface {
	// Replace atomically replaces all FQDNs contributed by a single DNSRecord,
	// recording the portalRef so the store can maintain its portal index.
	Replace(ctx context.Context, recordKey, portalRef string, fqdns []FQDNView) error
	Delete(ctx context.Context, recordKey string) error
}
```

- [ ] **Step 4.2.2: Update existing implementation to compile (skeleton only)**

In `internal/readstore/dns/fqdn_store.go`, update the `Replace` method signature to match. Existing logic still uses the old generic store underneath — we'll rewrite the body in Task 4.3+. For now, just adapt the signature; tests will guide the rewrite.

- [ ] **Step 4.2.3: Update callsite in `project_store.go`**

In `internal/controller/dnsrecords/chain/project_store.go`:

```go
if err := h.fqdnWriter.Replace(ctx, rc.Data.ResourceKey, rc.Resource.Spec.PortalRef, views); err != nil {
	return fmt.Errorf("project store: %w", err)
}
```

Also change `DNSRecordToFQDNViews` to set `Portals: []string{record.Spec.PortalRef}` instead of `PortalName: record.Spec.PortalRef`.

- [ ] **Step 4.2.4: Build**

```
go build ./...
```

Expected: passes.

- [ ] **Step 4.2.5: Commit**

```
git add internal/
git commit -m "refactor(readstore): FQDNWriter.Replace accepts portalRef"
```

### Task 4.3: Rewrite FQDNStore — TDD shell

**Files:**
- Modify: `internal/readstore/dns/fqdn_store.go`
- Modify: `internal/readstore/dns/fqdn_store_test.go`

- [ ] **Step 4.3.1: Sketch the new store layout (no logic)**

Replace `internal/readstore/dns/fqdn_store.go` with the skeleton:

```go
package dns

import (
	"context"
	"sync"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

type FQDNKey struct {
	Name       string
	RecordType string
}

type recordContribution struct {
	keys      map[FQDNKey]struct{}
	portalRef string
}

type FQDNStore struct {
	mu             sync.RWMutex
	fqdns          map[FQDNKey]*domaindns.FQDNView
	byPortal       map[string]map[FQDNKey]struct{}
	byRecord       map[string]recordContribution
	perPortalCount map[FQDNKey]map[string]int
	conflicts      *conflictRing

	notifyMu sync.Mutex
	notifyCh chan struct{}
}

// NewFQDNStore returns an empty FQDNStore. Source priority is enforced
// upstream (per-DNS-CR at DNSRecord generation time); the store treats
// each Replace call as authoritative for its (recordKey, portalRef) tuple.
func NewFQDNStore() *FQDNStore {
	return &FQDNStore{
		fqdns:          map[FQDNKey]*domaindns.FQDNView{},
		byPortal:       map[string]map[FQDNKey]struct{}{},
		byRecord:       map[string]recordContribution{},
		perPortalCount: map[FQDNKey]map[string]int{},
		conflicts:      newConflictRing(256),
		notifyCh:       make(chan struct{}),
	}
}

var (
	_ domaindns.FQDNReader = (*FQDNStore)(nil)
	_ domaindns.FQDNWriter = (*FQDNStore)(nil)
)

func (s *FQDNStore) Replace(ctx context.Context, recordKey, portalRef string, fqdns []domaindns.FQDNView) error {
	return nil // implemented in Task 4.4
}
func (s *FQDNStore) Delete(ctx context.Context, recordKey string) error {
	return nil // implemented in Task 4.5
}
func (s *FQDNStore) List(ctx context.Context, f domaindns.FQDNFilters) ([]domaindns.FQDNView, error) {
	return nil, nil // implemented in Task 4.6
}
func (s *FQDNStore) Get(ctx context.Context, name, recordType string) (domaindns.FQDNView, error) {
	return domaindns.FQDNView{}, nil // implemented in Task 4.6
}
func (s *FQDNStore) Count(ctx context.Context, f domaindns.FQDNFilters) (int, error) {
	return 0, nil // implemented in Task 4.6
}
func (s *FQDNStore) Subscribe() <-chan struct{} {
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()
	return s.notifyCh
}

func (s *FQDNStore) broadcast() {
	s.notifyMu.Lock()
	old := s.notifyCh
	s.notifyCh = make(chan struct{})
	s.notifyMu.Unlock()
	close(old)
}
```

- [ ] **Step 4.3.2: Create `conflict_ring.go`**

Create `internal/readstore/dns/conflict_ring.go`:

```go
package dns

import (
	"sync"
	"time"
)

// ConflictEvent records that two DNSRecords produced different targets for the
// same (fqdn, recordType) key. The first writer kept its data; the second
// (losing) DNSRecord is the one to surface a TargetsConflict condition on.
type ConflictEvent struct {
	FQDNKey      FQDNKey
	WinnerRecord string // resourceKey of the existing winner
	LoserRecord  string // resourceKey of the rejected writer
	WinnerDNS    string // owner DNS namespace/name (best-effort)
	LoserDNS     string
	PortalRef    string
	At           time.Time
}

type conflictRing struct {
	mu    sync.Mutex
	buf   []ConflictEvent
	idx   int
	full  bool
	cap   int
}

func newConflictRing(capacity int) *conflictRing {
	return &conflictRing{buf: make([]ConflictEvent, capacity), cap: capacity}
}

// Push records a new conflict, overwriting the oldest entry when full.
func (r *conflictRing) Push(e ConflictEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.idx] = e
	r.idx = (r.idx + 1) % r.cap
	if r.idx == 0 {
		r.full = true
	}
}

// Snapshot returns a copy of all events currently held.
func (r *conflictRing) Snapshot() []ConflictEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		out := make([]ConflictEvent, r.idx)
		copy(out, r.buf[:r.idx])
		return out
	}
	out := make([]ConflictEvent, r.cap)
	copy(out, r.buf[r.idx:])
	copy(out[r.cap-r.idx:], r.buf[:r.idx])
	return out
}
```

- [ ] **Step 4.3.3: Build**

```
go build ./internal/readstore/...
```

Expected: passes.

- [ ] **Step 4.3.4: Commit**

```
git add internal/readstore/dns/
git commit -m "refactor(readstore): skeleton for deduplicated FQDNStore with portal index"
```

### Task 4.4: TDD — `Replace` happy path + inter-record dedup

**Files:**
- Modify: `internal/readstore/dns/fqdn_store_test.go`
- Modify: `internal/readstore/dns/fqdn_store.go`

- [ ] **Step 4.4.1: Write failing tests**

Replace the body of `fqdn_store_test.go` (keep existing test that still apply, but most need rewriting). Add cases:

```go
var _ = Describe("FQDNStore.Replace", func() {
	var s *FQDNStore
	BeforeEach(func() { s = NewFQDNStore() })

	It("inserts a single FQDN from one DNSRecord", func() {
		err := s.Replace(ctx, "ns/rec-a", "portal-x", []domaindns.FQDNView{
			{Name: "foo.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}, Portals: []string{"portal-x"}},
		})
		Expect(err).NotTo(HaveOccurred())

		out, _ := s.List(ctx, domaindns.FQDNFilters{Portal: "portal-x"})
		Expect(out).To(HaveLen(1))
		Expect(out[0].Name).To(Equal("foo.example.com"))
		Expect(out[0].Portals).To(ConsistOf("portal-x"))
	})

	It("deduplicates same FQDN contributed by two DNSRecords (same portal)", func() {
		view := domaindns.FQDNView{Name: "shared.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}}
		Expect(s.Replace(ctx, "ns/rec-a", "portal-x", []domaindns.FQDNView{view})).To(Succeed())
		Expect(s.Replace(ctx, "ns/rec-b", "portal-x", []domaindns.FQDNView{view})).To(Succeed())

		out, _ := s.List(ctx, domaindns.FQDNFilters{Portal: "portal-x"})
		Expect(out).To(HaveLen(1)) // one canonical entry
		Expect(out[0].Portals).To(ConsistOf("portal-x"))
	})

	It("dedupes across two portals (multi-portal Portals list)", func() {
		view := domaindns.FQDNView{Name: "shared.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}}
		Expect(s.Replace(ctx, "ns/rec-a", "portal-x", []domaindns.FQDNView{view})).To(Succeed())
		Expect(s.Replace(ctx, "ns/rec-b", "portal-y", []domaindns.FQDNView{view})).To(Succeed())

		out, _ := s.List(ctx, domaindns.FQDNFilters{})
		Expect(out).To(HaveLen(1))
		Expect(out[0].Portals).To(ConsistOf("portal-x", "portal-y"))
	})

	It("merges Groups across contributors", func() {
		Expect(s.Replace(ctx, "ns/rec-a", "p", []domaindns.FQDNView{
			{Name: "g.example.com", RecordType: "A", Groups: []string{"team-a"}},
		})).To(Succeed())
		Expect(s.Replace(ctx, "ns/rec-b", "p", []domaindns.FQDNView{
			{Name: "g.example.com", RecordType: "A", Groups: []string{"team-b"}},
		})).To(Succeed())
		got, _ := s.Get(ctx, "g.example.com", "A")
		Expect(got.Groups).To(ConsistOf("team-a", "team-b"))
	})

	It("keeps first writer's Targets on conflict and records a conflict event", func() {
		Expect(s.Replace(ctx, "ns/rec-a", "p", []domaindns.FQDNView{
			{Name: "c.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}},
		})).To(Succeed())
		Expect(s.Replace(ctx, "ns/rec-b", "p", []domaindns.FQDNView{
			{Name: "c.example.com", RecordType: "A", Targets: []string{"2.2.2.2"}},
		})).To(Succeed())
		got, _ := s.Get(ctx, "c.example.com", "A")
		Expect(got.Targets).To(Equal([]string{"1.1.1.1"}))

		events := s.Conflicts() // exposed test helper, see step 4.4.3
		Expect(events).To(HaveLen(1))
		Expect(events[0].LoserRecord).To(Equal("ns/rec-b"))
	})
})
```

- [ ] **Step 4.4.2: Run (should fail)**

```
go test ./internal/readstore/dns/...
```

Expected: FAIL — store stubs return nil.

- [ ] **Step 4.4.3: Implement `Replace` + a test-only `Conflicts` accessor**

```go
func (s *FQDNStore) Replace(ctx context.Context, recordKey, portalRef string, fqdns []domaindns.FQDNView) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newKeys := make(map[FQDNKey]struct{}, len(fqdns))
	for _, v := range fqdns {
		newKeys[FQDNKey{Name: v.Name, RecordType: v.RecordType}] = struct{}{}
	}

	prev := s.byRecord[recordKey]

	// Remove keys this record no longer contributes.
	for k := range prev.keys {
		if _, kept := newKeys[k]; kept {
			continue
		}
		s.decrementContribution(k, prev.portalRef)
	}

	for _, v := range fqdns {
		k := FQDNKey{Name: v.Name, RecordType: v.RecordType}
		existing, ok := s.fqdns[k]
		if !ok {
			cp := v
			cp.Portals = []string{portalRef}
			s.fqdns[k] = &cp
		} else {
			// Conflict detection on Targets/RecordType.
			if !sameTargets(existing.Targets, v.Targets) {
				s.conflicts.Push(ConflictEvent{
					FQDNKey: k, LoserRecord: recordKey, PortalRef: portalRef, At: time.Now(),
				})
				// first-writer wins for Targets/SyncStatus/OriginRef/Description
			}
			existing.Groups = mergeStrings(existing.Groups, v.Groups)
			if !contains(existing.Portals, portalRef) {
				existing.Portals = append(existing.Portals, portalRef)
				sort.Strings(existing.Portals)
			}
		}
		// Per-portal contributor counter.
		if s.perPortalCount[k] == nil {
			s.perPortalCount[k] = map[string]int{}
		}
		// Only count NEW contributions from this record (i.e. keys it didn't have before).
		if _, hadBefore := prev.keys[k]; !hadBefore {
			s.perPortalCount[k][portalRef]++
		}

		// Portal index.
		if s.byPortal[portalRef] == nil {
			s.byPortal[portalRef] = map[FQDNKey]struct{}{}
		}
		s.byPortal[portalRef][k] = struct{}{}
	}

	s.byRecord[recordKey] = recordContribution{keys: newKeys, portalRef: portalRef}

	go s.broadcast() // outside mutex to avoid lock inversion with subscribers
	return nil
}

// Conflicts is a test/admin accessor returning a snapshot of recent conflicts.
func (s *FQDNStore) Conflicts() []ConflictEvent { return s.conflicts.Snapshot() }

func (s *FQDNStore) decrementContribution(k FQDNKey, portalRef string) {
	counts := s.perPortalCount[k]
	if counts == nil {
		return
	}
	counts[portalRef]--
	if counts[portalRef] <= 0 {
		delete(counts, portalRef)
		if set := s.byPortal[portalRef]; set != nil {
			delete(set, k)
			if len(set) == 0 {
				delete(s.byPortal, portalRef)
			}
		}
		if v := s.fqdns[k]; v != nil {
			v.Portals = removeString(v.Portals, portalRef)
		}
	}
	if len(counts) == 0 {
		delete(s.perPortalCount, k)
		delete(s.fqdns, k)
	}
}

func sameTargets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func mergeStrings(a, b []string) []string {
	out := append([]string(nil), a...)
	for _, x := range b {
		if !contains(out, x) {
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
func contains(haystack []string, needle string) bool {
	for _, x := range haystack {
		if x == needle {
			return true
		}
	}
	return false
}
func removeString(a []string, s string) []string {
	out := a[:0]
	for _, x := range a {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}
```

Note: `decrementContribution` deletes the FQDN from the canonical map when `perPortalCount[k]` is empty across all portals. The `byPortal` cleanup is inside the per-portal branch.

Note on broadcast: calling `broadcast()` while holding `s.mu` is also fine (close+create channel doesn't touch `s.mu`), but `go s.broadcast()` keeps subscribers off the write path. Either works.

- [ ] **Step 4.4.4: Run (should pass)**

```
go test ./internal/readstore/dns/...
```

Expected: PASS.

- [ ] **Step 4.4.5: Commit**

```
git add internal/readstore/dns/
git commit -m "feat(readstore): Replace with dedup, portal index, conflict ring"
```

### Task 4.5: TDD — `Delete` and shrink semantics

**Files:**
- Modify: `internal/readstore/dns/fqdn_store_test.go`
- Modify: `internal/readstore/dns/fqdn_store.go`

- [ ] **Step 4.5.1: Write failing tests**

Add to `fqdn_store_test.go`:

```go
var _ = Describe("FQDNStore.Delete", func() {
	var s *FQDNStore
	BeforeEach(func() { s = NewFQDNStore() })

	It("removes FQDN when last contributor of the only portal is deleted", func() {
		view := domaindns.FQDNView{Name: "x.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}}
		Expect(s.Replace(ctx, "ns/a", "p", []domaindns.FQDNView{view})).To(Succeed())
		Expect(s.Delete(ctx, "ns/a")).To(Succeed())
		out, _ := s.List(ctx, domaindns.FQDNFilters{})
		Expect(out).To(BeEmpty())
	})

	It("keeps FQDN if another DNSRecord still contributes", func() {
		view := domaindns.FQDNView{Name: "x.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}}
		Expect(s.Replace(ctx, "ns/a", "p", []domaindns.FQDNView{view})).To(Succeed())
		Expect(s.Replace(ctx, "ns/b", "p", []domaindns.FQDNView{view})).To(Succeed())
		Expect(s.Delete(ctx, "ns/a")).To(Succeed())
		out, _ := s.List(ctx, domaindns.FQDNFilters{Portal: "p"})
		Expect(out).To(HaveLen(1))
	})

	It("removes portal from Portals when its last contributor drops", func() {
		view := domaindns.FQDNView{Name: "x.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}}
		Expect(s.Replace(ctx, "ns/a", "p1", []domaindns.FQDNView{view})).To(Succeed())
		Expect(s.Replace(ctx, "ns/b", "p2", []domaindns.FQDNView{view})).To(Succeed())
		Expect(s.Delete(ctx, "ns/a")).To(Succeed())
		got, _ := s.Get(ctx, "x.example.com", "A")
		Expect(got.Portals).To(ConsistOf("p2"))
	})

	It("shrinking Replace removes orphaned FQDN keys", func() {
		Expect(s.Replace(ctx, "ns/a", "p", []domaindns.FQDNView{
			{Name: "a.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}},
			{Name: "b.example.com", RecordType: "A", Targets: []string{"2.2.2.2"}},
		})).To(Succeed())
		Expect(s.Replace(ctx, "ns/a", "p", []domaindns.FQDNView{
			{Name: "a.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}},
		})).To(Succeed())
		out, _ := s.List(ctx, domaindns.FQDNFilters{Portal: "p"})
		Expect(out).To(HaveLen(1))
		Expect(out[0].Name).To(Equal("a.example.com"))
	})
})
```

- [ ] **Step 4.5.2: Run (should fail)**

```
go test ./internal/readstore/dns/...
```

Expected: FAIL.

- [ ] **Step 4.5.3: Implement `Delete`**

```go
func (s *FQDNStore) Delete(ctx context.Context, recordKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	contrib, ok := s.byRecord[recordKey]
	if !ok {
		return nil
	}
	for k := range contrib.keys {
		s.decrementContribution(k, contrib.portalRef)
	}
	delete(s.byRecord, recordKey)

	go s.broadcast()
	return nil
}
```

- [ ] **Step 4.5.4: Run (should pass)**

```
go test ./internal/readstore/dns/...
```

Expected: PASS.

- [ ] **Step 4.5.5: Commit**

```
git add internal/readstore/dns/
git commit -m "feat(readstore): Delete + shrink semantics with refcount cleanup"
```

### Task 4.6: TDD — `List` / `Get` / `Count`

**Files:**
- Modify: `internal/readstore/dns/fqdn_store_test.go`
- Modify: `internal/readstore/dns/fqdn_store.go`

- [ ] **Step 4.6.1: Write failing tests**

```go
var _ = Describe("FQDNStore.List/Get/Count", func() {
	var s *FQDNStore
	BeforeEach(func() {
		s = NewFQDNStore()
		_ = s.Replace(ctx, "ns/a", "p1", []domaindns.FQDNView{
			{Name: "alpha.example.com", RecordType: "A", Namespace: "ns", Source: domaindns.SourceExternalDNS},
			{Name: "beta.example.com", RecordType: "A", Namespace: "ns", Source: domaindns.SourceExternalDNS},
		})
		_ = s.Replace(ctx, "ns/b", "p2", []domaindns.FQDNView{
			{Name: "gamma.example.com", RecordType: "A", Namespace: "ns", Source: domaindns.SourceManual},
		})
	})

	It("filters by portal via byPortal index", func() {
		out, _ := s.List(ctx, domaindns.FQDNFilters{Portal: "p1"})
		names := []string{}
		for _, v := range out { names = append(names, v.Name) }
		Expect(names).To(ConsistOf("alpha.example.com", "beta.example.com"))
	})

	It("returns sorted by (Name, RecordType)", func() {
		out, _ := s.List(ctx, domaindns.FQDNFilters{})
		Expect(out[0].Name).To(Equal("alpha.example.com"))
		Expect(out[2].Name).To(Equal("gamma.example.com"))
	})

	It("Count matches List length under same filter", func() {
		out, _ := s.List(ctx, domaindns.FQDNFilters{Portal: "p1"})
		n, _ := s.Count(ctx, domaindns.FQDNFilters{Portal: "p1"})
		Expect(n).To(Equal(len(out)))
	})

	It("Get returns ErrFQDNNotFound when absent", func() {
		_, err := s.Get(ctx, "nope.example.com", "A")
		Expect(err).To(MatchError(domaindns.ErrFQDNNotFound))
	})
})
```

- [ ] **Step 4.6.2: Implement `List`/`Get`/`Count`**

```go
func (s *FQDNStore) List(ctx context.Context, f domaindns.FQDNFilters) ([]domaindns.FQDNView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pool []*domaindns.FQDNView
	if f.Portal != "" {
		set := s.byPortal[f.Portal]
		pool = make([]*domaindns.FQDNView, 0, len(set))
		for k := range set {
			if v := s.fqdns[k]; v != nil {
				pool = append(pool, v)
			}
		}
	} else {
		pool = make([]*domaindns.FQDNView, 0, len(s.fqdns))
		for _, v := range s.fqdns {
			pool = append(pool, v)
		}
	}

	searchLower := strings.ToLower(f.Search)
	out := make([]domaindns.FQDNView, 0, len(pool))
	for _, v := range pool {
		if f.Namespace != "" && v.Namespace != f.Namespace {
			continue
		}
		if f.Source != "" && string(v.Source) != f.Source {
			continue
		}
		if f.Search != "" && !strings.Contains(strings.ToLower(v.Name), searchLower) {
			continue
		}
		out = append(out, *v) // copy
	}
	slices.SortFunc(out, func(a, b domaindns.FQDNView) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return cmp.Compare(a.RecordType, b.RecordType)
	})
	return out, nil
}

func (s *FQDNStore) Get(ctx context.Context, name, recordType string) (domaindns.FQDNView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lname := strings.ToLower(name)
	for k, v := range s.fqdns {
		if strings.ToLower(k.Name) != lname {
			continue
		}
		if recordType == "" || k.RecordType == recordType {
			return *v, nil
		}
	}
	return domaindns.FQDNView{}, fmt.Errorf("%w: %s/%s", domaindns.ErrFQDNNotFound, name, recordType)
}

func (s *FQDNStore) Count(ctx context.Context, f domaindns.FQDNFilters) (int, error) {
	out, err := s.List(ctx, f)
	return len(out), err
}
```

Add imports: `cmp`, `fmt`, `slices`, `strings`.

- [ ] **Step 4.6.3: Run (should pass)**

```
go test ./internal/readstore/dns/...
```

Expected: PASS.

- [ ] **Step 4.6.4: Commit**

```
git add internal/readstore/dns/
git commit -m "feat(readstore): List/Get/Count over portal-indexed deduped store"
```

### Task 4.7: TDD — concurrent stress test

**Files:**
- Modify: `internal/readstore/dns/fqdn_store_test.go`

- [ ] **Step 4.7.1: Write concurrency test**

```go
It("survives concurrent Replace and Delete with consistent invariants", func() {
	s := NewFQDNStore()
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				key := fmt.Sprintf("ns/rec-%d", (i+w)%16)
				portal := fmt.Sprintf("p%d", i%4)
				if i%5 == 0 {
					_ = s.Delete(ctx, key)
					continue
				}
				_ = s.Replace(ctx, key, portal, []domaindns.FQDNView{
					{Name: fmt.Sprintf("fqdn-%d.example.com", i%32), RecordType: "A", Targets: []string{"1.1.1.1"}},
				})
			}
		}(w)
	}
	wg.Wait()

	// Invariants: every FQDN in fqdns has non-empty Portals,
	// and every (portal, key) in byPortal maps to an existing fqdn.
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, v := range s.fqdns {
		Expect(v.Portals).NotTo(BeEmpty(), "fqdn %v has no portals", k)
	}
	for p, set := range s.byPortal {
		for k := range set {
			_, ok := s.fqdns[k]
			Expect(ok).To(BeTrue(), "portal %s references missing fqdn %v", p, k)
		}
	}
})
```

- [ ] **Step 4.7.2: Run with -race**

```
go test -race ./internal/readstore/dns/...
```

Expected: PASS.

If failures: most likely the `go s.broadcast()` after unlocking is fine, but reading `s.mu` directly in the test after the workers stop requires they're done — `wg.Wait()` handles that. If you see a race on `Portals` slice, the fix is to deep-copy the slice when returning from `List`/`Get` (already done via `*v` value copy + `removeString`/`mergeStrings` returning new slices).

- [ ] **Step 4.7.3: Commit**

```
git add internal/readstore/dns/
git commit -m "test(readstore): concurrent Replace/Delete stress test with invariants"
```

### Task 4.8: Expose conflicts for DNS controller

**Files:**
- Modify: `internal/readstore/dns/fqdn_store.go`
- Modify: `internal/domain/dns/reader.go`

- [ ] **Step 4.8.1: Add `ConflictsFor(dnsNS, dnsName)` accessor on the store**

We need DNS-owner tagging on writes. Extend `Replace` to also accept the owner DNS name (or compute it from `recordKey` if recordKey encodes it — `namespace/dnsrecord_name`, not the DNS name). Simpler: include the owner DNS name in the call. Update:

```go
// FQDNWriter (domain): no signature change; instead add a separate method.
type FQDNConflictReader interface {
	Conflicts(dnsNamespace, dnsName string) []ConflictEvent
}
```

For now, the store stores conflicts globally keyed by `LoserRecord`. We'll resolve LoserDNS at write time by passing the owner DNS name as an additional argument. Add a new `Writer.Replace`-like helper or extend signature:

```go
// In writer.go add:
type FQDNWriter interface {
	Replace(ctx context.Context, recordKey, portalRef string, fqdns []FQDNView) error
	Delete(ctx context.Context, recordKey string) error
	// Annotate is called by the DNSRecord controller after Replace to attach
	// the owner DNS (namespace/name) so conflicts can be reported per DNS.
	AnnotateOwner(recordKey, dnsNamespace, dnsName string)
}
```

Implement `AnnotateOwner` on `FQDNStore`:

```go
func (s *FQDNStore) AnnotateOwner(recordKey, dnsNS, dnsName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.byRecord[recordKey]
	c.dnsNamespace = dnsNS
	c.dnsName = dnsName
	s.byRecord[recordKey] = c
}
```

(Update `recordContribution` to include `dnsNamespace string; dnsName string`.)

Then in `Replace`, when pushing a conflict, look up the loser's contribution to populate `LoserDNS`. The first-time write doesn't yet have ownership annotated — that's OK, populate at the next conflict push using `s.byRecord[recordKey]`.

Expose:

```go
func (s *FQDNStore) Conflicts(dnsNS, dnsName string) []ConflictEvent {
	all := s.conflicts.Snapshot()
	out := make([]ConflictEvent, 0)
	for _, e := range all {
		if e.LoserDNS == dnsNS+"/"+dnsName {
			out = append(out, e)
		}
	}
	return out
}
```

- [ ] **Step 4.8.2: Tests**

Add an `It` in `fqdn_store_test.go`:

```go
It("returns conflicts scoped to a DNS owner", func() {
	s := NewFQDNStore()
	_ = s.Replace(ctx, "ns/a", "p", []domaindns.FQDNView{{Name:"x.example.com", RecordType:"A", Targets:[]string{"1.1.1.1"}}})
	s.AnnotateOwner("ns/a", "ns", "dns-a")
	_ = s.Replace(ctx, "ns/b", "p", []domaindns.FQDNView{{Name:"x.example.com", RecordType:"A", Targets:[]string{"2.2.2.2"}}})
	s.AnnotateOwner("ns/b", "ns", "dns-b")

	Expect(s.Conflicts("ns", "dns-b")).To(HaveLen(1))
	Expect(s.Conflicts("ns", "dns-a")).To(BeEmpty())
})
```

- [ ] **Step 4.8.3: Run**

```
go test ./internal/readstore/dns/... ./internal/domain/dns/...
```

Expected: PASS.

- [ ] **Step 4.8.4: Commit**

```
git add internal/readstore/dns/ internal/domain/dns/
git commit -m "feat(readstore): per-DNS conflict accessor for TargetsConflict condition"
```

---

## Phase 5 — DNSRecord controller fast-out & ownership wiring

### Task 5.1: Fast-out when Portal absent

**Files:**
- Modify: `internal/controller/dnsrecords/dnsrecord_controller.go`
- Modify: `internal/controller/dnsrecords/dnsrecord_controller_test.go`

- [ ] **Step 5.1.1: Add test for portal-absent fast-out**

In `dnsrecord_controller_test.go`, add:

```go
It("fast-outs and cleans the read store when the referenced Portal does not exist", func() {
	// Create a DNS (owner) without the Portal existing.
	dns := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "dns-orphan", Namespace: "default"},
		Spec: v1alpha2.DNSSpec{PortalRef: "missing-portal"},
	}
	Expect(k8sClient.Create(ctx, dns)).To(Succeed())
	rec := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rec-orphan", Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1alpha2.GroupVersion.String(), Kind: "DNS", Name: "dns-orphan",
				UID: dns.UID, Controller: ptr.To(true), BlockOwnerDeletion: ptr.To(true),
			}},
		},
		Spec: v1alpha2.DNSRecordSpec{
			Origin: v1alpha2.DNSRecordOriginManual, PortalRef: "missing-portal",
			Entries: []v1alpha2.DNSRecordEntry{{FQDN: "foo.example.com.", RecordType: "A"}},
		},
	}
	Expect(k8sClient.Create(ctx, rec)).To(Succeed())

	Eventually(func() error {
		_, err := writer.Get(ctx, "foo.example.com", "A")
		return err
	}).Should(MatchError(domaindns.ErrFQDNNotFound))
})
```

- [ ] **Step 5.1.2: Run (should fail or hang on retries)**

```
go test ./internal/controller/dnsrecords/...
```

Expected: FAIL.

- [ ] **Step 5.1.3: Implement fast-out**

In `dnsrecord_controller.go` `Reconcile`, before invoking the chain, after the existing feature-disabled check, add a Portal existence probe:

```go
var portal sreportalv1alpha1.Portal
if err := r.Get(ctx, types.NamespacedName{Namespace: record.Namespace, Name: record.Spec.PortalRef}, &portal); err != nil {
	if apierrors.IsNotFound(err) {
		logger.Info("portal not found, dropping DNSRecord from read store", "portal", record.Spec.PortalRef)
		if r.fqdnWriter != nil {
			resourceKey := record.Namespace + "/" + record.Name
			if wErr := r.fqdnWriter.Delete(ctx, resourceKey); wErr != nil {
				return ctrl.Result{}, wErr
			}
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, err
}
```

(Note: `apierrors "k8s.io/apimachinery/pkg/api/errors"` and `types` imports.)

- [ ] **Step 5.1.4: Run (should pass)**

```
go test ./internal/controller/dnsrecords/...
```

Expected: PASS.

- [ ] **Step 5.1.5: Commit**

```
git add internal/controller/dnsrecords/
git commit -m "feat(dnsrecord): fast-out and clean readstore when portal is absent"
```

### Task 5.2: Pass owner DNS to writer on every successful Replace

**Files:**
- Modify: `internal/controller/dnsrecords/chain/project_store.go`
- Modify: `internal/controller/dnsrecords/dnsrecord_controller.go`

- [ ] **Step 5.2.1: Carry owner DNS name in ChainData**

Edit `internal/controller/dnsrecords/chain/chain_data.go` (read existing file first to preserve fields). Add:

```go
type ChainData struct {
	// ... existing fields
	OwnerDNSName string
}
```

In `dnsrecord_controller.go`, after fetching the owner DNS (we already do this transitively when checking portal; if not yet, fetch via ownerReferences), set `rc.Data.OwnerDNSName`. Simplest: read the controller ownerRef on the DNSRecord:

```go
ownerName := ""
for _, or := range record.OwnerReferences {
	if or.Controller != nil && *or.Controller && or.Kind == "DNS" {
		ownerName = or.Name
		break
	}
}
```

Assign to `rc.Data.OwnerDNSName = ownerName`.

- [ ] **Step 5.2.2: Annotate the store in `ProjectStoreHandler`**

In `project_store.go`, after the successful `Replace` call:

```go
if w, ok := h.fqdnWriter.(interface {
	AnnotateOwner(recordKey, dnsNS, dnsName string)
}); ok {
	w.AnnotateOwner(rc.Data.ResourceKey, rc.Resource.Namespace, rc.Data.OwnerDNSName)
}
```

- [ ] **Step 5.2.3: Test**

Run:
```
go test ./internal/controller/dnsrecords/...
```

Expected: PASS.

- [ ] **Step 5.2.4: Commit**

```
git add internal/controller/dnsrecords/
git commit -m "feat(dnsrecord): annotate readstore with owner DNS for per-DNS conflict reporting"
```

---

## Phase 6 — Source controller: global informer + resolver-based DNSRecord production

This is the largest phase. Split into resolver refactor (6.A) then global informer + per-DNS pipeline (6.B), then MergeConfigs removal (6.C).

### Task 6.1: Define `Resolver` and `Filter`

**Files:**
- Create: `internal/source/registry/resolver.go`

- [ ] **Step 6.1.1: Define the resolver interface**

```go
package registry

import (
	"context"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Filter encodes the namespace and label selector applied to source objects
// pulled from the controller-runtime cache.
type Filter struct {
	Namespace   string
	LabelFilter string // labels.Selector syntax
}

// Resolver converts a slice of typed cache objects to external-dns Endpoints.
// One Resolver per source kind.
type Resolver interface {
	Type() SourceType
	// ObjectList returns a fresh empty list suitable for cache.List.
	ObjectList() client.ObjectList
	// Resolve converts the filtered items into Endpoints.
	Resolve(ctx context.Context, items []client.Object, filter Filter) ([]*endpoint.Endpoint, error)
}
```

- [ ] **Step 6.1.2: Build**

```
go build ./internal/source/...
```

Expected: passes.

- [ ] **Step 6.1.3: Commit**

```
git add internal/source/registry/
git commit -m "feat(source): introduce Resolver interface (cache-backed source conversion)"
```

### Task 6.2: Implement `ServiceResolver` as the pilot

**Files:**
- Create: `internal/source/service/resolver.go`
- Create: `internal/source/service/resolver_test.go`

- [ ] **Step 6.2.1: Test ServiceResolver**

Write `resolver_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golgoth31/sreportal/internal/source/registry"
	svcsrc "github.com/golgoth31/sreportal/internal/source/service"
)

func TestServiceResolver_Resolve(t *testing.T) {
	r := svcsrc.NewResolver()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "echo", Namespace: "default",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/hostname": "echo.example.com"},
		},
		Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: "1.2.3.4"}},
		}},
	}

	eps, err := r.Resolve(context.Background(), []client.Object{svc}, registry.Filter{})
	if err != nil { t.Fatal(err) }
	if len(eps) != 1 { t.Fatalf("want 1 endpoint, got %d", len(eps)) }
	if eps[0].DNSName != "echo.example.com" { t.Fatalf("bad dnsname: %s", eps[0].DNSName) }
}

func TestServiceResolver_FilterByLabel(t *testing.T) {
	r := svcsrc.NewResolver()
	labeled := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name:"a", Namespace:"x", Labels: map[string]string{"team":"a"}}}
	other := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name:"b", Namespace:"x", Labels: map[string]string{"team":"b"}}}
	eps, err := r.Resolve(context.Background(), []client.Object{labeled, other}, registry.Filter{LabelFilter: "team=a"})
	if err != nil { t.Fatal(err) }
	// Neither has hostname annotation → no endpoints; success on no panic.
	_ = eps
	// More directly: assert labels.Parse handled the filter.
	sel, _ := labels.Parse("team=a")
	if !sel.Matches(labels.Set(labeled.Labels)) { t.Fatal("selector should match") }
}
```

- [ ] **Step 6.2.2: Implement**

```go
package service

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

type Resolver struct{}

func NewResolver() *Resolver { return &Resolver{} }

func (*Resolver) Type() registry.SourceType { return registry.SourceTypeService }

func (*Resolver) ObjectList() client.ObjectList { return &corev1.ServiceList{} }

func (*Resolver) Resolve(_ context.Context, items []client.Object, f registry.Filter) ([]*endpoint.Endpoint, error) {
	sel := labels.Everything()
	if f.LabelFilter != "" {
		s, err := labels.Parse(f.LabelFilter)
		if err != nil { return nil, err }
		sel = s
	}
	var out []*endpoint.Endpoint
	for _, obj := range items {
		svc, ok := obj.(*corev1.Service)
		if !ok { continue }
		if f.Namespace != "" && svc.Namespace != f.Namespace { continue }
		if !sel.Matches(labels.Set(svc.Labels)) { continue }
		host := svc.Annotations["external-dns.alpha.kubernetes.io/hostname"]
		if host == "" { continue }
		ips := loadBalancerIPs(svc)
		if len(ips) == 0 { continue }
		out = append(out, endpoint.NewEndpoint(strings.TrimSuffix(host, "."), endpoint.RecordTypeA, ips...))
	}
	return out, nil
}

func loadBalancerIPs(svc *corev1.Service) []string {
	var ips []string
	for _, lb := range svc.Status.LoadBalancer.Ingress {
		if lb.IP != "" { ips = append(ips, lb.IP) }
	}
	return ips
}
```

This is intentionally minimal (LoadBalancer IP + hostname annotation). The current `internal/source/service` builder uses the full external-dns lib; we re-implement only what we need.

- [ ] **Step 6.2.3: Run**

```
go test ./internal/source/service/...
```

Expected: PASS.

- [ ] **Step 6.2.4: Commit**

```
git add internal/source/service/ internal/source/registry/
git commit -m "feat(source/service): cache-backed Resolver (Service → endpoints)"
```

### Task 6.3: Implement remaining Resolvers

For each existing source kind (`ingress`, `dnsendpoint`, `istiogateway`, `istiovirtualservice`, `gatewayhttproute`, `gatewaygrpcroute`, `gatewaytlsroute`, `gatewaytcproute`, `gatewayudproute`, `crossplanescalewayrecord`):

- [ ] **Step 6.3.x: Repeat the Task 6.2 pattern**

Each kind gets its own `<kind>/resolver.go` + `_test.go`. Mirror the structure: `Type()`, `ObjectList()`, `Resolve()`. Reuse the existing per-kind conversion logic from each `builder.go` — extract the endpoint-construction code into the Resolver, drop the informer wiring.

After each kind:
```
go test ./internal/source/<kind>/...
git add internal/source/<kind>/
git commit -m "feat(source/<kind>): cache-backed Resolver"
```

For `dnsendpoint` (the simplest — it's a 1:1 CR-to-endpoints), and `crossplanescalewayrecord` (similar), the resolver is straightforward.

For Gateway/Istio kinds, reuse the existing endpoint-extraction helpers from the current builders rather than rewriting from scratch.

**Acceptance for this task block:** all source kinds have a resolver with passing tests. Do not delete the old `builder.go` yet — both coexist until Task 6.5.

### Task 6.4: Source kind registry

**Files:**
- Create: `internal/source/registry/resolvers.go`

- [ ] **Step 6.4.1: Build a lookup**

```go
package registry

import "sync"

var (
	resolversMu sync.RWMutex
	resolvers   = map[SourceType]Resolver{}
)

func RegisterResolver(r Resolver) {
	resolversMu.Lock()
	defer resolversMu.Unlock()
	resolvers[r.Type()] = r
}

func GetResolver(t SourceType) (Resolver, bool) {
	resolversMu.RLock()
	defer resolversMu.RUnlock()
	r, ok := resolvers[t]
	return r, ok
}

func AllResolvers() []Resolver {
	resolversMu.RLock()
	defer resolversMu.RUnlock()
	out := make([]Resolver, 0, len(resolvers))
	for _, r := range resolvers {
		out = append(out, r)
	}
	return out
}
```

In each `<kind>/resolver.go`, add `func init() { registry.RegisterResolver(&Resolver{}) }`.

- [ ] **Step 6.4.2: Build**

```
go build ./...
```

- [ ] **Step 6.4.3: Commit**

```
git add internal/source/registry/ internal/source/
git commit -m "feat(source): central resolver registry with init-time registration"
```

### Task 6.5: New source controller — global informer + per-DNS production

This task replaces `internal/controller/source/source_controller.go` (732 lines) with a new pipeline. We rewrite, not patch.

**Files:**
- Modify (substantial rewrite): `internal/controller/source/source_controller.go`
- Modify or delete: `internal/controller/source/merged_config.go` (delete)
- Modify or delete: `internal/controller/source/merged_config_test.go` (delete)
- Modify or delete: `internal/controller/source/dns_config_notifier.go` (likely keep, see notes)

- [ ] **Step 6.5.1: Sketch the new structure**

The new controller is a controller-runtime `Reconciler` for `v1alpha2.DNS`. On each reconcile of a DNS:

1. Resolve owner Portal; if absent → no-op.
2. For each enabled source kind in `spec.sources`:
   - Compute `(ns, labelFilter)` = source override OR `spec.defaults`.
   - List from cache: `r.cache.List(ctx, resolver.ObjectList(), client.InNamespace(ns))` (or no namespace if `ns==""`).
   - Convert to `[]client.Object`.
   - `resolver.Resolve(ctx, items, registry.Filter{Namespace: ns, LabelFilter: labelFilter})`.
3. Apply intra-DNS priority: deduplicate endpoints across source kinds by `dnsName`, keep highest-priority kind per `spec.sources.priority`.
4. Group endpoints by source kind. For each kind producing ≥1 endpoint:
   - Upsert one `DNSRecord` owned by this DNS, named `<dns-name>-<kind>` (deterministic).
   - `controllerutil.SetControllerReference(&dns, &record, scheme)`.
5. Delete owned auto DNSRecords whose source kind no longer produces output.

```go
package source

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Cache  client.Reader // controller-runtime cache (mgr.GetCache().GetClient() or mgr.GetClient())
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var dns v1alpha2.DNS
	if err := r.Get(ctx, req.NamespacedName, &dns); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if dns.Spec.IsRemote {
		return ctrl.Result{}, nil
	}

	// Portal must exist.
	var portal sreportalv1alpha1.Portal
	if err := r.Get(ctx, types.NamespacedName{Namespace: dns.Namespace, Name: dns.Spec.PortalRef}, &portal); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil // fast-out; DNSRecord controller will GC
		}
		return ctrl.Result{}, err
	}

	produced, err := r.produceEndpointsForDNS(ctx, &dns)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileOwnedRecords(ctx, &dns, produced); err != nil {
		return ctrl.Result{}, err
	}

	interval := dns.Spec.Reconciliation.Interval.Duration
	if interval == 0 {
		interval = 5 * time.Minute
	}
	return ctrl.Result{RequeueAfter: interval}, nil
}
```

Then implement `produceEndpointsForDNS` and `reconcileOwnedRecords` in the same file.

- [ ] **Step 6.5.2: Implement `produceEndpointsForDNS`**

```go
func (r *Reconciler) produceEndpointsForDNS(ctx context.Context, dns *v1alpha2.DNS) (map[registry.SourceType][]*endpoint.Endpoint, error) {
	out := map[registry.SourceType][]*endpoint.Endpoint{}

	for _, kindSrc := range enumerateEnabled(dns) {
		resolver, ok := registry.GetResolver(kindSrc.Type)
		if !ok {
			continue
		}
		filter := mergeFilters(dns.Spec.Defaults, kindSrc.Filter)

		list := resolver.ObjectList()
		opts := []client.ListOption{}
		if filter.Namespace != "" {
			opts = append(opts, client.InNamespace(filter.Namespace))
		}
		if err := r.Cache.List(ctx, list, opts...); err != nil {
			return nil, fmt.Errorf("list %s: %w", kindSrc.Type, err)
		}
		items, err := flattenList(list)
		if err != nil {
			return nil, err
		}
		eps, err := resolver.Resolve(ctx, items, filter)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", kindSrc.Type, err)
		}
		out[kindSrc.Type] = eps
	}

	return applyIntraDNSPriority(out, dns.Spec.Sources.Priority), nil
}
```

`enumerateEnabled` walks the `DNSSpec.Sources` pointer fields, returns `[]struct{Type SourceType; Filter registry.Filter}`. `flattenList` uses `meta.ExtractList(list)` from `k8s.io/apimachinery/pkg/api/meta` then casts each item to `client.Object`. `applyIntraDNSPriority` walks the priority list in order; for each `dnsName`, keep only the entry from the highest-priority kind.

- [ ] **Step 6.5.3: Implement `reconcileOwnedRecords`**

```go
func (r *Reconciler) reconcileOwnedRecords(ctx context.Context, dns *v1alpha2.DNS, produced map[registry.SourceType][]*endpoint.Endpoint) error {
	// List existing owned auto records.
	var existing v1alpha2.DNSRecordList
	if err := r.List(ctx, &existing, client.InNamespace(dns.Namespace)); err != nil {
		return err
	}
	owned := map[registry.SourceType]*v1alpha2.DNSRecord{}
	for i := range existing.Items {
		rec := &existing.Items[i]
		if !isOwnedBy(rec, dns) { continue }
		if rec.Spec.Origin != v1alpha2.DNSRecordOriginAuto { continue }
		owned[registry.SourceType(rec.Spec.SourceType)] = rec
	}

	// Upsert one record per produced kind.
	for kind, eps := range produced {
		if len(eps) == 0 {
			continue
		}
		rec := owned[kind]
		if rec == nil {
			rec = &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: dns.Namespace,
					Name: fmt.Sprintf("%s-%s", dns.Name, string(kind)),
				},
				Spec: v1alpha2.DNSRecordSpec{
					Origin: v1alpha2.DNSRecordOriginAuto,
					PortalRef: dns.Spec.PortalRef,
					SourceType: v1alpha2.SourceType(kind),
				},
			}
			if err := controllerutil.SetControllerReference(dns, rec, r.Scheme); err != nil {
				return err
			}
			if err := r.Create(ctx, rec); err != nil {
				return err
			}
		}
		// Status (endpoints) is set by DNSRecord controller after Resolve. Trigger by patching annotation
		// or via field update if needed. For now, the Source controller writes the *spec* only; endpoints
		// land via DNSRecord chain. We just stamp the eps in status to avoid a second discovery pass:
		rec.Status.Endpoints = epsToEndpointStatus(eps)
		if err := r.Status().Update(ctx, rec); err != nil {
			return err
		}
		delete(owned, kind)
	}

	// Delete leftover owned records whose kind disappeared.
	for kind, rec := range owned {
		if err := r.Delete(ctx, rec); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		_ = kind
	}
	return nil
}
```

`isOwnedBy(rec, dns)` checks the controller ownerRef UID matches. `epsToEndpointStatus` converts `*endpoint.Endpoint` to the existing `v1alpha2.EndpointStatus` shape (LastSeen=now, TTL from ep, etc.).

- [ ] **Step 6.5.4: Watches**

```go
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Cache = mgr.GetClient()
	b := ctrl.NewControllerManagedBy(mgr).For(&v1alpha2.DNS{}).Named("source")

	// Watch each kind referenced by at least one Resolver, so cache events trigger
	// reconciliation of all DNS CRs.
	for _, res := range registry.AllResolvers() {
		obj := res.ObjectList()
		// Extract the item type via meta.NewListPtr or use the embedded GVK.
		itemType := itemTypeOf(obj) // returns client.Object representing one item
		b = b.Watches(itemType, handler.EnqueueRequestsFromMapFunc(r.mapToAllDNS))
	}
	return b.Complete(r)
}

func (r *Reconciler) mapToAllDNS(ctx context.Context, _ client.Object) []ctrl.Request {
	var list v1alpha2.DNSList
	if err := r.Cache.List(ctx, &list); err != nil { return nil }
	out := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&list.Items[i])})
	}
	return out
}
```

`itemTypeOf` returns a `client.Object` derived from a `client.ObjectList` (e.g. `&corev1.Service{}` from `*corev1.ServiceList{}`). Use `reflect.New(reflect.TypeOf(list).Elem().FieldByName("Items").Type.Elem())` — or simpler, add `ItemType() client.Object` to the `Resolver` interface (revisit Task 6.1 to add this method if you choose the cleaner route).

- [ ] **Step 6.5.5: Tests**

Write `source_controller_test.go` cases:

- Reconcile of DNS with Service source + no Portal → no DNSRecord created.
- Reconcile of DNS with Service source + Portal exists + matching Service in cache → 1 DNSRecord auto/service created, ownerRef set, status.Endpoints populated.
- Reconcile after Service source toggled off → DNSRecord auto/service deleted.
- Reconcile of two DNS sharing the same selector → both produce their own DNSRecord (no merge, no conflict).
- Priority: Service and Ingress both produce `foo.example.com`, `priority=[service,ingress]` → record for ingress excludes `foo.example.com`.

- [ ] **Step 6.5.6: Run**

```
go test ./internal/controller/source/...
```

Expected: PASS.

- [ ] **Step 6.5.7: Commit**

```
git add internal/controller/source/ internal/source/
git commit -m "feat(source): rewrite controller around per-DNS cache resolution + ownerRef-owned DNSRecords"
```

### Task 6.6: Remove MergeConfigs and SourceConflict

**Files:**
- Delete: `internal/controller/source/merged_config.go`
- Delete: `internal/controller/source/merged_config_test.go`
- Audit: any consumer of `SourceConflict` condition

- [ ] **Step 6.6.1: Delete merge files**

```
git rm internal/controller/source/merged_config.go internal/controller/source/merged_config_test.go
```

- [ ] **Step 6.6.2: Strip references**

```
grep -rn "MergeConfigs\|SourceConflict" internal/ cmd/
```

For each hit, delete or replace by the new model (the source controller no longer publishes `SourceConflict`; it's replaced by `TargetsConflict` written by DNS controller in Phase 7).

- [ ] **Step 6.6.3: Build + test**

```
go build ./... && go test ./...
```

Expected: PASS. Some controller tests may need fixture updates.

- [ ] **Step 6.6.4: Commit**

```
git add -A
git commit -m "refactor(source): drop MergeConfigs and SourceConflict (per-DNS independence)"
```

### Task 6.7: Delete old per-source `builder.go` files

Each `internal/source/<kind>/` still has a `builder.go` from the previous architecture. After Task 6.3 each kind has a `resolver.go`. Now we drop the old code.

- [ ] **Step 6.7.1: For each kind, delete the builder**

```
git rm internal/source/<kind>/builder.go internal/source/<kind>/builder_test.go
```

Repeat for: service, ingress, dnsendpoint, istiogateway, istiovirtualservice, gatewayhttproute, gatewaygrpcroute, gatewaytlsroute, gatewaytcproute, gatewayudproute, crossplanescalewayrecord.

- [ ] **Step 6.7.2: Delete `internal/source/source.go` (DefaultBuilders)**

It's no longer needed. The new controller uses `registry.AllResolvers()`.

```
git rm internal/source/source.go
```

- [ ] **Step 6.7.3: Delete `internal/source/factory.go` and `internal/source/factory_test.go`**

```
git rm internal/source/factory.go internal/source/factory_test.go
```

- [ ] **Step 6.7.4: Update `cmd/main.go` wiring**

Replace any `source.DefaultBuilders()` / `source.NewFactory(...)` calls with the new `source.Reconciler` setup (instantiation + `SetupWithManager`). Look at the `cmd/main.go` near the existing `SourceReconciler` registration.

- [ ] **Step 6.7.5: Build + test**

```
go build ./... && go test ./...
```

Expected: PASS.

- [ ] **Step 6.7.6: Commit**

```
git add -A
git commit -m "refactor(source): remove old builders/factory now that resolvers replace them"
```

---

## Phase 7 — DNS controller slim

### Task 7.1: Replace chain handlers

**Files:**
- Modify: `internal/controller/dns/dns_controller.go`
- Delete: `internal/controller/dns/chain/aggregate_dnsrecords.go` and tests
- Delete: `internal/controller/dns/chain/build_group_status.go`
- Modify: `internal/controller/dns/chain/update_status.go`
- Create: `internal/controller/dns/chain/check_portal.go`
- Create: `internal/controller/dns/chain/aggregate_conditions.go`

- [ ] **Step 7.1.1: Write tests for new chain steps**

Create `internal/controller/dns/chain/check_portal_test.go`:

```go
It("sets Ready=False and short-circuits when portal absent", func() {
	h := chain.NewCheckPortalHandler(k8sClient)
	dns := &v1alpha2.DNS{ /* portalRef=missing */ }
	rc := &reconciler.ReconcileContext[*v1alpha2.DNS, chain.ChainData]{Resource: dns}
	err := h.Handle(ctx, rc)
	Expect(err).NotTo(HaveOccurred())
	Expect(rc.Data.PortalAbsent).To(BeTrue())
})
```

Create `internal/controller/dns/chain/aggregate_conditions_test.go`:

```go
It("stamps TargetsConflict=True when readstore reports a conflict for this DNS", func() {
	store := readstoredns.NewFQDNStore()
	_ = store.Replace(ctx, "default/r1", "p", []domaindns.FQDNView{{Name:"x.example.com", RecordType:"A", Targets:[]string{"1.1.1.1"}}})
	store.AnnotateOwner("default/r1", "default", "dns-a")
	_ = store.Replace(ctx, "default/r2", "p", []domaindns.FQDNView{{Name:"x.example.com", RecordType:"A", Targets:[]string{"2.2.2.2"}}})
	store.AnnotateOwner("default/r2", "default", "dns-b")

	h := chain.NewAggregateConditionsHandler(store)
	dns := &v1alpha2.DNS{ObjectMeta: metav1.ObjectMeta{Name:"dns-b", Namespace:"default"}, Spec: v1alpha2.DNSSpec{PortalRef:"p"}}
	rc := &reconciler.ReconcileContext[*v1alpha2.DNS, chain.ChainData]{Resource: dns}
	Expect(h.Handle(ctx, rc)).To(Succeed())

	cond := findCondition(dns.Status.Conditions, v1alpha2.ConditionTargetsConflict)
	Expect(cond).NotTo(BeNil())
	Expect(cond.Status).To(Equal(metav1.ConditionTrue))
})
```

- [ ] **Step 7.1.2: Implement `CheckPortalHandler`**

```go
type CheckPortalHandler struct{ c client.Client }
func NewCheckPortalHandler(c client.Client) *CheckPortalHandler { return &CheckPortalHandler{c: c} }
func (h *CheckPortalHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNS, ChainData]) error {
	var p sreportalv1alpha1.Portal
	err := h.c.Get(ctx, types.NamespacedName{Namespace: rc.Resource.Namespace, Name: rc.Resource.Spec.PortalRef}, &p)
	if apierrors.IsNotFound(err) {
		rc.Data.PortalAbsent = true
		setCondition(&rc.Resource.Status.Conditions, metav1.Condition{
			Type: v1alpha2.ConditionReady, Status: metav1.ConditionFalse, Reason: "PortalAbsent",
			Message: "referenced Portal does not exist",
		})
		rc.Result = ctrl.Result{} // no requeue
		return nil
	}
	return err
}
```

Add `PortalAbsent bool` to `ChainData` (in `chain_data.go`).

- [ ] **Step 7.1.3: Implement `AggregateConditionsHandler`**

```go
type AggregateConditionsHandler struct{ store *readstoredns.FQDNStore }
func NewAggregateConditionsHandler(s *readstoredns.FQDNStore) *AggregateConditionsHandler { return &AggregateConditionsHandler{store: s} }
func (h *AggregateConditionsHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNS, ChainData]) error {
	if rc.Data.PortalAbsent { return nil }
	conflicts := h.store.Conflicts(rc.Resource.Namespace, rc.Resource.Name)
	cond := metav1.Condition{Type: v1alpha2.ConditionTargetsConflict}
	if len(conflicts) > 0 {
		cond.Status = metav1.ConditionTrue
		cond.Reason = "TargetsDiverge"
		cond.Message = fmt.Sprintf("%d FQDN(s) conflict with previously written targets", len(conflicts))
	} else {
		cond.Status = metav1.ConditionFalse
		cond.Reason = "NoConflicts"
		cond.Message = "no inter-DNS target conflicts"
	}
	setCondition(&rc.Resource.Status.Conditions, cond)
	setCondition(&rc.Resource.Status.Conditions, metav1.Condition{
		Type: v1alpha2.ConditionReady, Status: metav1.ConditionTrue, Reason: "Reconciled",
	})
	return nil
}
```

`setCondition` is a small helper using `meta.SetStatusCondition` from `k8s.io/apimachinery/pkg/api/meta`.

- [ ] **Step 7.1.4: Slim `UpdateStatusHandler`**

Modify `update_status.go` to only write `ActiveSources`, `LastReconcileTime`, `ObservedGeneration`, `Conditions`. Remove any reference to `Groups`.

- [ ] **Step 7.1.5: Update `dns_controller.go` chain**

```go
handlers := []reconciler.Handler[*v1alpha2.DNS, dnschain.ChainData]{
	dnschain.NewCheckPortalHandler(c),
	dnschain.NewAggregateConditionsHandler(fqdnStore),
	dnschain.NewUpdateStatusHandler(c),
}
```

Inject `*readstoredns.FQDNStore` (or a domain interface) into the reconciler constructor. Update `cmd/main.go` wiring.

- [ ] **Step 7.1.6: Delete old handlers**

```
git rm internal/controller/dns/chain/aggregate_dnsrecords.go internal/controller/dns/chain/aggregate_dnsrecords_test.go
git rm internal/controller/dns/chain/build_group_status.go
```

- [ ] **Step 7.1.7: Run**

```
go test ./internal/controller/dns/...
```

Expected: PASS.

- [ ] **Step 7.1.8: Commit**

```
git add -A
git commit -m "refactor(dns controller): slim chain, conditions only (readstore is source of truth)"
```

---

## Phase 8 — Proto + WebUI

### Task 8.1: Update proto

**Files:**
- Modify: `proto/sreportal/v1/dns.proto`

- [ ] **Step 8.1.1: Replace single-portal fields with list**

In `FQDN` message:

```proto
// dns_resource_name kept for back-compat with older clients (deprecated).
string dns_resource_name = 8 [deprecated = true];

// portals lists every portal this FQDN belongs to (post-dedup).
repeated string portals = 12;
```

(Drop `dns_resource_namespace = 9` similarly if no consumer uses it; else mark deprecated and keep.)

- [ ] **Step 8.1.2: Regenerate**

```
make proto
```

Expected: `internal/grpc/gen/` and `web/src/gen/` updated.

- [ ] **Step 8.1.3: Update adapter `FQDNView → proto.FQDN`**

In `internal/grpc/dns_service.go` (or wherever the conversion lives, grep for `dns_resource_name`):

```go
return &pb.FQDN{
	Name: v.Name,
	Source: string(v.Source),
	Groups: v.Groups,
	// ...
	Portals: v.Portals,
}
```

- [ ] **Step 8.1.4: Build**

```
go build ./...
(cd web && npx tsc -b)
```

- [ ] **Step 8.1.5: Commit**

```
git add proto/ internal/ web/
git commit -m "feat(proto): FQDN.portals repeated; deprecate dns_resource_name"
```

### Task 8.2: WebUI display

**Files:**
- Modify: `web/src/features/links/` (locate the FQDN row component)

- [ ] **Step 8.2.1: Render `portals` as chips**

Locate the FQDN list rendering. Replace any single-portal display with a chip group:

```tsx
{fqdn.portals?.map(p => <Badge key={p} variant="secondary">{p}</Badge>)}
```

(Use existing shadcn `Badge` component.)

- [ ] **Step 8.2.2: Run lint/build**

```
(cd web && npm run lint && npx tsc -b)
```

- [ ] **Step 8.2.3: Commit**

```
git add web/
git commit -m "feat(web/links): display all portals an FQDN belongs to as chips"
```

---

## Phase 9 — Metrics + cleanup

### Task 9.1: New metrics

**Files:**
- Modify: `internal/metrics/metrics.go` (or wherever metrics are declared — grep `DNSFQDNsTotal`)

- [ ] **Step 9.1.1: Register new metrics**

```go
var (
	DNSFQDNDedupRatio = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sreportal_dns_fqdn_dedup_ratio", Help: "Per-portal dedup ratio of FQDN entries.",
	}, []string{"portal"})
	DNSTargetsConflictTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "sreportal_dns_targets_conflict_total", Help: "Inter-DNS target conflicts.",
	}, []string{"dns", "portal"})
	DNSSourceKindActive = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sreportal_dns_source_kind_active", Help: "1 if at least one DNS enables this source kind.",
	}, []string{"kind"})
)

func init() {
	metrics.Registry.MustRegister(DNSFQDNDedupRatio, DNSTargetsConflictTotal, DNSSourceKindActive)
}
```

- [ ] **Step 9.1.2: Wire counters**

- `DNSTargetsConflictTotal`: increment in `FQDNStore.Replace` when a conflict is pushed.
- `DNSFQDNDedupRatio`: compute periodically in DNS controller (`unique_keys / total_contributions` per portal).
- `DNSSourceKindActive`: refresh in source controller after enumerating enabled sources.

- [ ] **Step 9.1.3: Build + test**

```
go build ./... && go test ./internal/metrics/... ./internal/readstore/...
```

- [ ] **Step 9.1.4: Commit**

```
git add internal/
git commit -m "feat(metrics): dedup ratio, targets conflict counter, active source kinds gauge"
```

### Task 9.2: Final cleanup

- [ ] **Step 9.2.1: Search for residual references to v1alpha1 DNS Groups status**

```
grep -rn "Status.Groups\|GroupsToFQDNViews\|sync_remote_dns" internal/ cmd/
```

Remove or update remaining references. `sync_remote_dns.go` may still need work for remote portal sync — leave it if it touches v1alpha1 only; otherwise update it to read from the new readstore.

- [ ] **Step 9.2.2: Make targets**

```
make helm
make test
make lint
make doc
```

Expected: all green.

- [ ] **Step 9.2.3: Final commit**

```
git add -A
git commit -m "chore(dns): cleanup after multi-CR refactor"
```

---

## Self-Review checklist (run after writing the plan, before execution)

- Spec §3 (CRD changes) → covered in Phase 1.
- Spec §4 (Source controller) → Phase 6 (resolvers + new controller + delete merge).
- Spec §5 (ReadStore) → Phase 4 (full rewrite TDD).
- Spec §6 (DNSRecord controller fast-out) → Phase 5.
- Spec §7 (DNS controller slim) → Phase 7.
- Spec §8 (Webhooks) → Phase 2 (DNS) + Phase 3 (DNSRecord).
- Spec §9 (Proto + WebUI + metrics) → Phase 8 + Phase 9.
- Spec §10 (migration) → no-op (isofunctional).
- Spec §11 open questions → conflict ring is implemented (snapshot-pull from DNS controller). The other two (Resolve batching, WebUI multi-portal display) are addressed in Phase 8.

**Type / signature consistency checks:**
- `FQDNWriter.Replace(ctx, recordKey, portalRef, fqdns)` — used in Phase 4 (declaration) and Phase 5 (callsite). ✓
- `FQDNStore.AnnotateOwner(recordKey, dnsNS, dnsName)` — declared Phase 4.8, called Phase 5.2. ✓
- `FQDNStore.Conflicts(dnsNS, dnsName)` — declared Phase 4.8, called Phase 7.1.3 (AggregateConditionsHandler). ✓
- `Resolver.Type/ObjectList/Resolve` — declared Phase 6.1, used Phase 6.5 (controller) and per-kind (Phase 6.2/6.3). ✓
- `ChainData.OwnerDNSName` — added Phase 5.2, used Phase 5.2. ✓
- `ChainData.PortalAbsent` — added Phase 7.1.2, used Phase 7.1.3. ✓
- `ConditionReady`, `ConditionSourcesReady`, `ConditionTargetsConflict` — declared Phase 1.1.3, used Phase 7. ✓
- `FQDNView.Portals []string` — declared Phase 4.1, used Phase 4 / Phase 5 / Phase 8. ✓

**Open follow-ups acknowledged in plan but deferred:**
- `Resolver.ItemType()` shortcut (mentioned in Task 6.5.4) — implement when convenient if reflection feels too hairy. Either reflection or interface extension works.
- Whether to keep `internal/source/factory_test.go` partially (some tests cover registry semantics that map to the new resolver registry). Recover what's reusable.

---

## After execution

Once all phases are complete:
- `make helm && make test && make lint && make doc` should all be green.
- Open a PR titled `feat(dns): multiple DNS CRs per portal + deduplicated read store`.
- Reviewer test plan: see spec §10 ordering — each phase is independently testable.
