# DNS — Multiple DNS CRs per Portal, source aggregation & deduplicated read store

- **Date**: 2026-05-15
- **Status**: Design — pending implementation plan
- **Scope**: API v1alpha2 (`DNS`, `DNSRecord`), `internal/controller/dns`, `internal/controller/dnsrecords`, `internal/controller/source`, `internal/readstore/dns`, `internal/webhook/v1alpha2`, gRPC + MCP read paths
- **Out of scope**: Status pages (Component/Incident/Maintenance), NetworkPolicy, Releases, Image
- **Related**: existing v1alpha2 migration (`migrate-dns-v2/`) remains isofunctional — no automatic team-split

## 1. Motivation & goals

The current DNS model enforces a **1:1 mapping between `Portal` and `DNS`** (CEL/webhook constraint `metadata.name == spec.portalRef`). All sources of a portal live in a single CR, requiring `Source` reconciler to merge configurations from all DNS CRs cluster-wide into one effective config (`MergeConfigs`), with the loser surfacing a `SourceConflict` condition.

We want to evolve the model so that:

1. **A `Portal` may be backed by N `DNS` CRs** — typically split per-team / per-owner. Each team manages its own sources and manual entries without touching the others.
2. **Each `DNS` CR owns its `DNSRecord` CRs** (`metadata.ownerReferences` with `controller: true, blockOwnerDeletion: true`) — cascade delete is native.
3. **If the referenced `Portal` is absent**, the `DNSRecord` controller short-circuits (no error) and the read store is cleaned. No work is performed on orphaned records.
4. **Each source in a `DNS` CR is filterable by `namespace` and `labelFilter`**, with sensible defaults at the `DNS` CR level.

The complex part — which drives most of the design — is that:

- Source data must be **aggregated/deduplicated** to avoid multiplying Kubernetes API watches across N DNS CRs.
- The same FQDN may legitimately appear in **multiple `DNSRecord` CRs** (different teams, different portals).
- The read store must **deduplicate FQDNs in memory** to limit footprint while exposing a **`portal` index** so the WebUI/gRPC can fetch portal-scoped views efficiently.

## 2. Design choices summary

| Decision | Choice |
|---|---|
| Portal → DNS cardinality | 1 portal → N DNS CRs (drop `name==portalRef` constraint) |
| DNS → DNSRecord linkage | Single `metadata.ownerReferences[0]` (`controller=true`); `DNSRecord.spec.portalRef` kept (fast-out, index, kubectl UX); no `spec.dnsRef` |
| Source filter granularity | Per-source override + per-DNS-CR `spec.defaults` (`namespace`, `labelFilter`) |
| K8s API aggregation | **Single global informer per kind**, no API-side label filter; filtering done in-memory with namespace index |
| ReadStore | True in-memory dedup by `(fqdn, recordType)` + portal index + DNSRecord reverse-index + refcount |
| Source priority | **Per-DNS-CR**, applied at DNSRecord *generation* time (intra-DNS). Inter-DNS conflicts on targets → first-writer-wins + `TargetsConflict` condition on the loser |
| Migration `v1alpha1 → v1alpha2` | Isofunctional, 1 DNS per portal; team-split is a manual post-migration operator action |

## 3. CRD changes (v1alpha2)

### 3.1 `DNS`

```go
type DNSSpec struct {
    // Immuable. Portal must exist in the same namespace.
    PortalRef string `json:"portalRef"`

    // Optional. When true, the DNS is materialized by the portal controller
    // from a remote source (existing semantics, unchanged).
    IsRemote bool `json:"isRemote,omitempty"`

    // New. Default filters applied to every source unless that source overrides them.
    // Sources omit fields they don't want to override.
    Defaults SourceFilterDefaults `json:"defaults,omitempty"`

    // Existing. Each source's CommonSourceSpec carries its own Namespace+LabelFilter
    // which, when set, overrides Defaults.
    Sources SourcesSpec `json:"sources,omitempty"`

    GroupMapping   GroupMappingSpec   `json:"groupMapping,omitempty"`
    Reconciliation ReconciliationSpec `json:"reconciliation,omitempty"`
}

type SourceFilterDefaults struct {
    Namespace   string `json:"namespace,omitempty"`
    LabelFilter string `json:"labelFilter,omitempty"`
}
```

