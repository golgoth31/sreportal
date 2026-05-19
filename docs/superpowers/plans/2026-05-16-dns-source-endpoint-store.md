# DNS Source-Endpoint-Store Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the per-DNS source-construction architecture with a global, cluster-wide `SourceReconciler` that populates an in-memory `SourceEndpointStore` consumed by the `DNSReconciler` to produce `DNSRecord`s.

**Architecture:** A single `SourceReconciler` (manager.Runnable, periodic tick) lists every enabled kind cluster-wide, runs per-kind `Resolver.ResolveObject(obj)` once per source object, and writes the result into `SourceEndpointStore` indexed by `(SourceType, Namespace)`. Each `DNSReconciler` reconcile reads the store with `(kind, namespace, labelFilter)`, applies intra-DNS priority, and upserts owned `DNSRecord`s.

**Tech Stack:** Go 1.26, controller-runtime v0.23, sigs.k8s.io/external-dns, testify/Ginkgo+Gomega, envtest.

**Spec reference:** `docs/superpowers/specs/2026-05-15-dns-multi-cr-per-portal-design.md` §4 + §7.

**Pre-conditions (already done in prior phases):**
- v1alpha2 API with `SourceFilterDefaults` + ownerRef-based DNS↔DNSRecord linkage.
- `FQDNStore` with dedup, portal index, refcount, conflict ring, AnnotateOwner.
- DNSRecord controller fast-out + AnnotateOwner.
- Resolver interface skeleton (`internal/source/registry/resolver.go`) + ServiceResolver pilot — both with the **old** `Resolve(items, filter)` signature; this plan updates them to the new per-object form in Task 6.3.

---

## File structure

**New files:**
- `internal/domain/source/store.go` — `EnrichedEndpoint`, `SourceEndpointReader`, `SourceEndpointWriter`
- `internal/readstore/source/store.go` — `Store` implementation (in-memory, indexed by `(kind, namespace)`)
- `internal/readstore/source/store_test.go` — TDD coverage (Lookup, ReplaceKind, DeleteKind, concurrency)
- `internal/source/registry/registry_v2.go` — new `Registry` aggregating Resolvers (parallel to old `Builder`-based one during transition)
- `internal/source/<kind>/resolver.go` for each remaining kind: `ingress`, `dnsendpoint`, `istiogateway`, `istiovirtualservice`, `gatewayhttproute`, `gatewaygrpcroute`, `gatewaytcproute`, `gatewaytlsroute`, `gatewayudproute`, `crossplanescalewayrecord`
- `internal/source/<kind>/resolver_test.go` for each
- `internal/controller/source/cycle.go` — extracted `cycle()` body of the new SourceReconciler
- `internal/controller/components/runnable.go` — extracted Components production (out of SourceReconciler)
- `internal/controller/dns/chain/lookup_sources.go` — new chain handler
- `internal/controller/dns/chain/intra_dns_dedup.go` — new chain handler
- `internal/controller/dns/chain/upsert_dnsrecords.go` — new chain handler
- `internal/controller/dns/chain/sources_status.go` — new chain handler

**Files modified in-place:**
- `internal/source/registry/resolver.go` — drop `Filter` from `Resolve`, switch to `ResolveObject(obj)`
- `internal/source/service/resolver.go` — switch to `ResolveObject`
- `internal/source/service/resolver_test.go` — update tests
- `internal/controller/source/source_controller.go` — strip down to manager.Runnable shell calling `cycle()`
- `internal/controller/dns/dns_controller.go` — rewire chain to `LookupSources → IntraDNSDedup → UpsertDNSRecords → SourcesStatus`; add `RequeueAfter(spec.reconciliation.interval)`
- `internal/controller/dns/chain/chain_data.go` — add `SourceEndpointsByKind` and other carryover fields
- `cmd/main.go` — instantiate `SourceEndpointStore`, new `Registry`, wire into Source + DNS controllers
- `api/v1alpha2/dns_types.go` — remove `SourceConflict` condition reason if defined as a typed constant