Removed:
- CEL rule `self == oldSelf` on `portalRef` remains (immuable).
- CEL/webhook constraint `metadata.name == spec.portalRef` — **dropped**.

Status (`DNSStatus`) trimmed:
- Keep `Conditions`, `LastReconcileTime`, `ObservedGeneration`, `ActiveSources`, `NextReconcileTime`.
- **Drop** the `Groups []FQDNGroupStatus` materialisation — the read store is now the single source of truth, and writing dozens of groups in a CR status is wasteful and racy.
- New condition types: `Ready`, `SourcesReady`, `TargetsConflict` (replaces `SourceConflict`).

### 3.2 `DNSRecord`

```go
type DNSRecordSpec struct {
    Origin     DNSRecordOrigin   `json:"origin"`     // auto | manual, immuable
    PortalRef  string            `json:"portalRef"`  // immuable, must equal owner DNS portalRef
    SourceType SourceType        `json:"sourceType,omitempty"` // required when origin=auto
    Entries    []DNSRecordEntry  `json:"entries,omitempty"`    // required when origin=manual
}
```

Spec shape **unchanged**. What changes is the **invariant on `metadata.ownerReferences`**, enforced by the webhook:

- Exactly **one** `ownerReference` with `apiVersion=sreportal.io/v1alpha2, kind=DNS, controller=true, blockOwnerDeletion=true`.
- The referenced `DNS` exists in the same namespace.
- `DNSRecord.spec.portalRef == owner.spec.portalRef`.
- The ownerRef is **immuable** post-creation (no re-parenting). `spec.portalRef` already immuable via CEL.

## 4. Source layer — global producer + `SourceEndpointStore`

### 4.1 Overview

The Source layer is **producer-only**. A single cluster-wide `SourceReconciler` periodically lists every enabled source kind and writes the resulting endpoints into an in-memory `SourceEndpointStore`. The `DNSReconciler` consumes this store at read time — there is no per-DNS Resolver, no Source→DNS coupling, and no merge of configs.

**Option A — provenance enriched at production time.** Each endpoint is paired with the K8s metadata of the object that produced it (kind, namespace, name, labels, annotations) **at the moment of conversion**, before it is stored. The store therefore returns self-describing `EnrichedEndpoint` values; consumers never need to round-trip to the apiserver to recover provenance for filtering, conflict diagnostics, or Component reconciliation.

```text
┌─────────────────────────────┐         ┌────────────────────────────┐
│ SourceReconciler (Runnable) │ writes  │   SourceEndpointStore       │
│  periodic ticker            ├────────►│   in-memory, indexed by     │
│  cluster-wide List + Resolve│         │   (SourceType, Namespace)   │
│  enrich with provenance     │         └────────────────┬───────────┘
└─────────────────────────────┘                          │ reads (snapshot copies)
                                                         ▼
                                              ┌──────────────────────┐
                                              │   DNSReconciler       │
                                              │   per-DNS filter +    │
                                              │   intra-DNS priority +│
                                              │   DNSRecord upsert    │
                                              └──────────────────────┘
```

- API cost: O(number of distinct kinds enabled cluster-wide), independent of the number of DNS CRs.
- Latency budget: end-to-end propagation ≤ `SourceReconciler.Interval` + `DNS.spec.reconciliation.interval`.
- No `Subscribe` from Store → DNSReconciler in v1: DNS reconcile is piloted by `RequeueAfter(spec.reconciliation.interval)` plus controller-runtime watches on `DNS`, `DNSRecord`, `Portal`. Event-driven push from the store is a deferred follow-up.

### 4.2 Enabled kinds

The set of kinds the producer polls is the **union of `spec.sources.*.enabled=true` across all non-remote `DNS` CRs** present in the cluster at tick time. There is **no global toggle in the operator ConfigMap** — DNS CRs are the single source of truth for what gets polled.