**Files deleted (Phase 8 cleanup):**
- `internal/source/factory.go`, `internal/source/factory_test.go`
- `internal/source/<kind>/builder.go` for the 10 non-service kinds (Service's `builder.go` may not exist; check before delete)
- `internal/source/registry/registry.go` (old `Builder`-based) — superseded by `registry_v2.go` (renamed back to `registry.go` after cleanup)
- `internal/controller/source/merged_config.go`, `internal/controller/source/merged_config_test.go`
- DNS-chain handlers: `aggregate_dnsrecords.go`, `build_group_status.go`, `update_status.go` (replaced by `sources_status.go`)
- `internal/controller/source/{rebuild_sources,build_portal_index,collect_endpoints,deduplicate,reconcile_dnsrecords,delete_orphaned}.go` (if any exist as standalone files)

---

## Phase 6 — Source layer (producer + store)

### Task 6.3: Switch Resolver interface to per-object signature

**Files:**
- Modify: `internal/source/registry/resolver.go`

- [ ] **Step 6.3.1: Update the interface**

Replace the body of `internal/source/registry/resolver.go` with:

```go
package registry

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"
)

// Resolver converts a single source object pulled from the controller-runtime
// cache into zero or more external-dns Endpoints. Filtering (namespace, labels)
// is the responsibility of the read-side (DNSReconciler).
type Resolver interface {
	Type() SourceType
	// ObjectList returns a fresh empty typed list suitable for cache.List.
	ObjectList() client.ObjectList
	// ResolveObject converts a single source object into zero or more Endpoints.
	ResolveObject(ctx context.Context, obj client.Object) ([]*endpoint.Endpoint, error)
}
```

(Remove the `Filter` type entirely — it is no longer used.)

- [ ] **Step 6.3.2: Update ServiceResolver to the new signature**

In `internal/source/service/resolver.go`, replace the existing `Resolve` method with:

```go
func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		return nil, nil
	}
	host := svc.Annotations["external-dns.alpha.kubernetes.io/hostname"]
	if host == "" {
		return nil, nil
	}
	ips := loadBalancerIPs(svc)
	if len(ips) == 0 {
		return nil, nil
	}
	return []*endpoint.Endpoint{
		endpoint.NewEndpoint(strings.TrimSuffix(host, "."), endpoint.RecordTypeA, ips...),
	}, nil
}
```

Also delete the imports `k8s.io/apimachinery/pkg/labels` (no longer used) and remove the now-unused `f registry.Filter` parameter.

- [ ] **Step 6.3.3: Update the existing ServiceResolver test**

Open `internal/source/service/resolver_test.go` and adapt:
- Remove all assertions that depend on namespace / labelFilter filtering (these were Resolver-side; they are now read-side).
- Replace every `r.Resolve(ctx, []client.Object{obj}, registry.Filter{...})` call with `r.ResolveObject(ctx, obj)`.
- Drop the `TestServiceResolver_FilterByLabel` test entirely (filtering moved to the store / DNS chain).

Keep at least:

```go
func TestServiceResolver_ResolveObject_Hostname(t *testing.T) {
	r := svcsrc.NewResolver()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "echo", Namespace: "default",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/hostname": "echo.example.com"},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: "1.2.3.4"}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "echo.example.com" || eps[0].Targets[0] != "1.2.3.4" {
		t.Fatalf("unexpected endpoints: %+v", eps)
	}
}

func TestServiceResolver_ResolveObject_NoHostname(t *testing.T) {
	r := svcsrc.NewResolver()
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"}}
	eps, _ := r.ResolveObject(context.Background(), svc)
	if len(eps) != 0 {
		t.Fatalf("want no endpoints, got %d", len(eps))
	}
}

func TestServiceResolver_ResolveObject_NoLBIngress(t *testing.T) {
	r := svcsrc.NewResolver()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "x", Namespace: "y",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/hostname": "x.example.com"},
		},
	}
	eps, _ := r.ResolveObject(context.Background(), svc)
	if len(eps) != 0 {
		t.Fatalf("want no endpoints, got %d", len(eps))
	}
}
```

- [ ] **Step 6.3.4: Build + test**

```bash
go build ./internal/source/...
go test ./internal/source/service/...
```

Expected: both pass.

- [ ] **Step 6.3.5: Commit**

```bash
git add internal/source/registry/resolver.go internal/source/service/
git commit -m "refactor(source): switch Resolver interface to per-object ResolveObject"
```

---

### Task 6.4: Domain types — `EnrichedEndpoint` + `SourceEndpointStore` interfaces

**Files:**
- Create: `internal/domain/source/store.go`

- [ ] **Step 6.4.1: Write the file**

```go
// Package source defines the domain interfaces and DTOs for the
// in-memory store of cluster-wide source endpoints, consumed by the
// DNSReconciler.
package source

import (
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

// EnrichedEndpoint is an external-dns Endpoint annotated with the provenance
// of the source object that produced it. SourceLabels carries the K8s labels
// of the source object so the DNSReconciler can apply per-DNS labelFilters
// at read time without touching the apiserver.
type EnrichedEndpoint struct {
	Endpoint     *endpoint.Endpoint
	Kind         registry.SourceType
	Namespace    string // "" for cluster-scoped sources
	Name         string
	SourceLabels map[string]string
}

// SourceEndpointReader is the read-side contract for the in-memory store.
// Returned slices are snapshot copies; callers may mutate freely.
type SourceEndpointReader interface {
	// Lookup returns enriched endpoints for the given kind, filtered by
	// namespace ("" = all namespaces) and labelFilter ("" = match-all,
	// labels.Selector syntax). An invalid labelFilter returns an error.
	Lookup(kind registry.SourceType, namespace, labelFilter string) ([]EnrichedEndpoint, error)
}

// SourceEndpointWriter is the write-side contract, used by the
// SourceReconciler each polling cycle.
type SourceEndpointWriter interface {
	// ReplaceKind atomically swaps all entries for a kind.
	ReplaceKind(kind registry.SourceType, entries []EnrichedEndpoint)
	// DeleteKind removes all entries for a kind (used when the kind becomes
	// unused cluster-wide).
	DeleteKind(kind registry.SourceType)
}
```

- [ ] **Step 6.4.2: Build**

```bash
go build ./internal/domain/source/...
```

Expected: passes.

- [ ] **Step 6.4.3: Commit**

```bash
git add internal/domain/source/
git commit -m "feat(domain/source): introduce EnrichedEndpoint + store interfaces"
```

---

### Task 6.5: `SourceEndpointStore` implementation — TDD shell

**Files:**
- Create: `internal/readstore/source/store.go`
- Create: `internal/readstore/source/store_test.go`

- [ ] **Step 6.5.1: Write the shell implementation**

```go
// Package source provides the in-memory SourceEndpointStore.
// It implements both domain.source.SourceEndpointReader and SourceEndpointWriter.
package source

import (
	"sync"

	"k8s.io/apimachinery/pkg/labels"

	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// Store indexes EnrichedEndpoints by (SourceType, Namespace).
type Store struct {
	mu     sync.RWMutex
	byKind map[registry.SourceType]map[string][]domainsource.EnrichedEndpoint
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{byKind: map[registry.SourceType]map[string][]domainsource.EnrichedEndpoint{}}
}

// compile-time interface checks
var (
	_ domainsource.SourceEndpointReader = (*Store)(nil)
	_ domainsource.SourceEndpointWriter = (*Store)(nil)
)

// ReplaceKind atomically swaps all entries for a kind.
func (s *Store) ReplaceKind(kind registry.SourceType, entries []domainsource.EnrichedEndpoint) {
	byNs := map[string][]domainsource.EnrichedEndpoint{}
	for _, e := range entries {
		byNs[e.Namespace] = append(byNs[e.Namespace], e)
	}
	s.mu.Lock()
	s.byKind[kind] = byNs
	s.mu.Unlock()
}

// DeleteKind removes all entries for a kind.
func (s *Store) DeleteKind(kind registry.SourceType) {
	s.mu.Lock()
	delete(s.byKind, kind)
	s.mu.Unlock()
}

// Lookup returns enriched endpoints for kind/namespace/labelFilter.
// namespace "" matches all namespaces; labelFilter "" matches all labels.
func (s *Store) Lookup(kind registry.SourceType, namespace, labelFilter string) ([]domainsource.EnrichedEndpoint, error) {
	var sel labels.Selector = labels.Everything()
	if labelFilter != "" {
		parsed, err := labels.Parse(labelFilter)
		if err != nil {
			return nil, err
		}
		sel = parsed
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	byNs, ok := s.byKind[kind]
	if !ok {
		return nil, nil
	}

	pickBucket := func(ns string) []domainsource.EnrichedEndpoint { return byNs[ns] }
	var pool []domainsource.EnrichedEndpoint
	if namespace != "" {
		pool = pickBucket(namespace)
	} else {
		for _, bucket := range byNs {
			pool = append(pool, bucket...)
		}
	}

	out := make([]domainsource.EnrichedEndpoint, 0, len(pool))
	for _, e := range pool {
		if !sel.Matches(labels.Set(e.SourceLabels)) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
```

- [ ] **Step 6.5.2: Write failing tests**

```go
package source_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/external-dns/endpoint"

	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	rsource "github.com/golgoth31/sreportal/internal/readstore/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

const kindSvc registry.SourceType = "Service"
const kindIng registry.SourceType = "Ingress"

func ep(name string) *endpoint.Endpoint { return endpoint.NewEndpoint(name, endpoint.RecordTypeA, "1.2.3.4") }

func TestStore_ReplaceAndLookupByNamespace(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{
		{Endpoint: ep("a.example.com"), Kind: kindSvc, Namespace: "ns1", Name: "a", SourceLabels: map[string]string{"team": "x"}},
		{Endpoint: ep("b.example.com"), Kind: kindSvc, Namespace: "ns2", Name: "b", SourceLabels: map[string]string{"team": "y"}},
	})
	got, err := s.Lookup(kindSvc, "ns1", "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "a.example.com", got[0].Endpoint.DNSName)
}

func TestStore_LookupAllNamespaces(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{
		{Endpoint: ep("a.example.com"), Namespace: "ns1"},
		{Endpoint: ep("b.example.com"), Namespace: "ns2"},
	})
	got, err := s.Lookup(kindSvc, "", "")
	require.NoError(t, err)
	require.Len(t, got, 2)
}

func TestStore_LookupLabelFilter(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{
		{Endpoint: ep("a.example.com"), Namespace: "ns1", SourceLabels: map[string]string{"team": "x"}},
		{Endpoint: ep("b.example.com"), Namespace: "ns1", SourceLabels: map[string]string{"team": "y"}},
	})
	got, err := s.Lookup(kindSvc, "ns1", "team=x")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "a.example.com", got[0].Endpoint.DNSName)
}

func TestStore_LookupInvalidLabelFilter(t *testing.T) {
	s := rsource.NewStore()
	_, err := s.Lookup(kindSvc, "", "==invalid")
	assert.Error(t, err)
}

func TestStore_LookupUnknownKind(t *testing.T) {
	s := rsource.NewStore()
	got, err := s.Lookup(kindIng, "", "")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestStore_ReplaceKind_Atomicity(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{{Endpoint: ep("old.example.com"), Namespace: "ns1"}})
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{{Endpoint: ep("new.example.com"), Namespace: "ns1"}})
	got, _ := s.Lookup(kindSvc, "ns1", "")
	require.Len(t, got, 1)
	assert.Equal(t, "new.example.com", got[0].Endpoint.DNSName)
}

func TestStore_DeleteKind(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{{Endpoint: ep("a.example.com"), Namespace: "ns1"}})
	s.DeleteKind(kindSvc)
	got, _ := s.Lookup(kindSvc, "ns1", "")
	assert.Empty(t, got)
}

func TestStore_Concurrent_ReplaceAndLookup(t *testing.T) {
	s := rsource.NewStore()
	const writers, readers, iter = 4, 8, 200
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iter; i++ {
				s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{{Endpoint: ep("x.example.com"), Namespace: "ns1"}})
			}
		}()
	}
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iter; i++ {
				_, _ = s.Lookup(kindSvc, "ns1", "")
			}
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 6.5.3: Run tests**

```bash
go test -race ./internal/readstore/source/...
```

Expected: all pass.

- [ ] **Step 6.5.4: Commit**

```bash
git add internal/readstore/source/
git commit -m "feat(readstore/source): in-memory SourceEndpointStore with TDD coverage"
```

---

### Task 6.6: Resolver `Registry`

**Files:**
- Create: `internal/source/registry/registry_v2.go`
- Create: `internal/source/registry/registry_v2_test.go`

- [ ] **Step 6.6.1: Write `Registry`**

```go
package registry

// Registry aggregates all Resolvers known to the operator.
type Registry struct {
	byKind map[SourceType]Resolver
}

// NewRegistry constructs a Registry from the given Resolvers. A duplicate
// Type() panics — registration is a static, startup-only operation.
func NewRegistry(resolvers ...Resolver) *Registry {
	r := &Registry{byKind: make(map[SourceType]Resolver, len(resolvers))}
	for _, res := range resolvers {
		if _, dup := r.byKind[res.Type()]; dup {
			panic("duplicate Resolver registered for kind " + string(res.Type()))
		}
		r.byKind[res.Type()] = res
	}
	return r
}

// Get returns the Resolver for kind, or false if none is registered.
func (r *Registry) Get(kind SourceType) (Resolver, bool) {
	res, ok := r.byKind[kind]
	return res, ok
}

// Resolvers returns all registered Resolvers in deterministic order by
// SourceType string.
func (r *Registry) Resolvers() []Resolver {
	keys := make([]SourceType, 0, len(r.byKind))
	for k := range r.byKind {
		keys = append(keys, k)
	}
	// stable order
	sortSourceTypes(keys)
	out := make([]Resolver, 0, len(keys))
	for _, k := range keys {
		out = append(out, r.byKind[k])
	}
	return out
}

func sortSourceTypes(s []SourceType) {
	// simple insertion sort (N≤11)
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
```

- [ ] **Step 6.6.2: Write tests**

```go
package registry_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

type fakeResolver struct{ kind registry.SourceType }

func (f fakeResolver) Type() registry.SourceType         { return f.kind }
func (fakeResolver) ObjectList() client.ObjectList        { return nil }
func (fakeResolver) ResolveObject(context.Context, client.Object) ([]*endpoint.Endpoint, error) {
	return nil, nil
}

func TestRegistry_GetAndResolvers(t *testing.T) {
	a := fakeResolver{kind: "A"}
	b := fakeResolver{kind: "B"}
	r := registry.NewRegistry(b, a) // intentionally out of order
	got, ok := r.Get("A")
	require.True(t, ok)
	assert.Equal(t, registry.SourceType("A"), got.Type())
	_, ok = r.Get("missing")
	assert.False(t, ok)
	all := r.Resolvers()
	require.Len(t, all, 2)
	assert.Equal(t, registry.SourceType("A"), all[0].Type())
	assert.Equal(t, registry.SourceType("B"), all[1].Type())
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	_ = registry.NewRegistry(fakeResolver{kind: "X"}, fakeResolver{kind: "X"})
}
```

- [ ] **Step 6.6.3: Run tests**

```bash
go test ./internal/source/registry/...
```

Expected: all pass.

- [ ] **Step 6.6.4: Commit**

```bash
git add internal/source/registry/registry_v2.go internal/source/registry/registry_v2_test.go
git commit -m "feat(source/registry): Resolver Registry with deterministic ordering"
```

---

### Pattern for remaining Resolvers (Tasks 6.7 → 6.16)

Each `<kind>Resolver` follows the same skeleton:

```go
package <kind>

import (
	"context"
	"strings"

	<typed import>
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

const SourceType<Kind> registry.SourceType = "<kind>"

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                          { return &Resolver{} }
func (*Resolver) Type() registry.SourceType           { return SourceType<Kind> }
func (*Resolver) ObjectList() client.ObjectList       { return &<TypedList>{} }
func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	o, ok := obj.(*<Typed>)
	if !ok {
		return nil, nil
	}
	host := o.GetAnnotations()["external-dns.alpha.kubernetes.io/hostname"]
	if host == "" {
		return nil, nil
	}
	targets := extractTargets(o)
	if len(targets) == 0 {
		return nil, nil
	}
	return []*endpoint.Endpoint{
		endpoint.NewEndpoint(strings.TrimSuffix(host, "."), endpoint.RecordTypeA, targets...),
	}, nil
}
```

For each kind, **the only kind-specific code** is:
1. The typed `Object` / `ObjectList` (e.g. `*networkingv1.Ingress` / `&networkingv1.IngressList{}`).
2. The `extractTargets(o)` body — see per-task details below.
3. The corresponding test fixtures.

When the kind already has an `internal/source/<kind>/builder.go`, **read it first** to confirm which annotations and target locations are in scope, then port the minimal extraction.

---

### Task 6.7: IngressResolver

**Files:**
- Create: `internal/source/ingress/resolver.go`
- Create: `internal/source/ingress/resolver_test.go`

- [ ] **Step 6.7.1: Read the existing builder for context**

```bash
cat internal/source/ingress/builder.go
```

Note which annotations/spec fields it forwards to external-dns. The hostname annotation `external-dns.alpha.kubernetes.io/hostname` and `Spec.Rules[].Host` are both candidates; for the first iteration, support only the annotation + `Status.LoadBalancer.Ingress`.

- [ ] **Step 6.7.2: Write the resolver**

```go
package ingress

import (
	"context"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

const SourceTypeIngress registry.SourceType = "Ingress"

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                    { return &Resolver{} }
func (*Resolver) Type() registry.SourceType     { return SourceTypeIngress }
func (*Resolver) ObjectList() client.ObjectList { return &networkingv1.IngressList{} }

func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	ing, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return nil, nil
	}
	host := ing.Annotations["external-dns.alpha.kubernetes.io/hostname"]
	if host == "" {
		return nil, nil
	}
	var targets []string
	for _, lb := range ing.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			targets = append(targets, lb.IP)
		}
	}
	if len(targets) == 0 {
		return nil, nil
	}
	return []*endpoint.Endpoint{
		endpoint.NewEndpoint(strings.TrimSuffix(host, "."), endpoint.RecordTypeA, targets...),
	}, nil
}
```

- [ ] **Step 6.7.3: Write the test**

```go
package ingress_test

import (
	"context"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ingsrc "github.com/golgoth31/sreportal/internal/source/ingress"
)

func TestIngressResolver_Hostname(t *testing.T) {
	r := ingsrc.NewResolver()
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: "default",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/hostname": "web.example.com"},
		},
		Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{
			Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "10.0.0.1"}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), ing)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "web.example.com" || eps[0].Targets[0] != "10.0.0.1" {
		t.Fatalf("unexpected: %+v", eps)
	}
}

func TestIngressResolver_NoHostname(t *testing.T) {
	r := ingsrc.NewResolver()
	ing := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"}}
	eps, _ := r.ResolveObject(context.Background(), ing)
	if len(eps) != 0 {
		t.Fatalf("want 0, got %d", len(eps))
	}
}
```

- [ ] **Step 6.7.4: Run tests**

```bash
go test ./internal/source/ingress/...
```

Expected: passes.

- [ ] **Step 6.7.5: Commit**

```bash
git add internal/source/ingress/resolver.go internal/source/ingress/resolver_test.go
git commit -m "feat(source/ingress): per-object Resolver"
```

---

### Task 6.8: DNSEndpointResolver

**Files:**
- Create: `internal/source/dnsendpoint/resolver.go`
- Create: `internal/source/dnsendpoint/resolver_test.go`

- [ ] **Step 6.8.1: Read the existing builder**

```bash
cat internal/source/dnsendpoint/builder.go
```

DNSEndpoint (external-dns CRD) carries its `Endpoints` directly in `Spec.Endpoints []*endpoint.Endpoint`. No annotation logic needed — just pass them through.

- [ ] **Step 6.8.2: Write the resolver**

```go
package dnsendpoint

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

const SourceTypeDNSEndpoint registry.SourceType = "DNSEndpoint"

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                    { return &Resolver{} }
func (*Resolver) Type() registry.SourceType     { return SourceTypeDNSEndpoint }
func (*Resolver) ObjectList() client.ObjectList { return &v1alpha1.DNSEndpointList{} }

func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	de, ok := obj.(*v1alpha1.DNSEndpoint)
	if !ok || de == nil {
		return nil, nil
	}
	if len(de.Spec.Endpoints) == 0 {
		return nil, nil
	}
	out := make([]*endpoint.Endpoint, 0, len(de.Spec.Endpoints))
	for _, e := range de.Spec.Endpoints {
		if e == nil || e.DNSName == "" {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
```

- [ ] **Step 6.8.3: Write the test**

```go
package dnsendpoint_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"

	desrc "github.com/golgoth31/sreportal/internal/source/dnsendpoint"
)

func TestDNSEndpointResolver_Passthrough(t *testing.T) {
	r := desrc.NewResolver()
	de := &v1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"},
		Spec: v1alpha1.DNSEndpointSpec{Endpoints: []*endpoint.Endpoint{
			endpoint.NewEndpoint("a.example.com", endpoint.RecordTypeA, "1.2.3.4"),
		}},
	}
	eps, err := r.ResolveObject(context.Background(), de)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "a.example.com" {
		t.Fatalf("unexpected: %+v", eps)
	}
}
```

- [ ] **Step 6.8.4: Run tests + commit**

```bash
go test ./internal/source/dnsendpoint/...
git add internal/source/dnsendpoint/resolver.go internal/source/dnsendpoint/resolver_test.go
git commit -m "feat(source/dnsendpoint): per-object Resolver"
```

---

### Task 6.9: IstioGatewayResolver + IstioVirtualServiceResolver

**Files:**
- Create: `internal/source/istiogateway/resolver.go`
- Create: `internal/source/istiogateway/resolver_test.go`
- Create: `internal/source/istiovirtualservice/resolver.go`
- Create: `internal/source/istiovirtualservice/resolver_test.go`

- [ ] **Step 6.9.1: Read existing builders**

```bash
cat internal/source/istiogateway/builder.go internal/source/istiovirtualservice/builder.go
```

Istio `Gateway` carries hosts in `Spec.Servers[].Hosts`. `VirtualService` carries hosts in `Spec.Hosts`. Targets come from the `external-dns.alpha.kubernetes.io/target` annotation (NOT from `Status.LoadBalancer` — Istio resources are not LB-typed). When the target annotation is absent, return no endpoints.

- [ ] **Step 6.9.2: Write IstioGatewayResolver**

```go
package istiogateway

import (
	"context"
	"strings"

	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

const SourceTypeIstioGateway registry.SourceType = "IstioGateway"

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                    { return &Resolver{} }
func (*Resolver) Type() registry.SourceType     { return SourceTypeIstioGateway }
func (*Resolver) ObjectList() client.ObjectList { return &istionetworkingv1.GatewayList{} }

func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	gw, ok := obj.(*istionetworkingv1.Gateway)
	if !ok {
		return nil, nil
	}
	target := gw.Annotations["external-dns.alpha.kubernetes.io/target"]
	if target == "" {
		return nil, nil
	}
	hosts := map[string]struct{}{}
	for _, server := range gw.Spec.Servers {
		for _, h := range server.Hosts {
			// "namespace/host" → keep host part
			if i := strings.LastIndex(h, "/"); i >= 0 {
				h = h[i+1:]
			}
			if h == "" || h == "*" {
				continue
			}
			hosts[strings.TrimSuffix(h, ".")] = struct{}{}
		}
	}
	if len(hosts) == 0 {
		return nil, nil
	}
	out := make([]*endpoint.Endpoint, 0, len(hosts))
	for h := range hosts {
		out = append(out, endpoint.NewEndpoint(h, endpoint.RecordTypeA, target))
	}
	return out, nil
}
```

- [ ] **Step 6.9.3: Write the IstioGateway test**

```go
package istiogateway_test

import (
	"context"
	"testing"

	istionetworking "istio.io/api/networking/v1"
	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	igw "github.com/golgoth31/sreportal/internal/source/istiogateway"
)

func TestIstioGatewayResolver_HostsFromServers(t *testing.T) {
	r := igw.NewResolver()
	gw := &istionetworkingv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: "edge", Namespace: "istio-system",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/target": "1.2.3.4"},
		},
		Spec: istionetworking.Gateway{Servers: []*istionetworking.Server{
			{Hosts: []string{"namespace/foo.example.com", "*", "bar.example.com"}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), gw)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("want 2 endpoints, got %d", len(eps))
	}
}
```

- [ ] **Step 6.9.4: Write IstioVirtualServiceResolver**

```go
package istiovirtualservice

import (
	"context"
	"strings"

	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

const SourceTypeIstioVirtualService registry.SourceType = "IstioVirtualService"

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                    { return &Resolver{} }
func (*Resolver) Type() registry.SourceType     { return SourceTypeIstioVirtualService }
func (*Resolver) ObjectList() client.ObjectList { return &istionetworkingv1.VirtualServiceList{} }

func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	vs, ok := obj.(*istionetworkingv1.VirtualService)
	if !ok {
		return nil, nil
	}
	target := vs.Annotations["external-dns.alpha.kubernetes.io/target"]
	if target == "" {
		return nil, nil
	}
	out := make([]*endpoint.Endpoint, 0, len(vs.Spec.Hosts))
	for _, h := range vs.Spec.Hosts {
		if h == "" || h == "*" {
			continue
		}
		out = append(out, endpoint.NewEndpoint(strings.TrimSuffix(h, "."), endpoint.RecordTypeA, target))
	}
	return out, nil
}
```

- [ ] **Step 6.9.5: Write the VirtualService test**

```go
package istiovirtualservice_test

import (
	"context"
	"testing"

	istionetworking "istio.io/api/networking/v1"
	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ivs "github.com/golgoth31/sreportal/internal/source/istiovirtualservice"
)

func TestIstioVirtualServiceResolver_Hosts(t *testing.T) {
	r := ivs.NewResolver()
	vs := &istionetworkingv1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vs", Namespace: "x",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/target": "9.9.9.9"},
		},
		Spec: istionetworking.VirtualService{Hosts: []string{"a.example.com", "*"}},
	}
	eps, err := r.ResolveObject(context.Background(), vs)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "a.example.com" {
		t.Fatalf("unexpected: %+v", eps)
	}
}
```

- [ ] **Step 6.9.6: Run + commit**

```bash
go test ./internal/source/istiogateway/... ./internal/source/istiovirtualservice/...
git add internal/source/istiogateway/resolver.go internal/source/istiogateway/resolver_test.go internal/source/istiovirtualservice/resolver.go internal/source/istiovirtualservice/resolver_test.go
git commit -m "feat(source/istio): per-object Resolvers for Gateway + VirtualService"
```

---

### Task 6.10: Gateway API HTTPRouteResolver

**Files:**
- Create: `internal/source/gatewayhttproute/resolver.go`
- Create: `internal/source/gatewayhttproute/resolver_test.go`

- [ ] **Step 6.10.1: Read existing builder**

```bash
cat internal/source/gatewayhttproute/builder.go
```

Gateway API `HTTPRoute` carries hosts in `Spec.Hostnames []v1.Hostname`. Target comes from `external-dns.alpha.kubernetes.io/target` annotation; if absent, fall back to the parent Gateway IPs via the same logic the existing builder uses (for the first iteration, support only the annotation — leaves parent-Gateway resolution for a follow-up).

- [ ] **Step 6.10.2: Write the resolver**

```go
package gatewayhttproute

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

const SourceTypeGatewayHTTPRoute registry.SourceType = "GatewayHTTPRoute"

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                    { return &Resolver{} }
func (*Resolver) Type() registry.SourceType     { return SourceTypeGatewayHTTPRoute }
func (*Resolver) ObjectList() client.ObjectList { return &gwapiv1.HTTPRouteList{} }

func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	rt, ok := obj.(*gwapiv1.HTTPRoute)
	if !ok {
		return nil, nil
	}
	target := rt.Annotations["external-dns.alpha.kubernetes.io/target"]
	if target == "" || len(rt.Spec.Hostnames) == 0 {
		return nil, nil
	}
	out := make([]*endpoint.Endpoint, 0, len(rt.Spec.Hostnames))
	for _, h := range rt.Spec.Hostnames {
		s := strings.TrimSuffix(string(h), ".")
		if s == "" {
			continue
		}
		out = append(out, endpoint.NewEndpoint(s, endpoint.RecordTypeA, target))
	}
	return out, nil
}
```

- [ ] **Step 6.10.3: Write the test**

```go
package gatewayhttproute_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	ghr "github.com/golgoth31/sreportal/internal/source/gatewayhttproute"
)