When a kind transitions from enabled to no-longer-enabled (because the last DNS CR using it was deleted or disabled it), the next cycle calls `Store.DeleteKind(kind)` to evict that kind's entries.

### 4.3 `SourceEndpointStore` — data model & interface

Domain types live in `internal/domain/source/`:

```go
// EnrichedEndpoint pairs an external-dns Endpoint with the provenance of the
// K8s object that produced it. Provenance is captured by the producer at
// conversion time (Option A), so readers never need to call the apiserver to
// recover labels/annotations for filtering or diagnostics.
type EnrichedEndpoint struct {
    Endpoint          *endpoint.Endpoint
    Kind              registry.SourceType
    Namespace         string // "" for cluster-scoped sources
    Name              string
    SourceLabels      map[string]string
    SourceAnnotations map[string]string
}

type SourceEndpointReader interface {
    // Lookup returns enriched endpoints for the given kind, filtered by
    // namespace ("" = all namespaces) and labelFilter ("" = match-all,
    // labels.Selector syntax). An invalid labelFilter returns an error.
    // Returned slices are snapshot copies; callers may mutate freely.
    Lookup(kind registry.SourceType, namespace, labelFilter string) ([]EnrichedEndpoint, error)
}

type SourceEndpointWriter interface {
    // ReplaceKind atomically swaps all entries for a kind.
    ReplaceKind(kind registry.SourceType, entries []EnrichedEndpoint)
    // DeleteKind removes all entries for a kind (used when the kind becomes
    // unused cluster-wide).
    DeleteKind(kind registry.SourceType)
}
```

Implementation in `internal/readstore/source/`:

```go
type Store struct {
    mu     sync.RWMutex
    byKind map[registry.SourceType]map[string][]EnrichedEndpoint // kind → namespace → entries
}
```

- `Lookup` is O(1) on `kind`, O(1) on `namespace` when set; iterates all namespace buckets when `namespace==""`. Label filter is applied via `labels.Parse(labelFilter).Matches(labels.Set(entry.SourceLabels))` in a linear scan of the bucket (typical bucket size: <100 entries).
- `ReplaceKind` rebuilds the `namespace→entries` sub-map under write lock; no in-place mutation of existing slices.
- `Lookup` deep-clones `SourceLabels` and `SourceAnnotations` into the returned slice so callers may mutate them safely. The `*endpoint.Endpoint` pointer is shared — it is an external-dns DTO treated as read-only by convention.
- No `Subscribe` broadcast in v1 (see §4.1).

### 4.4 `Resolver` and `Registry` — per-object conversion

Conversion logic per kind (Service → endpoints, Ingress → endpoints, …) implements a per-object signature so the `SourceReconciler` can capture provenance directly from each K8s object:

```go
// internal/source/registry/resolver.go
type Resolver interface {
    Type() SourceType
    // ObjectList returns a fresh empty typed list for client.List.
    ObjectList() client.ObjectList
    // ResolveObject converts a single source object into zero or more Endpoints.
    // The Resolver knows nothing about namespace/labelFilter — selection is
    // applied at read time by the DNSReconciler via the store's Lookup.
    ResolveObject(ctx context.Context, obj client.Object) ([]*endpoint.Endpoint, error)
}
```

`Registry` is a concrete struct in `internal/source/registry/` (not an interface) constructed at startup from the full set of compiled-in Resolvers. Duplicate `Type()` registration panics — this is a programming error caught immediately. The Registry is injected into the `SourceReconciler` (producer) and is **not** consumed by the `DNSReconciler` chain (which works directly off the store; the Resolver is not needed downstream).

```go
func NewRegistry(resolvers ...Resolver) *Registry
func (r *Registry) Get(kind SourceType) (Resolver, bool)
func (r *Registry) Resolvers() []Resolver // deterministic order
```