func TestHTTPRouteResolver_Hostnames(t *testing.T) {
	r := ghr.NewResolver()
	rt := &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/target": "5.5.5.5"},
		},
		Spec: gwapiv1.HTTPRouteSpec{Hostnames: []gwapiv1.Hostname{"a.example.com", "b.example.com"}},
	}
	eps, err := r.ResolveObject(context.Background(), rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("want 2, got %d", len(eps))
	}
}
```

- [ ] **Step 6.10.4: Run + commit**

```bash
go test ./internal/source/gatewayhttproute/...
git add internal/source/gatewayhttproute/resolver.go internal/source/gatewayhttproute/resolver_test.go
git commit -m "feat(source/gatewayhttproute): per-object Resolver"
```

---

### Task 6.11: Gateway API GRPCRoute / TCPRoute / TLSRoute / UDPRoute Resolvers

Each route kind follows the **same** logic as HTTPRoute (annotation `external-dns.alpha.kubernetes.io/target` + `Spec.Hostnames`). The only diff is the typed `Object`/`ObjectList`. Implement one task per kind.

**Files (per kind):**
- Create: `internal/source/gateway<kind>route/resolver.go`
- Create: `internal/source/gateway<kind>route/resolver_test.go`

For `<kind>` ∈ `{grpc, tcp, tls, udp}`:

- [ ] **Step 6.11.<kind>.1: Read the existing builder**

```bash
cat internal/source/gateway<kind>route/builder.go
```

- [ ] **Step 6.11.<kind>.2: Write resolver**

Use the HTTPRoute template (Task 6.10.2). Replace the typed names:

| kind | SourceType constant | typed type | typed list |
|------|---------------------|------------|------------|
| grpc | `GatewayGRPCRoute` | `gwapiv1.GRPCRoute` | `gwapiv1.GRPCRouteList` |
| tcp  | `GatewayTCPRoute`  | `gwapiv1alpha2.TCPRoute` | `gwapiv1alpha2.TCPRouteList` |
| tls  | `GatewayTLSRoute`  | `gwapiv1alpha2.TLSRoute` | `gwapiv1alpha2.TLSRouteList` |
| udp  | `GatewayUDPRoute`  | `gwapiv1alpha2.UDPRoute` | `gwapiv1alpha2.UDPRouteList` |

Imports:

```go
gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"            // GRPCRoute lives here in current versions
gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2" // TCP/TLS/UDP still in v1alpha2
```

If the typed version differs in the vendored gateway-api, use whatever import the builder.go uses — that's the source of truth.

The body is identical to HTTPRoute, swapping the type assertion target.

- [ ] **Step 6.11.<kind>.3: Write tests**

Same shape as `TestHTTPRouteResolver_Hostnames` — instantiate the typed route with `Spec.Hostnames` and the target annotation, assert endpoint count and DNSName.

- [ ] **Step 6.11.<kind>.4: Run + commit**

```bash
go test ./internal/source/gateway<kind>route/...
git add internal/source/gateway<kind>route/resolver.go internal/source/gateway<kind>route/resolver_test.go
git commit -m "feat(source/gateway<kind>route): per-object Resolver"
```

---

### Task 6.12: CrossplaneScalewayRecordResolver

**Files:**
- Create: `internal/source/crossplanescalewayrecord/resolver.go`
- Create: `internal/source/crossplanescalewayrecord/resolver_test.go`

- [ ] **Step 6.12.1: Read existing builder**

```bash
cat internal/source/crossplanescalewayrecord/builder.go
```

A Crossplane Scaleway `Record` carries `spec.forProvider.name` (FQDN) and `spec.forProvider.data` (target). It is a Crossplane CR so it must be addressed as an `*unstructured.Unstructured` unless typed bindings exist. Match what `builder.go` does — if it uses typed bindings, reuse them; if it uses `Unstructured`, extract the fields via `unstructured.NestedString`.

- [ ] **Step 6.12.2: Write the resolver**

(Assuming Unstructured access — adapt to typed if `builder.go` uses typed bindings.)

```go
package crossplanescalewayrecord

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

const SourceTypeCrossplaneScalewayRecord registry.SourceType = "CrossplaneScalewayRecord"

const (
	gvrGroup    = "dns.scaleway.crossplane.io"
	gvrVersion  = "v1alpha1"
	gvrKind     = "Record"
	gvrListKind = "RecordList"
)

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                { return &Resolver{} }
func (*Resolver) Type() registry.SourceType { return SourceTypeCrossplaneScalewayRecord }
func (*Resolver) ObjectList() client.ObjectList {
	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(unstructured.GroupVersionKind{Group: gvrGroup, Version: gvrVersion, Kind: gvrListKind})
	return u
}

func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, nil
	}
	name, _, _ := unstructured.NestedString(u.Object, "spec", "forProvider", "name")
	data, _, _ := unstructured.NestedString(u.Object, "spec", "forProvider", "data")
	if name == "" || data == "" {
		return nil, nil
	}
	return []*endpoint.Endpoint{endpoint.NewEndpoint(name, endpoint.RecordTypeA, data)}, nil
}
```

- [ ] **Step 6.12.3: Write the test**

```go
package crossplanescalewayrecord_test

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	cprec "github.com/golgoth31/sreportal/internal/source/crossplanescalewayrecord"
)

func TestCrossplaneScalewayRecordResolver(t *testing.T) {
	r := cprec.NewResolver()
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{"forProvider": map[string]interface{}{
			"name": "a.example.com",
			"data": "1.2.3.4",
		}},
	}}
	eps, err := r.ResolveObject(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "a.example.com" {
		t.Fatalf("unexpected: %+v", eps)
	}
}
```

- [ ] **Step 6.12.4: Run + commit**

```bash
go test ./internal/source/crossplanescalewayrecord/...
git add internal/source/crossplanescalewayrecord/resolver.go internal/source/crossplanescalewayrecord/resolver_test.go
git commit -m "feat(source/crossplanescalewayrecord): per-object Resolver"
```

---

### Task 6.13: `SourceReconciler.cycle()` — global producer

**Files:**
- Create: `internal/controller/source/cycle.go`
- Modify: `internal/controller/source/source_controller.go`
- (Test envtest existing in `internal/controller/source/suite_test.go` will need updates in Task 6.14 onwards — for now we add a focused unit test only.)
- Create: `internal/controller/source/cycle_test.go`

- [ ] **Step 6.13.1: Write `cycle.go`**

```go
package source

import (
	"context"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// Cycle is the global producer loop body, exported for testability.
// Caller is responsible for the time.Ticker.
func Cycle(
	ctx context.Context,
	c client.Client,
	reg *registry.Registry,
	store domainsource.SourceEndpointWriter,
	prev map[registry.SourceType]bool,
) map[registry.SourceType]bool {
	logger := log.FromContext(ctx).WithName("source.cycle")

	enabled, err := computeEnabledKinds(ctx, c)
	if err != nil {
		logger.Error(err, "failed to compute enabled kinds; skipping cycle")
		return prev
	}

	for kind := range enabled {
		resolver, ok := reg.Get(kind)
		if !ok {
			logger.Info("no resolver registered", "kind", kind)
			continue
		}
		list := resolver.ObjectList()
		if err := c.List(ctx, list); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("CRD not installed; skipping kind", "kind", kind)
				continue
			}
			logger.Error(err, "list failed", "kind", kind)
			continue
		}
		items := extractItems(list)
		entries := make([]domainsource.EnrichedEndpoint, 0, len(items))
		for _, obj := range items {
			eps, rerr := resolver.ResolveObject(ctx, obj)
			if rerr != nil {
				logger.Error(rerr, "resolve failed", "kind", kind, "name", obj.GetName(), "ns", obj.GetNamespace())
				continue
			}
			for _, ep := range eps {
				entries = append(entries, domainsource.EnrichedEndpoint{
					Endpoint:     ep,
					Kind:         kind,
					Namespace:    obj.GetNamespace(),
					Name:         obj.GetName(),
					SourceLabels: obj.GetLabels(),
				})
			}
		}
		store.ReplaceKind(kind, entries)
	}

	for k := range prev {
		if !enabled[k] {
			store.DeleteKind(k)
		}
	}
	return enabled
}

// computeEnabledKinds returns the union of spec.sources.<k>.enabled across
// every non-remote v1alpha2 DNS CR in the cluster.
func computeEnabledKinds(ctx context.Context, c client.Client) (map[registry.SourceType]bool, error) {
	var dnsList sreportalv1alpha2.DNSList
	if err := c.List(ctx, &dnsList); err != nil {
		return nil, err
	}
	out := map[registry.SourceType]bool{}
	for i := range dnsList.Items {
		d := &dnsList.Items[i]
		if d.Spec.IsRemote {
			continue
		}
		for kind, enabled := range enabledKindsFromSpec(&d.Spec.Sources) {
			if enabled {
				out[kind] = true
			}
		}
	}
	return out, nil
}

// extractItems extracts client.Object slice from any *List via reflection.
// Returns []client.Object referring to addressable copies of each list item.
func extractItems(list client.ObjectList) []client.Object {
	v := reflect.ValueOf(list).Elem().FieldByName("Items")
	if !v.IsValid() || v.Kind() != reflect.Slice {
		return nil
	}
	out := make([]client.Object, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i).Addr().Interface().(client.Object)
		out = append(out, item)
	}
	return out
}
```

- [ ] **Step 6.13.2: Add `enabledKindsFromSpec`**

This translates a `SourcesSpec` to the set of `(SourceType → enabled)`. Add it next to the existing source-related helpers (e.g. in `internal/source/registry/registry_v2.go` or a new `internal/source/enabled.go`).

```go
// internal/source/enabled.go
package source

import (
	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/source/crossplanescalewayrecord"
	"github.com/golgoth31/sreportal/internal/source/dnsendpoint"
	"github.com/golgoth31/sreportal/internal/source/gatewaygrpcroute"
	"github.com/golgoth31/sreportal/internal/source/gatewayhttproute"
	"github.com/golgoth31/sreportal/internal/source/gatewaytcproute"
	"github.com/golgoth31/sreportal/internal/source/gatewaytlsroute"
	"github.com/golgoth31/sreportal/internal/source/gatewayudproute"
	"github.com/golgoth31/sreportal/internal/source/ingress"
	"github.com/golgoth31/sreportal/internal/source/istiogateway"
	"github.com/golgoth31/sreportal/internal/source/istiovirtualservice"
	"github.com/golgoth31/sreportal/internal/source/registry"
	"github.com/golgoth31/sreportal/internal/source/service"
)

// EnabledKindsFromSpec maps a DNS.spec.sources to (kind → enabled).
func EnabledKindsFromSpec(s *sreportalv1alpha2.SourcesSpec) map[registry.SourceType]bool {
	out := map[registry.SourceType]bool{}
	check := func(k registry.SourceType, src *sreportalv1alpha2.CommonSourceSpec) {
		if src != nil && src.Enabled {
			out[k] = true
		}
	}
	check(service.SourceTypeService, s.Service)
	check(ingress.SourceTypeIngress, s.Ingress)
	check(dnsendpoint.SourceTypeDNSEndpoint, s.DNSEndpoint)
	check(istiogateway.SourceTypeIstioGateway, s.IstioGateway)
	check(istiovirtualservice.SourceTypeIstioVirtualService, s.IstioVirtualService)
	check(gatewayhttproute.SourceTypeGatewayHTTPRoute, s.GatewayHTTPRoute)
	check(gatewaygrpcroute.SourceTypeGatewayGRPCRoute, s.GatewayGRPCRoute)
	check(gatewaytcproute.SourceTypeGatewayTCPRoute, s.GatewayTCPRoute)
	check(gatewaytlsroute.SourceTypeGatewayTLSRoute, s.GatewayTLSRoute)
	check(gatewayudproute.SourceTypeGatewayUDPRoute, s.GatewayUDPRoute)
	check(crossplanescalewayrecord.SourceTypeCrossplaneScalewayRecord, s.CrossplaneScalewayRecord)
	return out
}
```

Read `api/v1alpha2/dns_types.go` first to confirm the exact field names on `SourcesSpec`. If field names differ (e.g. embedded vs. pointer), adapt — keep the function signature identical.

- [ ] **Step 6.13.3: Update `cycle.go` to call the resolved name**

Replace `enabledKindsFromSpec(&d.Spec.Sources)` with the right import:

```go
import sourcepkg "github.com/golgoth31/sreportal/internal/source"
// ...
for kind, enabled := range sourcepkg.EnabledKindsFromSpec(&d.Spec.Sources) {
```

- [ ] **Step 6.13.4: Slim down `source_controller.go`**

The existing `SourceReconciler` has a heavy implementation (DNSRecord building, MergeConfigs, etc.). Replace the body with a minimal Runnable that invokes `Cycle` on each tick. Keep the same struct/constructor shape so `cmd/main.go` wiring changes are minimal.

```go
// internal/controller/source/source_controller.go (after rewrite)
package source

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// SourceReconciler is the global producer: periodically lists every enabled
// source kind cluster-wide and populates the SourceEndpointStore.
type SourceReconciler struct {
	Client   client.Client
	Registry *registry.Registry
	Store    domainsource.SourceEndpointWriter
	Interval time.Duration

	previousKinds map[registry.SourceType]bool
}

var _ manager.Runnable = (*SourceReconciler)(nil)

// Start runs the producer loop until ctx is cancelled.
func (r *SourceReconciler) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("source.reconciler")
	r.previousKinds = Cycle(ctx, r.Client, r.Registry, r.Store, r.previousKinds)
	t := time.NewTicker(r.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			r.previousKinds = Cycle(ctx, r.Client, r.Registry, r.Store, r.previousKinds)
			logger.V(2).Info("cycle complete", "kinds", len(r.previousKinds))
		}
	}
}
```

All previous logic (handlers, DNSRecord production, merged config, conflict detection) is removed. The handler files (`rebuild_sources.go`, `build_portal_index.go`, `collect_endpoints.go`, `deduplicate.go`, `reconcile_dnsrecords.go`, `delete_orphaned.go`, `reconcile_components.go`) are deleted in Task 6.17.

- [ ] **Step 6.13.5: Write a focused unit test**

```go
package source_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	srccontrol "github.com/golgoth31/sreportal/internal/controller/source"
	rsource "github.com/golgoth31/sreportal/internal/readstore/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
	svcsrc "github.com/golgoth31/sreportal/internal/source/service"
)

func TestCycle_ProducesServiceEndpoints(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "team-a"},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: "team-a",
			Sources: sreportalv1alpha2.SourcesSpec{
				Service: &sreportalv1alpha2.CommonSourceSpec{Enabled: true},
			},
		},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "echo", Namespace: "team-a",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/hostname": "echo.example.com"},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: "1.2.3.4"}},
		}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dns, svc).Build()
	reg := registry.NewRegistry(svcsrc.NewResolver())
	store := rsource.NewStore()

	prev := srccontrol.Cycle(context.Background(), c, reg, store, nil)
	require.NotEmpty(t, prev)
	got, err := store.Lookup(svcsrc.SourceTypeService, "team-a", "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "echo.example.com", got[0].Endpoint.DNSName)
}

func TestCycle_DeletesKindsNoLongerEnabled(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := registry.NewRegistry(svcsrc.NewResolver())
	store := rsource.NewStore()
	store.ReplaceKind(svcsrc.SourceTypeService, []domainsource.EnrichedEndpoint{
		{Kind: svcsrc.SourceTypeService, Namespace: "x"},
	})
	prev := map[registry.SourceType]bool{svcsrc.SourceTypeService: true}
	_ = srccontrol.Cycle(context.Background(), c, reg, store, prev)
	got, _ := store.Lookup(svcsrc.SourceTypeService, "", "")
	require.Empty(t, got)
}
```

- [ ] **Step 6.13.6: Run tests**

```bash
go test ./internal/controller/source/...
```

Expected: passes.

- [ ] **Step 6.13.7: Commit**

```bash
git add internal/controller/source/cycle.go internal/controller/source/cycle_test.go internal/controller/source/source_controller.go internal/source/enabled.go
git commit -m "feat(source): rewrite SourceReconciler as global producer-only cycle"
```

---

### Task 6.14: Wire `SourceEndpointStore` + new Registry in `cmd/main.go`

**Files:**
- Modify: `cmd/main.go`

- [ ] **Step 6.14.1: Construct store + registry**

In `cmd/main.go`, near the existing FQDNStore construction, add:

```go
sourceStore := readstoresource.NewStore()
sourceRegistry := registry.NewRegistry(
	service.NewResolver(),
	ingress.NewResolver(),
	dnsendpoint.NewResolver(),
	istiogateway.NewResolver(),
	istiovirtualservice.NewResolver(),
	gatewayhttproute.NewResolver(),
	gatewaygrpcroute.NewResolver(),
	gatewaytcproute.NewResolver(),
	gatewaytlsroute.NewResolver(),
	gatewayudproute.NewResolver(),
	crossplanescalewayrecord.NewResolver(),
)
```

Add the imports:

```go
readstoresource "github.com/golgoth31/sreportal/internal/readstore/source"
"github.com/golgoth31/sreportal/internal/source/crossplanescalewayrecord"
"github.com/golgoth31/sreportal/internal/source/dnsendpoint"
"github.com/golgoth31/sreportal/internal/source/gatewaygrpcroute"
"github.com/golgoth31/sreportal/internal/source/gatewayhttproute"
"github.com/golgoth31/sreportal/internal/source/gatewaytcproute"
"github.com/golgoth31/sreportal/internal/source/gatewaytlsroute"
"github.com/golgoth31/sreportal/internal/source/gatewayudproute"
"github.com/golgoth31/sreportal/internal/source/ingress"
"github.com/golgoth31/sreportal/internal/source/istiogateway"
"github.com/golgoth31/sreportal/internal/source/istiovirtualservice"
"github.com/golgoth31/sreportal/internal/source/registry"
"github.com/golgoth31/sreportal/internal/source/service"
```

- [ ] **Step 6.14.2: Replace the old SourceReconciler registration**

Find the `mgr.Add(&source.SourceReconciler{...})` (or similar `SetupWithManager` call) and replace with:

```go
if err := mgr.Add(&srccontrol.SourceReconciler{
	Client:   mgr.GetClient(),
	Registry: sourceRegistry,
	Store:    sourceStore,
	Interval: cfg.Reconciliation.SourceInterval, // existing config field
}); err != nil {
	setupLog.Error(err, "unable to set up SourceReconciler")
	os.Exit(1)
}
```

(Adjust `cfg.Reconciliation.SourceInterval` to whatever field name the operator config already exposes.)

- [ ] **Step 6.14.3: Build**

```bash
go build ./cmd/...
```

Expected: passes. (Compile errors from old DNSReconciler wiring or removed handlers will surface — fix them in subsequent tasks.) If the build fails because `DNSReconciler` still expects old dependencies, **stash** the old wiring temporarily by commenting it out and add `// XXX rewired in Task 7.x` markers; this is the only allowed comment marker, removed in Phase 8 cleanup. Prefer to keep the build green by reading the error and adjusting the smallest set of fields.

- [ ] **Step 6.14.4: Commit**

```bash
git add cmd/main.go
git commit -m "feat(cmd): wire SourceEndpointStore and Resolver Registry"
```

---

### Task 6.15: Extract Components production into its own Runnable

**Files:**
- Create: `internal/controller/components/runnable.go`
- Delete (Task 6.17): `internal/controller/source/reconcile_components.go`

The existing `ReconcileComponentsHandler` lives inside the Source chain. Since the Source chain is gone, the Components logic moves to its own `manager.Runnable` with the same periodic shape.

- [ ] **Step 6.15.1: Read the existing handler**

```bash
cat internal/controller/source/reconcile_components.go
```

Capture: its dependencies (`client.Client`, ComponentReader/Writer, source list), its tick interval, and its core loop.

- [ ] **Step 6.15.2: Write the Runnable**

```go
package components

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
)

// Reconciler periodically materialises Component CRs from the
// SourceEndpointStore. Decoupled from the Source producer so each can have
// its own tick.
type Reconciler struct {
	Client   client.Client
	Source   domainsource.SourceEndpointReader
	Interval time.Duration
	// add any other dependencies the existing reconcile_components.go uses
}

var _ manager.Runnable = (*Reconciler)(nil)

func (r *Reconciler) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("components.reconciler")
	t := time.NewTicker(r.Interval)
	defer t.Stop()
	r.tick(ctx, logger)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			r.tick(ctx, logger)
		}
	}
}

func (r *Reconciler) tick(ctx context.Context, logger logr.Logger) {
	// Port the body of reconcile_components.go's Handle method here,
	// substituting the input data (it used to receive collected endpoints
	// via ReconcileContext; now it reads them via r.Source.Lookup).
}
```

The exact body depends on what `reconcile_components.go` does — port verbatim, replacing the previous chain-context input with `r.Source.Lookup(...)` calls for the kinds the Components logic cares about.

- [ ] **Step 6.15.3: Wire in `cmd/main.go`**

```go
if err := mgr.Add(&components.Reconciler{
	Client:   mgr.GetClient(),
	Source:   sourceStore,
	Interval: cfg.Reconciliation.ComponentsInterval, // or fall back to SourceInterval
}); err != nil {
	setupLog.Error(err, "unable to set up Components reconciler")
	os.Exit(1)
}
```

- [ ] **Step 6.15.4: Build + manual test**

```bash
go build ./cmd/...
go test ./internal/controller/components/...
```

Expected: build passes; if tests for components exist, they pass.

- [ ] **Step 6.15.5: Commit**

```bash
git add internal/controller/components/runnable.go cmd/main.go
git commit -m "feat(components): extract production into dedicated Runnable"
```

---

### Task 6.16: Add `SourceConflict` condition removal preparation

**Files:**
- Modify: `api/v1alpha2/dns_types.go`

- [ ] **Step 6.16.1: Find and remove the `SourceConflict` reason**

```bash
grep -n "SourceConflict" api/v1alpha2/dns_types.go
```

If present as a `const`, delete it. If consumed anywhere (other than the source chain handlers being deleted in 6.17), tag the call site for deletion. We do **not** delete users of `SourceConflict` here — that happens in 6.17 alongside file deletion.