Eleven Resolvers are registered: Service, Ingress, DNSEndpoint, IstioGateway, IstioVirtualService, GatewayHTTPRoute, GatewayGRPCRoute, GatewayTCPRoute, GatewayTLSRoute, GatewayUDPRoute, CrossplaneScalewayRecord.

### 4.5 `SourceReconciler` cycle

`SourceReconciler` is a `manager.Runnable` driven by a `time.Ticker(Interval)`. The cycle body is exported as `Cycle` so it can be unit-tested without a manager:

```go
func Cycle(
    ctx context.Context,
    c client.Client,
    reg *registry.Registry,
    store domainsource.SourceEndpointWriter,
    prev map[registry.SourceType]bool,
) map[registry.SourceType]bool {
    enabled, err := computeEnabledKinds(ctx, c) // union over non-remote DNS CRs
    if err != nil {
        return prev // skip cycle; preserve prev so kinds aren't spuriously evicted
    }

    for kind := range enabled {
        resolver, ok := reg.Get(kind)
        if !ok { continue }

        list := resolver.ObjectList()
        if err := c.List(ctx, list); err != nil {
            if apierrors.IsNotFound(err) { continue } // CRD not installed → skip
            continue                                    // log + move on
        }

        entries := make([]domainsource.EnrichedEndpoint, 0)
        for _, obj := range extractItems(list) {
            eps, rerr := resolver.ResolveObject(ctx, obj)
            if rerr != nil { continue } // per-object failure; do not abort kind
            for _, ep := range eps {
                entries = append(entries, domainsource.EnrichedEndpoint{
                    Endpoint:          ep,
                    Kind:              kind,
                    Namespace:         obj.GetNamespace(),
                    Name:              obj.GetName(),
                    SourceLabels:      obj.GetLabels(),
                    SourceAnnotations: obj.GetAnnotations(),
                })
            }
        }
        store.ReplaceKind(kind, entries)
    }

    for k := range prev {
        if !enabled[k] { store.DeleteKind(k) }
    }
    return enabled
}
```

The Runnable wrapper:

```go
type SourceReconciler struct {
    Client   client.Client
    Registry *registry.Registry
    Store    domainsource.SourceEndpointWriter
    Interval time.Duration

    previousKinds map[registry.SourceType]bool
}

func (r *SourceReconciler) Start(ctx context.Context) error {
    r.previousKinds = Cycle(ctx, r.Client, r.Registry, r.Store, r.previousKinds)
    t := time.NewTicker(r.Interval); defer t.Stop()
    for {
        select {
        case <-ctx.Done():
            return nil
        case <-t.C:
            r.previousKinds = Cycle(ctx, r.Client, r.Registry, r.Store, r.previousKinds)
        }
    }
}
```

Notes:

- `c.List` is the manager's cached client; informers for new kinds are added lazily by controller-runtime on first `List`. Missing CRDs surface as `IsNotFound` and are skipped silently — the cycle never aborts.
- The cycle is single-goroutine and the `Store` only enforces per-kind atomicity, so concurrent cycles against the same store are unsupported (none happen by construction).
- Error handling is **log + continue**: per-kind List errors and per-object Resolve errors degrade gracefully to "stale data for that kind/object", never aborting the cycle. The old `failures.Tracker` and `SourceFailure` metric are not carried forward — the v1 surface is structured logs; richer per-kind health is part of Phase 9 observability.

### 4.6 Removal of `MergeConfigs` and inter-DNS `SourceConflict`

Per-DNS source production is gone — each DNS CR reads the global store independently. As a consequence:

- `internal/controller/source/merged_config.go` and `merged_config_test.go` are **deleted**.
- The `DNS.status.conditions[Type=SourceConflict]` condition is **removed**. Two DNS CRs declaring the same kind with different filters now produce independent `DNSRecord`s — there is no inter-DNS coupling at the Source layer to surface.
- Per-DNS `SourcesReady` is set by the `DNSReconciler` based on its own `Lookup` outcomes (see §7). The orthogonal `TargetsConflict` signal — two `DNSRecord`s contributing the same FQDN with different targets — is sourced from the `FQDNStore`'s conflict ring (see §5.4), not from the Source layer.