- [ ] **Step 6.16.2: Run codegen**

```bash
make helm doc
```

Expected: regenerates CRD YAML + docs without the obsolete reason. Commit any auto-generated diffs.

- [ ] **Step 6.16.3: Commit**

```bash
git add api/v1alpha2/dns_types.go config/crd/bases/ docs/api/
git commit -m "refactor(api): drop SourceConflict condition reason"
```

---

### Task 6.17: Delete old source factory, builders, merged_config, and chain handlers

**Files (deleted):**
- `internal/source/factory.go`, `internal/source/factory_test.go`
- `internal/source/registry/registry.go` (rename `registry_v2.go` → `registry.go` after old is gone)
- `internal/source/ingress/builder.go`, `internal/source/dnsendpoint/builder.go`, `internal/source/istiogateway/builder.go`, `internal/source/istiovirtualservice/builder.go`, `internal/source/gatewayhttproute/builder.go`, `internal/source/gatewaygrpcroute/builder.go`, `internal/source/gatewaytcproute/builder.go`, `internal/source/gatewaytlsroute/builder.go`, `internal/source/gatewayudproute/builder.go`, `internal/source/crossplanescalewayrecord/builder.go`
- `internal/source/service/builder.go` (if it exists)
- `internal/controller/source/merged_config.go`, `internal/controller/source/merged_config_test.go`
- `internal/controller/source/rebuild_sources.go`, `internal/controller/source/build_portal_index.go`, `internal/controller/source/collect_endpoints.go`, `internal/controller/source/deduplicate.go`, `internal/controller/source/reconcile_dnsrecords.go`, `internal/controller/source/delete_orphaned.go`, `internal/controller/source/reconcile_components.go` (logic migrated in 6.15)

- [ ] **Step 6.17.1: Verify nothing references old API surface**

```bash
grep -rn "registry.Builder\|registry.Deps\|factory.BuildTypedSources\|MergeConfigs\|SourceConflict" internal/ cmd/
```

Expected: only matches inside the files about to be deleted. If matches elsewhere, fix them first (likely small adjustments in cmd/main.go imports).

- [ ] **Step 6.17.2: Delete files**

```bash
rm internal/source/factory.go internal/source/factory_test.go
rm internal/source/registry/registry.go
rm internal/source/registry/client_generator.go internal/source/registry/gateway.go internal/source/registry/istio.go  # only if these existed for the old Builder pattern; verify with `cat` first
rm internal/source/*/builder.go
rm internal/controller/source/merged_config.go internal/controller/source/merged_config_test.go
rm internal/controller/source/rebuild_sources.go internal/controller/source/build_portal_index.go internal/controller/source/collect_endpoints.go internal/controller/source/deduplicate.go internal/controller/source/reconcile_dnsrecords.go internal/controller/source/delete_orphaned.go internal/controller/source/reconcile_components.go
```

For each file: only delete after `cat`-ing it to confirm it's the old code with no remaining unique value.

- [ ] **Step 6.17.3: Rename `registry_v2.go` → `registry.go`**

```bash
git mv internal/source/registry/registry_v2.go internal/source/registry/registry.go
git mv internal/source/registry/registry_v2_test.go internal/source/registry/registry_test.go
```

- [ ] **Step 6.17.4: Build + test**

```bash
go build ./...
go test ./internal/source/... ./internal/controller/source/...
```

Expected: passes (DNS controller may still be using the old chain — those failures are Phase 7's responsibility; if `go build ./...` breaks at DNS controller, isolate the failure to that path and continue).

- [ ] **Step 6.17.5: Commit**

```bash
git add -A internal/source/ internal/controller/source/
git commit -m "refactor(source): remove old Builder factory, merged config, and chain handlers"
```

---

## Phase 7 — DNS controller (store consumer + DNSRecord producer)

The DNS controller becomes the single consumer of `SourceEndpointStore` and the sole producer of auto `DNSRecord`s. The previous chain (`AggregateFromDNSRecords`, `BuildGroupStatus`, `UpdateStatus`) is replaced.

### Task 7.1: Update `ChainData` to carry source lookups

**Files:**
- Modify: `internal/controller/dns/chain/chain_data.go`

- [ ] **Step 7.1.1: Read existing ChainData**

```bash
cat internal/controller/dns/chain/chain_data.go
```

- [ ] **Step 7.1.2: Rewrite to focus on the new chain**

```go
package chain

import (
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

// ChainData carries per-reconcile state through the DNS chain handlers.
type ChainData struct {
	// EndpointsByKind is populated by LookupSourcesHandler. Each entry is the
	// post-filter (namespace, labelFilter) slice of enriched endpoints for
	// that kind. Iteration order follows spec.sources.priority.
	EndpointsByKind map[registry.SourceType][]*endpoint.Endpoint

	// KeptEndpointsByKind is populated by IntraDNSDedupHandler — the
	// priority-deduped subset that UpsertDNSRecordsHandler will project.
	KeptEndpointsByKind map[registry.SourceType][]*endpoint.Endpoint

	// PriorityOrder is the iteration order across kinds (from
	// spec.sources.priority + spec.sources.* enabled fallback). Provided to
	// downstream handlers so they don't recompute it.
	PriorityOrder []registry.SourceType

	// PortalDisabled is set to true when the Portal exists but has DNS
	// feature disabled — controllers use this to choose between cleanup and
	// production paths.
	PortalDisabled bool
}
```

- [ ] **Step 7.1.3: Build**

```bash
go build ./internal/controller/dns/...
```

Expected: may break old handlers (`AggregateFromDNSRecords` etc.) — they will be removed in Task 7.6.

- [ ] **Step 7.1.4: Commit**

```bash
git add internal/controller/dns/chain/chain_data.go
git commit -m "refactor(dns/chain): ChainData carries source lookups + dedup state"
```

---

### Task 7.2: `LookupSourcesHandler`

**Files:**
- Create: `internal/controller/dns/chain/lookup_sources.go`
- Create: `internal/controller/dns/chain/lookup_sources_test.go`

- [ ] **Step 7.2.1: Write the failing test**

```go
package chain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/controller/dns/chain"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	rsource "github.com/golgoth31/sreportal/internal/readstore/source"
	"github.com/golgoth31/sreportal/internal/reconciler"
	svcsrc "github.com/golgoth31/sreportal/internal/source/service"
)

func TestLookupSources_FiltersByNamespaceAndLabel(t *testing.T) {
	store := rsource.NewStore()
	store.ReplaceKind(svcsrc.SourceTypeService, []domainsource.EnrichedEndpoint{
		{Endpoint: endpoint.NewEndpoint("a.example.com", "A", "1.1.1.1"), Kind: svcsrc.SourceTypeService, Namespace: "ns1", SourceLabels: map[string]string{"team": "a"}},
		{Endpoint: endpoint.NewEndpoint("b.example.com", "A", "2.2.2.2"), Kind: svcsrc.SourceTypeService, Namespace: "ns1", SourceLabels: map[string]string{"team": "b"}},
		{Endpoint: endpoint.NewEndpoint("c.example.com", "A", "3.3.3.3"), Kind: svcsrc.SourceTypeService, Namespace: "ns2"},
	})

	h := &chain.LookupSourcesHandler{Source: store}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, chain.ChainData]{
		Resource: &sreportalv1alpha2.DNS{
			ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "ns1"},
			Spec: sreportalv1alpha2.DNSSpec{
				Defaults: sreportalv1alpha2.SourceFilterDefaults{Namespace: "ns1"},
				Sources: sreportalv1alpha2.SourcesSpec{
					Service: &sreportalv1alpha2.CommonSourceSpec{Enabled: true, LabelFilter: "team=a"},
				},
			},
		},
		Data: chain.ChainData{},
	}
	require.NoError(t, h.Handle(context.Background(), rc))
	got := rc.Data.EndpointsByKind[svcsrc.SourceTypeService]
	require.Len(t, got, 1)
	require.Equal(t, "a.example.com", got[0].DNSName)
}
```

Run:

```bash
go test ./internal/controller/dns/chain/ -run LookupSources -v
```

Expected: FAIL ("LookupSourcesHandler is undefined").

- [ ] **Step 7.2.2: Write the handler**

```go
package chain

import (
	"context"

	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/reconciler"
	sourcepkg "github.com/golgoth31/sreportal/internal/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// LookupSourcesHandler queries the SourceEndpointStore for each enabled kind
// in the DNS CR, applying the effective (namespace, labelFilter) computed
// from spec.sources.<k> ∪ spec.defaults.
type LookupSourcesHandler struct {
	Source domainsource.SourceEndpointReader
}

func (h *LookupSourcesHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha2.DNS, ChainData]) error {
	dns := rc.Resource
	enabled := sourcepkg.EnabledKindsFromSpec(&dns.Spec.Sources)
	rc.Data.EndpointsByKind = make(map[registry.SourceType][]*endpoint.Endpoint, len(enabled))
	rc.Data.PriorityOrder = orderedKinds(dns, enabled)

	for _, kind := range rc.Data.PriorityOrder {
		ns, lbl := effectiveFilter(dns, kind)
		entries, err := h.Source.Lookup(kind, ns, lbl)
		if err != nil {
			return err
		}
		eps := make([]*endpoint.Endpoint, 0, len(entries))
		for _, e := range entries {
			eps = append(eps, e.Endpoint)
		}
		rc.Data.EndpointsByKind[kind] = eps
	}
	return nil
}

// effectiveFilter returns the (namespace, labelFilter) for a given kind,
// using spec.sources.<k> first and spec.defaults as fallback.
func effectiveFilter(dns *sreportalv1alpha2.DNS, kind registry.SourceType) (string, string) {
	src := perKindSource(dns, kind)
	ns := firstNonEmpty(safeNamespace(src), dns.Spec.Defaults.Namespace)
	lbl := firstNonEmpty(safeLabelFilter(src), dns.Spec.Defaults.LabelFilter)
	return ns, lbl
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func safeNamespace(s *sreportalv1alpha2.CommonSourceSpec) string {
	if s == nil {
		return ""
	}
	return s.Namespace
}

func safeLabelFilter(s *sreportalv1alpha2.CommonSourceSpec) string {
	if s == nil {
		return ""
	}
	return s.LabelFilter
}

// perKindSource maps a SourceType to the typed pointer on SourcesSpec.
// Mirror EnabledKindsFromSpec.
func perKindSource(dns *sreportalv1alpha2.DNS, kind registry.SourceType) *sreportalv1alpha2.CommonSourceSpec {
	s := &dns.Spec.Sources
	switch kind {
	case "Service":
		return s.Service
	case "Ingress":
		return s.Ingress
	case "DNSEndpoint":
		return s.DNSEndpoint
	case "IstioGateway":
		return s.IstioGateway
	case "IstioVirtualService":
		return s.IstioVirtualService
	case "GatewayHTTPRoute":
		return s.GatewayHTTPRoute
	case "GatewayGRPCRoute":
		return s.GatewayGRPCRoute
	case "GatewayTCPRoute":
		return s.GatewayTCPRoute
	case "GatewayTLSRoute":
		return s.GatewayTLSRoute
	case "GatewayUDPRoute":
		return s.GatewayUDPRoute
	case "CrossplaneScalewayRecord":
		return s.CrossplaneScalewayRecord
	}
	return nil
}

// orderedKinds returns enabled kinds in spec.sources.priority order, with
// any leftover enabled kinds appended in deterministic SourceType order.
func orderedKinds(dns *sreportalv1alpha2.DNS, enabled map[registry.SourceType]bool) []registry.SourceType {
	out := make([]registry.SourceType, 0, len(enabled))
	seen := map[registry.SourceType]bool{}
	for _, k := range dns.Spec.Sources.Priority {
		st := registry.SourceType(k)
		if enabled[st] && !seen[st] {
			out = append(out, st)
			seen[st] = true
		}
	}
	// append the rest deterministically
	rest := make([]registry.SourceType, 0)
	for k := range enabled {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sortKinds(rest)
	out = append(out, rest...)
	return out
}

func sortKinds(s []registry.SourceType) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
```

- [ ] **Step 7.2.3: Run tests**

```bash
go test ./internal/controller/dns/chain/ -run LookupSources -v
```

Expected: PASS.

- [ ] **Step 7.2.4: Commit**

```bash
git add internal/controller/dns/chain/lookup_sources.go internal/controller/dns/chain/lookup_sources_test.go
git commit -m "feat(dns/chain): LookupSourcesHandler — reads SourceEndpointStore with effective filters"
```

---

### Task 7.3: `IntraDNSDedupHandler`

**Files:**
- Create: `internal/controller/dns/chain/intra_dns_dedup.go`
- Create: `internal/controller/dns/chain/intra_dns_dedup_test.go`

- [ ] **Step 7.3.1: Write the failing test**

```go
func TestIntraDNSDedup_FirstKindWins(t *testing.T) {
	h := &chain.IntraDNSDedupHandler{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, chain.ChainData]{
		Data: chain.ChainData{
			PriorityOrder: []registry.SourceType{"Service", "Ingress"},
			EndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				"Service": {endpoint.NewEndpoint("a.example.com", "A", "1.1.1.1")},
				"Ingress": {endpoint.NewEndpoint("a.example.com", "A", "2.2.2.2"), endpoint.NewEndpoint("b.example.com", "A", "3.3.3.3")},
			},
		},
	}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Len(t, rc.Data.KeptEndpointsByKind["Service"], 1)
	require.Equal(t, "a.example.com", rc.Data.KeptEndpointsByKind["Service"][0].DNSName)
	require.Len(t, rc.Data.KeptEndpointsByKind["Ingress"], 1)
	require.Equal(t, "b.example.com", rc.Data.KeptEndpointsByKind["Ingress"][0].DNSName)
}
```

Run:

```bash
go test ./internal/controller/dns/chain/ -run IntraDNSDedup -v
```

Expected: FAIL (undefined handler).

- [ ] **Step 7.3.2: Write the handler**

```go
package chain

import (
	"context"

	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// IntraDNSDedupHandler drops endpoints whose FQDN was already produced by a
// higher-priority kind earlier in the iteration order.
type IntraDNSDedupHandler struct{}

func (*IntraDNSDedupHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha2.DNS, ChainData]) error {
	seen := map[string]struct{}{}
	kept := make(map[registry.SourceType][]*endpoint.Endpoint, len(rc.Data.EndpointsByKind))
	for _, kind := range rc.Data.PriorityOrder {
		eps := rc.Data.EndpointsByKind[kind]
		out := make([]*endpoint.Endpoint, 0, len(eps))
		for _, e := range eps {
			if _, dup := seen[e.DNSName]; dup {
				continue
			}
			seen[e.DNSName] = struct{}{}
			out = append(out, e)
		}
		kept[kind] = out
	}
	rc.Data.KeptEndpointsByKind = kept
	return nil
}
```

- [ ] **Step 7.3.3: Run tests + commit**

```bash
go test ./internal/controller/dns/chain/ -run IntraDNSDedup -v
git add internal/controller/dns/chain/intra_dns_dedup.go internal/controller/dns/chain/intra_dns_dedup_test.go
git commit -m "feat(dns/chain): IntraDNSDedupHandler — first-kind-wins by FQDN"
```

---

### Task 7.4: `UpsertDNSRecordsHandler`

**Files:**
- Create: `internal/controller/dns/chain/upsert_dnsrecords.go`
- Create: `internal/controller/dns/chain/upsert_dnsrecords_test.go`

- [ ] **Step 7.4.1: Read prior `DNSRecord`-creation code for reference**

The previous source chain had `reconcile_dnsrecords.go` doing this work. Find the construction logic and port it:

```bash
git show HEAD~10:internal/controller/source/reconcile_dnsrecords.go 2>/dev/null || find . -name reconcile_dnsrecords.go
```

Capture: the `DNSRecord` spec shape (origin, sourceType, portalRef, ownerRef), the apply mode (server-side apply or get-or-create), and group-mapping integration.

- [ ] **Step 7.4.2: Write the handler**

```go
package chain

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// UpsertDNSRecordsHandler creates or updates one DNSRecord per kind that
// produced at least one endpoint, owned by the DNS CR. It also deletes
// auto-DNSRecords whose kind is no longer producing.
type UpsertDNSRecordsHandler struct {
	Client client.Client
}

func (h *UpsertDNSRecordsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha2.DNS, ChainData]) error {
	dns := rc.Resource
	desiredKinds := map[registry.SourceType]bool{}

	for kind, eps := range rc.Data.KeptEndpointsByKind {
		if len(eps) == 0 {
			continue
		}
		desiredKinds[kind] = true
		if err := h.upsertOne(ctx, dns, kind, eps); err != nil {
			return err
		}
	}

	// Delete auto-DNSRecords owned by this DNS whose kind is not in desiredKinds.
	var existing sreportalv1alpha2.DNSRecordList
	if err := h.Client.List(ctx, &existing, client.InNamespace(dns.Namespace)); err != nil {
		return err
	}
	for i := range existing.Items {
		dr := &existing.Items[i]
		if !ownedBy(dr, dns) || dr.Spec.Origin != sreportalv1alpha2.DNSRecordOriginAuto {
			continue
		}
		if desiredKinds[registry.SourceType(dr.Spec.SourceType)] {
			continue
		}
		if err := h.Client.Delete(ctx, dr); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (h *UpsertDNSRecordsHandler) upsertOne(ctx context.Context, dns *sreportalv1alpha2.DNS, kind registry.SourceType, eps []*endpoint.Endpoint) error {
	name := fmt.Sprintf("%s-%s", dns.Name, string(kind))
	dr := &sreportalv1alpha2.DNSRecord{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: dns.Namespace}}
	op, err := controllerutil.CreateOrUpdate(ctx, h.Client, dr, func() error {
		if dr.Spec.Origin == "" {
			dr.Spec.Origin = sreportalv1alpha2.DNSRecordOriginAuto
		}
		dr.Spec.PortalRef = dns.Spec.PortalRef
		dr.Spec.SourceType = string(kind)
		dr.Spec.Entries = endpointsToEntries(eps)
		return controllerutil.SetControllerReference(dns, dr, h.Client.Scheme())
	})
	_ = op
	return err
}

func endpointsToEntries(eps []*endpoint.Endpoint) []sreportalv1alpha2.DNSRecordEntry {
	out := make([]sreportalv1alpha2.DNSRecordEntry, 0, len(eps))
	for _, e := range eps {
		out = append(out, sreportalv1alpha2.DNSRecordEntry{
			FQDN:       e.DNSName,
			RecordType: string(e.RecordType),
			Targets:    append([]string(nil), e.Targets...),
		})
	}
	return out
}

func ownedBy(obj client.Object, owner *sreportalv1alpha2.DNS) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == owner.UID && ref.Kind == "DNS" {
			return true
		}
	}
	_ = types.UID("")
	return false
}
```

Imports to add at the top:

```go
"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
"sigs.k8s.io/external-dns/endpoint"
```

Adapt `DNSRecordEntry` field names to whatever the current v1alpha2 types use — read `api/v1alpha2/dnsrecord_types.go` to verify.

- [ ] **Step 7.4.3: Write tests**

```go
func TestUpsertDNSRecords_CreatesAndDeletes(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "ns1", UID: "u1"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "p"},
	}
	existing := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name: "d-Ingress", Namespace: "ns1",
			OwnerReferences: []metav1.OwnerReference{{UID: dns.UID, Kind: "DNS", Name: dns.Name, Controller: ptr.To(true)}},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{Origin: sreportalv1alpha2.DNSRecordOriginAuto, SourceType: "Ingress", PortalRef: "p"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dns, existing).Build()
	h := &chain.UpsertDNSRecordsHandler{Client: c}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, chain.ChainData]{
		Resource: dns,
		Data: chain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				"Service": {endpoint.NewEndpoint("a.example.com", "A", "1.1.1.1")},
			},
		},
	}
	require.NoError(t, h.Handle(context.Background(), rc))

	// Service DNSRecord created
	var created sreportalv1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: "ns1", Name: "d-Service"}, &created))
	require.Equal(t, "a.example.com", created.Spec.Entries[0].FQDN)

	// Ingress DNSRecord deleted (no longer kept)
	var gone sreportalv1alpha2.DNSRecord
	err := c.Get(context.Background(), types.NamespacedName{Namespace: "ns1", Name: "d-Ingress"}, &gone)
	require.True(t, apierrors.IsNotFound(err))
}
```

Adapt to actual v1alpha2 field names.

- [ ] **Step 7.4.4: Run tests + commit**

```bash
go test ./internal/controller/dns/chain/ -run UpsertDNSRecords -v
git add internal/controller/dns/chain/upsert_dnsrecords.go internal/controller/dns/chain/upsert_dnsrecords_test.go
git commit -m "feat(dns/chain): UpsertDNSRecordsHandler — owns auto DNSRecords per kind"
```

---

### Task 7.5: `SourcesStatusHandler`

**Files:**
- Create: `internal/controller/dns/chain/sources_status.go`
- Create: `internal/controller/dns/chain/sources_status_test.go`

- [ ] **Step 7.5.1: Write the handler**

```go
package chain

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// SourcesStatusHandler sets the SourcesReady and TargetsConflict conditions
// based on the lookup result and the FQDNStore conflict ring.
type SourcesStatusHandler struct {
	Conflicts domaindns.FQDNConflictReader
}

func (h *SourcesStatusHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha2.DNS, ChainData]) error {
	dns := rc.Resource
	setCondition(dns, metav1.Condition{
		Type:   "SourcesReady",
		Status: metav1.ConditionTrue,
		Reason: "Producing",
	})

	events := h.Conflicts.Conflicts(dns.Namespace, dns.Name)
	if len(events) > 0 {
		setCondition(dns, metav1.Condition{
			Type:    "TargetsConflict",
			Status:  metav1.ConditionTrue,
			Reason:  "FirstWriterWins",
			Message: "this DNS lost target conflicts on one or more FQDNs",
		})
	} else {
		setCondition(dns, metav1.Condition{
			Type:   "TargetsConflict",
			Status: metav1.ConditionFalse,
			Reason: "NoConflicts",
		})
	}
	return nil
}

func setCondition(dns *sreportalv1alpha2.DNS, c metav1.Condition) {
	for i := range dns.Status.Conditions {
		if dns.Status.Conditions[i].Type == c.Type {
			if dns.Status.Conditions[i].Status != c.Status {
				c.LastTransitionTime = metav1.Now()
			} else {
				c.LastTransitionTime = dns.Status.Conditions[i].LastTransitionTime
			}
			dns.Status.Conditions[i] = c
			return
		}
	}
	c.LastTransitionTime = metav1.Now()
	dns.Status.Conditions = append(dns.Status.Conditions, c)
}
```

- [ ] **Step 7.5.2: Write tests**

```go
type fakeConflicts struct{ events []domaindns.ConflictEvent }

func (f fakeConflicts) Conflicts(string, string) []domaindns.ConflictEvent { return f.events }

func TestSourcesStatus_NoConflicts(t *testing.T) {
	dns := &sreportalv1alpha2.DNS{ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "n"}}
	h := &chain.SourcesStatusHandler{Conflicts: fakeConflicts{}}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, chain.ChainData]{Resource: dns}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Equal(t, metav1.ConditionTrue, conditionStatus(dns, "SourcesReady"))
	require.Equal(t, metav1.ConditionFalse, conditionStatus(dns, "TargetsConflict"))
}

func TestSourcesStatus_WithConflicts(t *testing.T) {
	dns := &sreportalv1alpha2.DNS{ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "n"}}
	h := &chain.SourcesStatusHandler{Conflicts: fakeConflicts{events: []domaindns.ConflictEvent{{LoserRecord: "n/d"}}}}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, chain.ChainData]{Resource: dns}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Equal(t, metav1.ConditionTrue, conditionStatus(dns, "TargetsConflict"))
}

func conditionStatus(dns *sreportalv1alpha2.DNS, t string) metav1.ConditionStatus {
	for _, c := range dns.Status.Conditions {
		if c.Type == t {
			return c.Status
		}
	}
	return ""
}
```

- [ ] **Step 7.5.3: Run + commit**

```bash
go test ./internal/controller/dns/chain/ -run SourcesStatus -v
git add internal/controller/dns/chain/sources_status.go internal/controller/dns/chain/sources_status_test.go
git commit -m "feat(dns/chain): SourcesStatusHandler — SourcesReady + TargetsConflict"
```

---

### Task 7.6: Rewire `DNSReconciler` chain + watches + RequeueAfter

**Files:**
- Modify: `internal/controller/dns/dns_controller.go`
- Modify: `cmd/main.go` (only if dependency wiring changes)
- Delete: `internal/controller/dns/chain/aggregate_dnsrecords.go`, `build_group_status.go`, `update_status.go` and their tests

- [ ] **Step 7.6.1: Inspect current controller**

```bash
sed -n '1,80p' internal/controller/dns/dns_controller.go
```

Find the existing Reconcile method, the chain construction, and the watches.

- [ ] **Step 7.6.2: Replace the chain**

In `dns_controller.go`'s `Reconcile`, replace the chain construction with:

```go
chainSteps := chain.New(
    &chain.LookupSourcesHandler{Source: r.SourceReader},
    &chain.IntraDNSDedupHandler{},
    &chain.UpsertDNSRecordsHandler{Client: r.Client},
    &chain.SourcesStatusHandler{Conflicts: r.Conflicts},
)
```

(Match whatever constructor the chain framework exposes — `chain.New(...)` is illustrative.)

Add the new struct fields:

```go
type DNSReconciler struct {
    client.Client
    Scheme       *runtime.Scheme
    SourceReader domainsource.SourceEndpointReader
    Conflicts    domaindns.FQDNConflictReader
    // existing fields kept as-is
}
```

After the chain runs, set:

```go
return ctrl.Result{RequeueAfter: requeueInterval(dns)}, nil
```

where `requeueInterval` reads `dns.Spec.Reconciliation.Interval` (existing field) with a sane minimum (e.g. 30s) and a sane default (e.g. 5m).

- [ ] **Step 7.6.3: Add watches**

```go
return ctrl.NewControllerManagedBy(mgr).
    For(&sreportalv1alpha2.DNS{}).
    Watches(&sreportalv1alpha2.DNSRecord{}, handler.EnqueueRequestsFromMapFunc(r.enqueueDNSForRecord)).
    Watches(&sreportalv1alpha2.Portal{}, handler.EnqueueRequestsFromMapFunc(r.enqueueDNSForPortal)).
    Complete(r)
```

Implement `enqueueDNSForRecord` (read ownerRefs, enqueue owner) and `enqueueDNSForPortal` (list DNSs in the same namespace with `spec.portalRef == portal.name`).

- [ ] **Step 7.6.4: Delete obsolete chain files**

```bash
git rm internal/controller/dns/chain/aggregate_dnsrecords.go internal/controller/dns/chain/aggregate_dnsrecords_test.go internal/controller/dns/chain/build_group_status.go internal/controller/dns/chain/update_status.go internal/controller/dns/chain/update_status_test.go
```

- [ ] **Step 7.6.5: Wire in cmd/main.go**

Ensure `DNSReconciler` receives `sourceStore` and `fqdnStore` (as `domaindns.FQDNConflictReader`):

```go
if err := (&dnscontroller.DNSReconciler{
    Client:       mgr.GetClient(),
    Scheme:       mgr.GetScheme(),
    SourceReader: sourceStore,
    Conflicts:    fqdnStore,
}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to set up DNS controller")
    os.Exit(1)
}
```

- [ ] **Step 7.6.6: Build + test**

```bash
go build ./...
go test ./internal/controller/dns/...
```

Expected: passes.

- [ ] **Step 7.6.7: Commit**

```bash
git add internal/controller/dns/ cmd/main.go
git commit -m "feat(dns/controller): consume SourceEndpointStore + produce DNSRecords via new chain"
```

---

## Phase 8 — Proto + WebUI (unchanged from prior plan)

### Task 8.1: Proto — `Portals` repeated string

Same as previously planned. See `docs/superpowers/plans/2026-05-15-dns-multi-cr-per-portal.md` Phase 8 Task 8.1 (lines 2146+) — no design change in this revision.

### Task 8.2: WebUI — multi-portal chips

Same as previously planned. See `docs/superpowers/plans/2026-05-15-dns-multi-cr-per-portal.md` Phase 8 Task 8.2 (lines 2201+).

---

## Phase 9 — Observability + cleanup

### Task 9.1: New metrics

**Files:**
- Modify: `internal/metrics/dns.go` (or wherever existing metrics live)

- [ ] **Step 9.1.1: Add gauges and counters**

```go
var (
    sourceKindActive = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "sreportal_dns_source_kind_active",
        Help: "1 when at least one DNS CR enables this source kind.",
    }, []string{"kind"})
    sourceStoreEntries = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "sreportal_dns_source_store_entries",
        Help: "Number of EnrichedEndpoint entries currently in the SourceEndpointStore.",
    }, []string{"kind"})
)
```

Register on the controller-runtime registry in `init()`.

Update them in `SourceReconciler.Cycle()` after each `ReplaceKind` (track `len(entries)` per kind) and at the end of each cycle for `sourceKindActive` (1 per enabled kind, 0 for kinds that were active in `prev` but not in `enabled`).

- [ ] **Step 9.1.2: Add the FQDN store metrics from the original plan**

`sreportal_dns_fqdn_dedup_ratio`, `sreportal_dns_fqdn_refcount`, `sreportal_dns_targets_conflict_total` — see prior plan task 9.1.

- [ ] **Step 9.1.3: Run + commit**

```bash
go test ./internal/metrics/...
git add internal/metrics/ internal/controller/source/cycle.go
git commit -m "feat(metrics): source store + dedup + conflict counters"
```

---

### Task 9.2: Final cleanup

- [ ] **Step 9.2.1: Hunt residual v1alpha1 paths**

```bash
grep -rn "v1alpha1" internal/ cmd/ | grep -v "external-dns/apis/v1alpha1" | grep -v "_test.go"
```

Triage each match: if it's a compatibility shim no longer needed, delete; if it's the remote-DNS sync path (`sync_remote_dns.go`), validate it still serves a purpose.

- [ ] **Step 9.2.2: Verify no `XXX rewired in Task 7.x` markers remain**

```bash
grep -rn "XXX rewired" internal/ cmd/
```

Expected: no matches. If any, finish the rewire now.

- [ ] **Step 9.2.3: Regenerate auto-files + run full suite**

```bash
make helm
make doc
make test
make lint
```

Expected: all pass. Commit any auto-generated diffs.

- [ ] **Step 9.2.4: Commit**

```bash
git add -A
git commit -m "chore(cleanup): finalize DNS multi-CR + source-store refactor"
```

---

## Self-Review checklist (after writing the plan, before execution)

1. **Spec coverage**
   - §4.1 Overview → Task 6.13 (`Cycle`).
   - §4.2 Enabled kinds → Task 6.13 (`computeEnabledKinds`).
   - §4.3 `SourceEndpointStore` data model + interface → Tasks 6.4 + 6.5.
   - §4.4 `Resolver` per-object interface → Task 6.3.
   - §4.5 `SourceReconciler` cycle → Tasks 6.13 + 6.14.
   - §4.6 Removal of `MergeConfigs` + `SourceConflict` → Tasks 6.16 + 6.17.
   - §4.7 Decommissioned components → Task 6.17.
   - §7.1 Chain → Tasks 7.1 → 7.5.
   - §7.2 Watches → Task 7.6.
   - §7.3 Effective filter resolution → Task 7.2 (`effectiveFilter`).

2. **Placeholder scan**
   - The Phase 8 tasks reference the prior plan rather than spell out steps. That is acceptable because the prior plan's Phase 8 is unchanged by this revision; the subagent reads it directly. If the executor agent expects everything inline, expand at execution time.
   - Task 6.15 `tick(ctx, logger)` body is described as "port the existing handler" — this is a directed port, not a placeholder, because the executor has the existing file to copy from.

3. **Type consistency**
   - `EnrichedEndpoint`, `SourceEndpointReader/Writer`, `Registry`, `Resolver.ResolveObject` are used identically across Tasks 6.4 → 7.5.
   - `SourceType` constants per kind are introduced once (in each Resolver task) and reused by `EnabledKindsFromSpec` and `perKindSource`.
   - `ChainData.EndpointsByKind` / `KeptEndpointsByKind` / `PriorityOrder` are defined in 7.1 and consumed in 7.2/7.3/7.4 with matching shapes.

---

## After execution

When all tasks are green:
- Use `superpowers:finishing-a-development-branch` to wrap up the feature branch.
- Run `make helm`, `make doc`, `make test`, `make lint` one more time.
- Open PR against `main`; PR description should cite this plan and the spec.