### 4.7 Decommissioned components

Deleted as part of the Source layer refactor:

- `internal/source/factory.go`, `internal/source/factory_test.go` — typed-source factory replaced by Resolver+Registry.
- `internal/source/<kind>/builder.go` for the 10 kinds wrapping `external-dns/source.NewXxxSource` — each kind keeps only its `resolver.go`.
- `internal/controller/source/merged_config.go` (+ test).
- `internal/controller/source/failures.go` (+ test) — `failures.Tracker` and the `SourceFailure` condition were tied to the merged-config flow and are not part of the new pipeline.
- Handlers `rebuild_sources.go`, `build_portal_index.go`, `collect_endpoints.go`, `deduplicate.go`, `reconcile_dnsrecords.go`, `delete_orphaned.go` — DNSRecord-production logic migrates into the `DNSReconciler` chain (§7).
- `ReconcileComponentsHandler` is **extracted** into its own `manager.Runnable` (orthogonal to DNS — it never belonged in the Source chain).

Kept:

- `internal/source/<kind>/resolver.go` (one per kind, eleven total).
- `internal/source/registry/` (`Resolver` interface + `Registry` struct).
- `internal/readstore/source/store.go` — the `SourceEndpointStore` implementation.
- `internal/controller/source/source_controller.go` + `cycle.go` — the Runnable and exported `Cycle` body.

## 5. ReadStore — true dedup + portal index

### 5.1 Data model

```go
type FQDNKey struct {
    Name       string // lowercased canonical
    RecordType string
}

type FQDNStore struct {
    mu             sync.RWMutex
    fqdns          map[FQDNKey]*FQDNView                       // single canonical entry
    byPortal       map[string]map[FQDNKey]struct{}             // portal → keys (WebUI index)
    byRecord       map[ResourceKey]recordContribution          // DNSRecord → keys + portalRef
    perPortalCount map[FQDNKey]map[string]int                  // key → portal → contributors
    conflicts      *conflictRing                               // bounded ring of TargetsConflict events (see §5.4)
    notifyMu       sync.Mutex
    notifyCh       chan struct{}
}

type recordContribution struct {
    keys      map[FQDNKey]struct{}
    portalRef string
}
```

`FQDNView` change:
- `PortalName string` → `Portals []string` (sorted, deduplicated).
- Add `RefCount int` (number of contributing DNSRecord). Not exposed via proto — internal GC bookkeeping.

### 5.2 Write operations (from DNSRecord controller)

**`Replace(ctx, recordKey, portalRef, fqdns []FQDNView) error`**

1. Compute `oldKeys = byRecord[recordKey].keys`, `newKeys = set(fqdns)`.
2. For `k ∈ oldKeys \ newKeys`: `fqdns[k].RefCount--`; if 0, delete from `fqdns` and from `byPortal[portalRef]`. Also remove portalRef from any retained entry that no longer has contributors for that portal (see refcount-per-portal below).
3. For each `f ∈ fqdns`:
   - If absent in `fqdns`: insert, `Portals=[portalRef]`, `RefCount=1`.
   - If present: `RefCount++`; add `portalRef` to `Portals` if absent; merge `Groups`; **keep first-writer's** `Targets`, `RecordType`, `SyncStatus`, `OriginRef`, `Description`. If the incoming view has different targets, attach a `TargetsConflict` signal (see §5.4).
4. Update `byRecord[recordKey] = {keys: newKeys, portalRef}` and `byPortal[portalRef] |= newKeys`.
5. Broadcast.

**Per-portal refcount nuance**: a `FQDNKey`'s `Portals` list is valid only as long as at least one DNSRecord with that portalRef contributes. The `perPortalCount[key][portal]` map maintains the count of contributing DNSRecords per (key, portal); when it hits 0, the portal is removed from `FQDNView.Portals` and from `byPortal[portal]`.

**`Delete(ctx, recordKey) error`**

1. Read `contrib = byRecord[recordKey]`; if missing, no-op.
2. For each `k ∈ contrib.keys`: decrement global `RefCount` and per-portal counter for `contrib.portalRef`. Remove key from `byPortal[contrib.portalRef]` if per-portal counter hits 0. Delete from `fqdns` if global `RefCount` hits 0.
3. Drop `byRecord[recordKey]`.
4. Broadcast.

### 5.3 Read operations (gRPC / MCP)

**`List(ctx, filters)`**:
- If `filters.Portal != ""`: iterate `byPortal[portal]` → lookup in `fqdns` → apply remaining filters (Namespace, Source, Search). O(|portal FQDNs|).
- Else: iterate `fqdns` fully.
- **No dedup at read time** — `deduplicateBySourcePriority` is removed (priority is applied at write time, intra-DNS).
- Sort by `(Name, RecordType)`.

**`Get(ctx, name, recordType)`**: direct map lookup.

**`Count`** / **`Subscribe`**: same shape, backed by the new store.

### 5.4 Inter-DNS conflict signal

When `Replace` detects `existing.Targets != incoming.Targets` for the same `FQDNKey`:
- Keep existing (first-writer-wins).
- Record a conflict event with `(fqdn, recordType, existingDNS, incomingDNS, portalRef)` in a bounded ring buffer exposed by the store.
- The DNS controller of the incoming DNS CR reads this buffer on its next reconcile (or via a notification) and stamps `Conditions[Type=TargetsConflict]=True` on its DNS CR. Implementation detail: ring buffer is keyed by DNS CR namespace/name so each controller fetches only its conflicts.

### 5.5 Domain interface stability

`internal/domain/dns/reader.go` (`FQDNReader.List/Get/Count/Subscribe`) and `writer.go` (`FQDNWriter.Replace/Delete`) preserve their signatures. The `Replace` writer signature gains a `portalRef` parameter:

```go
type FQDNWriter interface {
    Replace(ctx context.Context, resourceKey, portalRef string, fqdns []FQDNView) error
    Delete(ctx context.Context, resourceKey string) error
}
```

`Delete` does not take `portalRef`: the store knows it from `byRecord`.

## 6. DNSRecord controller — fast-out & projection

```text
1. Get DNSRecord
   └─ NotFound → store.Delete(recordKey); return.
2. Get owner DNS via ownerReferences
   └─ NotFound → store.Delete(recordKey); return nil. (cascade will remove the CR.)
3. Get Portal (DNS.spec.portalRef in DNS.namespace)
   ├─ NotFound → store.Delete(recordKey); log info; return nil.
   └─ Portal.Spec.Features.IsDNSEnabled() == false → store.Delete(recordKey); return nil.
4. Run chain:
   a. LoadDNSConfigHandler        (groupMapping, disableDNSCheck, sourcePriority from owner DNS)
   b. MaterialiseManualEntriesHandler
   c. SyncEndpointsHashHandler
   d. ResolveDNSHandler
   e. ProjectStoreHandler          (Replace(recordKey, portalRef, fqdns))
```

### 6.1 Watches

- `For(DNSRecord)`.
- `Watches(DNS)` → enqueue all DNSRecord owned by that DNS. Owner lookup via a new field indexer on `metadata.ownerReferences.uid` or by listing in the DNS namespace and filtering ownerRef in-memory (lower cost given typical N).
- `Watches(Portal)` → enqueue all DNSRecord whose `spec.portalRef == portal.name` in the portal's namespace (existing field indexer preserved).

### 6.2 Removal of `portalfeatures.LookupPortalFeature` error path

When the Portal is absent we now treat it as a normal `fast-out`, not an error. The current behaviour (returning the error) is changed to:

- `NotFound` → log info, store cleanup, requeue not needed.
- Real API errors (timeouts, etc.) → return error, controller-runtime retries.

## 7. DNS controller — store consumer + DNSRecord producer

The DNS controller becomes the **single consumer** of `SourceEndpointStore` and the **owner of `DNSRecord` production**. It is triggered by `RequeueAfter(spec.reconciliation.interval)` plus controller-runtime watches on `DNS`, `DNSRecord`, and `Portal` — no signal from the Source layer.

The previous chain (`AggregateFromDNSRecords`, `BuildGroupStatus`, `UpdateStatus`) materialised `DNS.status.groups`. With the read store as source of truth for FQDN aggregation, that work is dropped; status focuses on conditions only.

### 7.1 Chain

For each non-remote `DNS` CR reconcile:

```text
1. Get DNS CR
   └─ NotFound → cleanup auto-DNSRecords owned by this DNS; return.
2. Get Portal (DNS.spec.portalRef in DNS.namespace)
   ├─ NotFound → cleanup auto-DNSRecords; set SourcesReady=False(PortalMissing); return.
   └─ Portal.Spec.Features.IsDNSEnabled() == false → cleanup; set SourcesReady=False(PortalDNSDisabled); return.
3. Run chain:
   a. LookupSourcesHandler
      For each enabled source kind k (iteration order = spec.sources.priority then arbitrary):
        ns, lbl := effectiveFilter(dns, k)  // spec.sources.<k> OR spec.defaults
        eps := sourceReader.Lookup(k, ns, lbl)
        endpointsByKind[k] = eps

   b. IntraDNSDedupHandler
      seen := set[FQDN]
      for k in spec.sources.priority order:
        for ep in endpointsByKind[k]:
          if ep.DNSName ∈ seen → drop
          else → keep; seen.add(ep.DNSName)

   c. UpsertDNSRecordsHandler
      For each k with non-empty kept set:
        - Build DNSRecord (origin=auto, sourceType=k, ownerRef=DNS CR, portalRef=DNS.spec.portalRef)
        - Apply spec.groupMapping (existing logic)
        - Server-side apply (idempotent)
      Delete auto-DNSRecords owned by this DNS for kinds no longer producing.

   d. StatusHandler
      Set conditions:
        - SourcesReady=True (or False with reason if failures.Tracker reports degradation
          for any kind enabled by this DNS)
        - TargetsConflict=True if the FQDNStore conflict ring exposes events whose
          LoserRecord is owned by this DNS (read via FQDNConflictReader.Conflicts(ns,name))
      Write ActiveSources, LastReconcileTime, ObservedGeneration. No Groups.
```

### 7.2 Watches

- `For(DNS)` (primary).
- `Watches(DNSRecord)` → enqueue owner DNS CR (so manual-create/manual-delete events refresh status).
- `Watches(Portal)` → enqueue all DNS CRs whose `spec.portalRef == portal.name` in the portal's namespace.

### 7.3 Effective filter resolution

For each enabled source `k` in the DNS CR:

```go
ns  := firstNonEmpty(spec.sources.<k>.namespace, spec.defaults.namespace, "")
lbl := firstNonEmpty(spec.sources.<k>.labelFilter, spec.defaults.labelFilter, "")
```

The empty string semantically means "all" for both axes (no namespace restriction, no label restriction).

## 8. Webhooks

### 8.1 DNS webhook

- Drop `metadata.name == spec.portalRef` validation.
- Keep Portal-exists validation.
- New: validate `spec.defaults.labelFilter` parses via `labels.Parse`. Same for each source override.
- New: validate `spec.sources.priority` entries refer to sources that are `enabled: true` in the same spec.
- `spec.portalRef` and `spec.isRemote` remain immuable.

### 8.2 DNSRecord webhook (new or extended)

On **create**:
- Exactly one `ownerReferences` entry with `apiVersion=sreportal.io/v1alpha2, kind=DNS, controller=true, blockOwnerDeletion=true`.
- Owner DNS exists in the same namespace.
- `spec.portalRef == owner.spec.portalRef`.

On **update**:
- `ownerReferences[controller=true]` immuable (no re-parenting).
- `spec.origin`, `spec.portalRef`, `spec.sourceType` immuable.

Auto-created records (by the `DNSReconciler`'s `UpsertDNSRecordsHandler`) populate the ownerRef before issuing the apply call. No special admission carve-out needed — the controller owns the ownerRef like any client.

## 9. gRPC / MCP / WebUI impact

### 9.1 Proto

`FQDNView` proto:
- Change `string portal_name = N` → `repeated string portals = N` (semver-minor breaking; acceptable while v1alpha2 is pre-stable).
- No other field changes.

### 9.2 WebUI

- Links page passes `portal` filter (already does) → benefits from `byPortal` O(|portal|) lookup.
- Display: when a FQDN belongs to multiple portals, show all of them as chips. UX detail to be refined during implementation.

### 9.3 MCP

- MCP servers serialise `FQDNView` as JSON. The `portals` array is additive in semantics; existing consumers that read `portal_name` need a one-line code change (we own the few consumers).

### 9.4 Observability

New metrics on the operator-runtime registry:

- `sreportal_dns_fqdn_dedup_ratio{portal}` — gauge: (raw_writes - unique_keys) / raw_writes. Validates the dedup gain.
- `sreportal_dns_fqdn_refcount` — histogram: distribution of `RefCount` across keys.
- `sreportal_dns_targets_conflict_total{dns,portal}` — counter: increments on each first-writer-wins conflict.
- `sreportal_dns_source_kind_active{kind}` — gauge: 1 when at least one DNS CR enables that kind.

## 10. Migration & implementation order

### 10.1 Migration `v1alpha1 → v1alpha2`

`migrate-dns-v2/` keeps producing **one `DNS` CR per portal**, no automatic team-split. No tool change required by this spec.

Post-migration, the operator (human) optionally splits per team by creating additional `DNS` CRs and reassigning ownerRefs of existing manual `DNSRecord` CRs. This is documented in operations docs but is **not** in scope of this implementation plan.

### 10.2 Implementation order (each step is independently testable)

1. **CRD changes** — drop `name==portalRef`, add `spec.defaults`, trim `DNSStatus`. Regenerate helm/doc.
2. **DNSRecord webhook** — enforce ownerRef + portalRef coherence. Update suite.
3. **ReadStore refactor** — new `FQDNStore` with dedup + portal index + refcount. TDD-first.
4. **DNSRecord controller** — fast-out on Portal absence; chain projects via new `Replace(recordKey, portalRef, fqdns)`.
5. **Source layer** — `SourceEndpointStore` + per-object `Resolver` for each kind + `Registry` + slim `SourceReconciler` (cycle()). Delete `factory.go`, `merged_config.go`, old handlers and 10 builders. Remove `SourceConflict` condition.
6. **DNS controller** — new chain (`LookupSources` → `IntraDNSDedup` → `UpsertDNSRecords` → `Status`); becomes the sole producer of auto `DNSRecord`s.
7. **Proto + WebUI** — `repeated string portals`; new metrics.
8. **Cleanup** — residual `v1alpha1` paths in `sync_remote_dns.go` if applicable; verify all decommissioned files (§4.7) are removed.

### 10.3 Risk & rollback

- Read-store refactor (step 3) is the highest-risk piece (correctness of refcount/dedup). TDD with ≥80% coverage and dedicated stress tests (concurrent Replace/Delete, churn).
- Steps 1–4 are deployable independently and do not require the new source controller; the cluster keeps using the v1alpha2 1:1 model in the meantime.
- Step 5 introduces the user-visible change (N DNS CRs allowed). Until then, the existing immuable-name UX still allows only 1.
- No data migration needed in the read store (in-memory, rebuilt from CRs on start).

## 11. Open questions deferred to the plan

- Whether `Resolve` (`internal/domain/dns/resolver.go`) batching needs adjustment when `DNSRecord`s are owned by different DNS CRs but resolve concurrently.
- WebUI display of multi-portal FQDNs (chip layout, filter UX).
- Improving end-to-end freshness for the WebUI (event-driven `Subscribe` from `SourceEndpointStore`, push notifications) — explicitly deferred post-refactor.

These are implementation-plan-level decisions, not design changes.
