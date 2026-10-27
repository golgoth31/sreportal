# DNS v1alpha2 PR #274 — Review Fix-Pack Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address all critical, important, and selected suggestion-level findings from the multi-agent review of PR #274, in a sequence that minimises rework against auto-generated artefacts (CRDs, deepcopies, helm templates).

**Architecture:**
- Type-design changes land *first* so subsequent regenerations are one-shot.
- Conversion data-loss fixes use the same annotation-preservation pattern already in place for DNS Groups.
- Multi-DNS source aggregation is implemented as a deterministic per-portal merge with a fan-out source factory, replacing the "first-from-map" placeholder.
- All silent-failure fixes follow the same rule: return the error if reconcile should retry; otherwise log with context and a metric.

**Tech Stack:** Go 1.26, Kubebuilder, controller-runtime v0.23, Ginkgo v2 + Gomega, envtest, external-dns v0.20.

---

## File Structure (high-level)

**API types (changed):**
- `api/v1alpha2/common_source_spec.go` — NEW. `CommonSourceSpec` embedded type + `SourceType`, `SyncStatus`, `FQDNGroupSource` typed aliases with constants.
- `api/v1alpha2/dns_types.go` — modify each `*SourceSpec` to embed `CommonSourceSpec`; promote `Priority` to `[]SourceType`; promote `SyncStatus`/`Source` enum strings to typed aliases.
- `api/v1alpha2/dnsrecord_types.go` — promote `SourceType` field to typed alias; tighten CEL using `size(self.entries) == 0`.
- `api/v1alpha1/dns_types.go` — extend `ConvertTo`/`ConvertFrom` to preserve `Sources`/`GroupMapping`/`Reconciliation` via annotation.
- `api/v1alpha1/dnsrecord_types.go` — extend `ConvertTo`/`ConvertFrom` to preserve `Origin`/`Entries` via annotation; reject v1alpha1 PATCH on manual records via webhook.

**Conversion tests:**
- `api/v1alpha1/dns_conversion_test.go` — add round-trip + spec-preservation tests.
- `api/v1alpha1/dnsrecord_conversion_test.go` — add manual round-trip tests.

**Controllers:**
- `internal/controller/source/source_controller.go` — replace `synthesizeConfig`; add `MergedConfig()`; per-portal `reconcileOne`; surface ready/degraded; fix `MarkDegraded` NotFound; fix `EnrichEndpoints` error log; fix `Notify` drop counter.
- `internal/controller/source/dns_config_notifier.go` — add `GenerationChangedPredicate`.
- `internal/controller/source/source_controller_test.go` — multi-DNS merge tests; notify-drop test; reload lifecycle tests.
- `internal/controller/dnsrecords/dnsrecord_controller.go` — return error on read-store delete failure; return error (or synthetic requeue) on watch list failures.
- `internal/controller/dnsrecords/chain/load_dns_config.go` — short-circuit when DNS CR absent; return wrapped error.
- `internal/controller/dnsrecords/chain/project_store.go` — return store-write errors.
- `internal/controller/dnsrecords/chain/sync_hash.go` — clear hash on empty endpoints.

**Webhooks:**
- `internal/webhook/v1alpha2/dnsrecord_webhook.go` — gate ValidateUpdate so SA-less context fails closed when controllerSA is configured.
- `internal/webhook/v1alpha1/dnsrecord_webhook.go` — NEW (or extend existing). Reject v1alpha1 PATCH that would erase v1alpha2-only state.

**Migration CLI:**
- `hack/migrate-dns-v2/migrate.go` — NEW. Extract `Migrate(ctx, c, dryRun) (Summary, error)` and `slug(string) string` for unit testing.
- `hack/migrate-dns-v2/main.go` — thin CLI wrapper; non-zero exit on partial failure.
- `hack/migrate-dns-v2/migrate_test.go` — NEW.
- `hack/migrate-dns-v2/slug_test.go` — NEW.

**Envtest for CEL:**
- `test/envtest/v1alpha2/dnsrecord_cel_test.go` — NEW. Asserts CRD-side validation runs.

**Regenerated (DO NOT EDIT MANUALLY — produced by `make helm` / `make doc`):**
- `api/v1alpha2/zz_generated.deepcopy.go`
- `config/crd/bases/sreportal.io_dns.yaml`
- `config/crd/bases/sreportal.io_dnsrecords.yaml`
- `config/webhook/manifests.yaml`
- `helm/templates/dns-crd.yaml`
- `helm/templates/dnsrecord-crd.yaml`
- `helm/templates/validating-webhook-configuration.yaml`
- `docs/content/docs/api/crds.md`

---

## Phase 1 — Type design (typed aliases + CommonSourceSpec)

Land first so all subsequent code can use the new types and one regeneration covers all changes.

### Task 1.1: Create `api/v1alpha2/common_source_spec.go` with typed aliases

**Files:**
- Create: `api/v1alpha2/common_source_spec.go`

- [ ] **Step 1: Write the file**

```go
package v1alpha2

// SourceType identifies an external-dns source kind referenced by DNSRecord.spec.sourceType
// and by SourcesSpec.Priority.
// +kubebuilder:validation:Enum=service;ingress;dnsendpoint;istio-gateway;istio-virtualservice;gateway-httproute;gateway-grpcroute;gateway-tlsroute;gateway-tcproute;gateway-udproute;crossplane-scaleway-record
type SourceType string

const (
	SourceTypeService                  SourceType = "service"
	SourceTypeIngress                  SourceType = "ingress"
	SourceTypeDNSEndpoint              SourceType = "dnsendpoint"
	SourceTypeIstioGateway             SourceType = "istio-gateway"
	SourceTypeIstioVirtualService      SourceType = "istio-virtualservice"
	SourceTypeGatewayHTTPRoute         SourceType = "gateway-httproute"
	SourceTypeGatewayGRPCRoute         SourceType = "gateway-grpcroute"
	SourceTypeGatewayTLSRoute          SourceType = "gateway-tlsroute"
	SourceTypeGatewayTCPRoute          SourceType = "gateway-tcproute"
	SourceTypeGatewayUDPRoute          SourceType = "gateway-udproute"
	SourceTypeCrossplaneScalewayRecord SourceType = "crossplane-scaleway-record"
)

// SyncStatus is the DNS-side resolution status of an FQDN.
// +kubebuilder:validation:Enum=sync;notavailable;notsync;""
type SyncStatus string

const (
	SyncStatusUnknown      SyncStatus = ""
	SyncStatusSync         SyncStatus = "sync"
	SyncStatusNotAvailable SyncStatus = "notavailable"
	SyncStatusNotSync      SyncStatus = "notsync"
)

// FQDNGroupSource indicates where an FQDN group came from.
// +kubebuilder:validation:Enum=manual;external-dns;remote
type FQDNGroupSource string

const (
	FQDNGroupSourceManual      FQDNGroupSource = "manual"
	FQDNGroupSourceExternalDNS FQDNGroupSource = "external-dns"
	FQDNGroupSourceRemote      FQDNGroupSource = "remote"
)

// CommonSourceSpec carries the fields shared by every external-dns source spec.
// Embed it with json:",inline" so the CRD schema remains flat (no nesting).
type CommonSourceSpec struct {
	// +kubebuilder:default=false
	Enabled                  bool   `json:"enabled"`
	Namespace                string `json:"namespace,omitempty"`
	AnnotationFilter         string `json:"annotationFilter,omitempty"`
	LabelFilter              string `json:"labelFilter,omitempty"`
	FQDNTemplate             string `json:"fqdnTemplate,omitempty"`
	CombineFQDNAndAnnotation bool   `json:"combineFqdnAndAnnotation,omitempty"`
	IgnoreHostnameAnnotation bool   `json:"ignoreHostnameAnnotation,omitempty"`
}
```

- [ ] **Step 2: Verify it compiles**

```
go build ./api/v1alpha2/...
```
Expected: success.

- [ ] **Step 3: Commit**

```
git add api/v1alpha2/common_source_spec.go
git commit -m "feat(api/v1alpha2): add CommonSourceSpec, SourceType, SyncStatus, FQDNGroupSource typed aliases"
```

### Task 1.2: Embed `CommonSourceSpec` in each `*SourceSpec`

**Files:**
- Modify: `api/v1alpha2/dns_types.go`

- [ ] **Step 1: Rewrite `ServiceSourceSpec`**

```go
type ServiceSourceSpec struct {
	CommonSourceSpec  `json:",inline"`
	PublishInternal   bool     `json:"publishInternal,omitempty"`
	PublishHostIP     bool     `json:"publishHostIP,omitempty"`
	ServiceTypeFilter []string `json:"serviceTypeFilter,omitempty"`
}
```

- [ ] **Step 2: Rewrite `IngressSourceSpec`**

```go
type IngressSourceSpec struct {
	CommonSourceSpec  `json:",inline"`
	IngressClassNames []string `json:"ingressClassNames,omitempty"`
}
```

- [ ] **Step 3: Rewrite `DNSEndpointSourceSpec`**

```go
type DNSEndpointSourceSpec struct {
	// +kubebuilder:default=false
	Enabled     bool   `json:"enabled"`
	Namespace   string `json:"namespace,omitempty"`
	LabelFilter string `json:"labelFilter,omitempty"`
}
```
(Kept distinct — only 3 fields, no FQDNTemplate / combine / ignore semantics.)

- [ ] **Step 4: Rewrite `IstioGatewaySourceSpec`, `IstioVirtualServiceSourceSpec`**

Both reduce to:
```go
type IstioGatewaySourceSpec struct {
	CommonSourceSpec `json:",inline"`
}
type IstioVirtualServiceSourceSpec struct {
	CommonSourceSpec `json:",inline"`
}
```

- [ ] **Step 5: Rewrite `GatewayRouteSourceSpec`**

```go
type GatewayRouteSourceSpec struct {
	CommonSourceSpec   `json:",inline"`
	GatewayName        string `json:"gatewayName,omitempty"`
	GatewayNamespace   string `json:"gatewayNamespace,omitempty"`
	GatewayLabelFilter string `json:"gatewayLabelFilter,omitempty"`
}
```

- [ ] **Step 6: Rewrite `CrossplaneScalewayRecordSourceSpec`**

```go
type CrossplaneScalewayRecordSourceSpec struct {
	// +kubebuilder:default=false
	Enabled       bool   `json:"enabled"`
	Namespace     string `json:"namespace,omitempty"`
	LabelFilter   string `json:"labelFilter,omitempty"`
	ClusterScoped bool   `json:"clusterScoped,omitempty"`
}
```
(Kept distinct — no FQDN/annotation knobs.)

- [ ] **Step 7: Promote `SourcesSpec.Priority`**

In `SourcesSpec`:
```go
// +optional
Priority []SourceType `json:"priority,omitempty"`
```

- [ ] **Step 8: Promote enum strings in DNS status**

In `DNSStatus.Groups []FQDNGroupStatus`: change `FQDNGroupStatus.Source string` → `Source FQDNGroupSource`.
In `FQDNStatus.SyncStatus string` → `SyncStatus SyncStatus`.
Drop the inline `+kubebuilder:validation:Enum=...` markers — they now live on the typed alias.

- [ ] **Step 9: Build**

```
go build ./...
```
Expected: failures in callers (`source_controller.go`, chain handlers, tests). They are addressed in Task 1.4.

- [ ] **Step 10: Commit**

```
git add api/v1alpha2/dns_types.go
git commit -m "refactor(api/v1alpha2): embed CommonSourceSpec, promote SourceType/SyncStatus/FQDNGroupSource to typed aliases"
```

### Task 1.3: Promote `DNSRecord.spec.sourceType` to `SourceType`

**Files:**
- Modify: `api/v1alpha2/dnsrecord_types.go`

- [ ] **Step 1: Change the field type**

Replace:
```go
// +kubebuilder:validation:Enum=service;ingress;dnsendpoint;istio-gateway;istio-virtualservice;gateway-httproute;gateway-grpcroute;gateway-tlsroute;gateway-tcproute;gateway-udproute;crossplane-scaleway-record
// +optional
SourceType string `json:"sourceType,omitempty"`
```
With:
```go
// Required when origin=auto. Must be empty when origin=manual.
// +optional
SourceType SourceType `json:"sourceType,omitempty"`
```
(The enum tag is now on the typed alias in `common_source_spec.go`.)

- [ ] **Step 2: Tighten the XValidation rule**

Replace `(!has(self.entries) || size(self.entries) == 0)` with the simpler `size(self.entries) == 0`. The rule becomes:
```go
// +kubebuilder:validation:XValidation:rule="self.origin == 'auto' ? has(self.sourceType) && size(self.entries) == 0 : !has(self.sourceType) && size(self.entries) > 0",message="auto records require sourceType and no entries; manual records require entries and no sourceType"
```

- [ ] **Step 3: Promote `EndpointStatus.SyncStatus`**

`SyncStatus string` → `SyncStatus SyncStatus`. Remove the inline `+kubebuilder:validation:Enum=...` marker (lives on the alias now).

- [ ] **Step 4: Commit**

```
git add api/v1alpha2/dnsrecord_types.go
git commit -m "refactor(api/v1alpha2/dnsrecord): use typed SourceType/SyncStatus, tighten CEL"
```

### Task 1.4: Update all consumers to new typed values

This is a mechanical sweep. Run `go build ./...` and fix one error at a time. The known sites are:

**Files:**
- Modify: `internal/controller/source/source_controller.go` (`MarkDegraded` uses `sreportalv1alpha1.DNSRecord` — leave that switch to Phase 2 / Phase 3; but `EnabledSourceTypes` returns `registry.SourceType`, not `v1alpha2.SourceType` — keep separate).
- Modify: `internal/adapter/endpoint.go` (any FQDN status assembly).
- Modify: `internal/controller/dns/chain/build_group_status.go` (sets `Source`/`SyncStatus` strings).
- Modify: `internal/controller/dns/chain/aggregate_dnsrecords.go`.
- Modify: `internal/controller/dnsrecords/chain/materialise_manual.go`, `project_store.go`, `resolve_dns.go`, `sync_hash.go`.
- Modify: `internal/webhook/v1alpha2/dnsrecord_webhook.go` (`record.Spec.SourceType` comparisons).
- Modify: `hack/migrate-dns-v2/main.go` (`DNSRecordEntry` construction is unaffected; `SourceType` not set on manual).
- Modify: every `*_test.go` that constructs `ServiceSourceSpec{Enabled: true, ...}` inline — these must become `ServiceSourceSpec{CommonSourceSpec: v1alpha2.CommonSourceSpec{Enabled: true, ...}}`.

- [ ] **Step 1: Run build and capture error list**

```
go build ./... 2>&1 | tee /tmp/build-errors.txt
```

- [ ] **Step 2: For each error, change the call site**

Patterns:
- `string(...) → SourceType` casts: replace inline literals (`"service"`) with `v1alpha2.SourceTypeService`.
- `*SourceSpec{Enabled: true, Namespace: "ns"}` → `*SourceSpec{CommonSourceSpec: v1alpha2.CommonSourceSpec{Enabled: true, Namespace: "ns"}}`.
- `FQDNGroupStatus{Source: "external-dns"}` → `FQDNGroupStatus{Source: v1alpha2.FQDNGroupSourceExternalDNS}`.
- `EndpointStatus.SyncStatus = "notavailable"` → `SyncStatus: v1alpha2.SyncStatusNotAvailable`.

- [ ] **Step 3: Build clean**

```
go build ./...
```
Expected: success.

- [ ] **Step 4: Run unit tests (will likely still fail in conversion package — fixed in Phase 2)**

```
go test ./api/... ./internal/controller/... ./internal/webhook/... ./hack/...
```
Track failures; expect only conversion-related test diffs. Conversion test rewrites land in Phase 2.

- [ ] **Step 5: Commit**

```
git add -u
git commit -m "refactor: adopt v1alpha2 typed aliases across controllers, webhooks, tests"
```

### Task 1.5: Regenerate deepcopies + CRDs + helm + docs

**Files (regenerated — DO NOT EDIT MANUALLY):**
- `api/v1alpha2/zz_generated.deepcopy.go`
- `config/crd/bases/sreportal.io_dns.yaml`, `_dnsrecords.yaml`
- `helm/templates/dns-crd.yaml`, `dnsrecord-crd.yaml`
- `docs/content/docs/api/crds.md`

- [ ] **Step 1: Generate**

```
make helm
make doc
```

- [ ] **Step 2: Inspect diff**

```
git status
git diff --stat
```
Expected: only auto-generated files changed.

- [ ] **Step 3: Verify CRD schema is flat (embedding worked)**

```
grep -A2 "annotationFilter" config/crd/bases/sreportal.io_dns.yaml | head -20
```
Each source schema should still list `enabled`, `namespace`, `annotationFilter`, … inline — *not* nested under a `commonSourceSpec` sub-object. If nested, the embedding lost `json:",inline"`.

- [ ] **Step 4: Run unit tests + lint**

```
make test
make lint
```
Expected: all green.

- [ ] **Step 5: Commit**

```
git add api/v1alpha2/zz_generated.deepcopy.go config/crd helm/templates docs/content/docs/api/crds.md
git commit -m "chore: regenerate CRDs, helm templates, and API docs for typed aliases"
```

---

## Phase 2 — Conversion data-loss fixes

Goal: v1alpha1 ↔ v1alpha2 round-trip is **lossless** for both DNS and DNSRecord.

### Task 2.1: Preserve DNS `Spec.Sources` / `GroupMapping` / `Reconciliation` via annotation

**Files:**
- Modify: `api/v1alpha1/dns_types.go`
- Test: `api/v1alpha1/dns_conversion_test.go`

- [ ] **Step 1: Add the failing round-trip test first**

Append to `api/v1alpha1/dns_conversion_test.go`:
```go
func TestDNSRoundTrip_PreservesV1Alpha2OnlySpec(t *testing.T) {
	g := NewWithT(t)

	src := &v1alpha2.DNS{
		Spec: v1alpha2.DNSSpec{
			PortalRef: "main",
			Sources: v1alpha2.SourcesSpec{
				Service: &v1alpha2.ServiceSourceSpec{
					CommonSourceSpec: v1alpha2.CommonSourceSpec{Enabled: true, Namespace: "prod"},
				},
				Priority: []v1alpha2.SourceType{v1alpha2.SourceTypeService},
			},
			GroupMapping: v1alpha2.GroupMappingSpec{DefaultGroup: "Apps", LabelKey: "team"},
			Reconciliation: v1alpha2.ReconciliationSpec{
				Interval:        metav1.Duration{Duration: 90 * time.Second},
				DisableDNSCheck: true,
			},
		},
	}

	var spoke v1alpha1.DNS
	g.Expect(spoke.ConvertFrom(src)).To(Succeed())

	var hub v1alpha2.DNS
	g.Expect(spoke.ConvertTo(&hub)).To(Succeed())

	g.Expect(hub.Spec.Sources.Service).NotTo(BeNil())
	g.Expect(hub.Spec.Sources.Service.Enabled).To(BeTrue())
	g.Expect(hub.Spec.Sources.Service.Namespace).To(Equal("prod"))
	g.Expect(hub.Spec.Sources.Priority).To(ConsistOf(v1alpha2.SourceTypeService))
	g.Expect(hub.Spec.GroupMapping.DefaultGroup).To(Equal("Apps"))
	g.Expect(hub.Spec.Reconciliation.Interval.Duration).To(Equal(90 * time.Second))
	g.Expect(hub.Spec.Reconciliation.DisableDNSCheck).To(BeTrue())
	// Migration annotation is internal and must not leak back to v1alpha2 storage
	g.Expect(hub.Annotations).NotTo(HaveKey("sreportal.io/v1alpha2-spec"))
}
```

- [ ] **Step 2: Run it — expect FAIL**

```
go test ./api/v1alpha1/... -run TestDNSRoundTrip_PreservesV1Alpha2OnlySpec
```

- [ ] **Step 3: Define the spec-preservation annotation constant**

In `api/v1alpha1/dns_types.go`, add:
```go
const annotationV1Alpha2DNSSpec = "sreportal.io/v1alpha2-spec"

// preservedDNSSpec holds v1alpha2-only DNSSpec fields that have no v1alpha1
// representation. It is JSON-encoded into annotationV1Alpha2DNSSpec on ConvertFrom
// and restored on ConvertTo.
type preservedDNSSpec struct {
	Sources        v1alpha2.SourcesSpec       `json:"sources,omitempty"`
	GroupMapping   v1alpha2.GroupMappingSpec  `json:"groupMapping,omitempty"`
	Reconciliation v1alpha2.ReconciliationSpec `json:"reconciliation,omitempty"`
}
```

- [ ] **Step 4: Update `ConvertFrom` to stash v1alpha2-only spec**

In `DNS.ConvertFrom`, after copying `PortalRef`/`IsRemote`, *before* returning, marshal and stash:
```go
preserved := preservedDNSSpec{
	Sources:        src.Spec.Sources,
	GroupMapping:   src.Spec.GroupMapping,
	Reconciliation: src.Spec.Reconciliation,
}
raw, err := json.Marshal(preserved)
if err != nil {
	return fmt.Errorf("marshal v1alpha2-only DNSSpec for %s/%s: %w", src.Namespace, src.Name, err)
}
if dst.Annotations == nil {
	dst.Annotations = make(map[string]string, 1)
}
dst.Annotations[annotationV1Alpha2DNSSpec] = string(raw)
```

- [ ] **Step 5: Update `ConvertTo` to restore and strip**

In `DNS.ConvertTo`, after copying `ObjectMeta`/`PortalRef`/`IsRemote`, before the groups-annotation block:
```go
if raw, ok := src.Annotations[annotationV1Alpha2DNSSpec]; ok && raw != "" {
	var p preservedDNSSpec
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return fmt.Errorf("unmarshal v1alpha2 DNSSpec annotation on %s/%s: %w", src.Namespace, src.Name, err)
	}
	dst.Spec.Sources = p.Sources
	dst.Spec.GroupMapping = p.GroupMapping
	dst.Spec.Reconciliation = p.Reconciliation
	// Strip migration annotation from hub copy
	delete(dst.Annotations, annotationV1Alpha2DNSSpec)
}
```

Replace the existing TODO comment ("sources/groupMapping/reconciliation left empty — migration tool fills them") with: `// v1alpha2-only spec is restored above from annotationV1Alpha2DNSSpec; fresh v1alpha1 objects leave them zero (migration CLI fills them).`

- [ ] **Step 6: Run the new test — expect PASS**

```
go test ./api/v1alpha1/... -run TestDNSRoundTrip_PreservesV1Alpha2OnlySpec -v
```

- [ ] **Step 7: Run full conversion test suite**

```
go test ./api/v1alpha1/...
```
Expected: all green. If `TestDNSConvertFrom_NoAnnotation` now sets the new annotation, update it to assert that the annotation is *only* present when source spec is non-zero (or simply unconditional).

- [ ] **Step 8: Commit**

```
git add api/v1alpha1/dns_types.go api/v1alpha1/dns_conversion_test.go
git commit -m "fix(api/v1alpha1): preserve v1alpha2-only DNSSpec via annotation on round-trip"
```

### Task 2.2: Preserve DNSRecord `Origin` / `Entries` via annotation

**Files:**
- Modify: `api/v1alpha1/dnsrecord_types.go`
- Test: `api/v1alpha1/dnsrecord_conversion_test.go`

- [ ] **Step 1: Add the failing test**

```go
func TestDNSRecordRoundTrip_PreservesManualEntries(t *testing.T) {
	g := NewWithT(t)

	src := &v1alpha2.DNSRecord{
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: "main",
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "manual.example.com", Group: "Apps", RecordType: "A", Targets: []string{"10.0.0.1"}},
			},
		},
	}

	var spoke v1alpha1.DNSRecord
	g.Expect(spoke.ConvertFrom(src)).To(Succeed())
	g.Expect(spoke.Annotations).To(HaveKey("sreportal.io/v1alpha2-dnsrecord-spec"))

	var hub v1alpha2.DNSRecord
	g.Expect(spoke.ConvertTo(&hub)).To(Succeed())
	g.Expect(hub.Spec.Origin).To(Equal(v1alpha2.DNSRecordOriginManual))
	g.Expect(hub.Spec.Entries).To(HaveLen(1))
	g.Expect(hub.Spec.Entries[0].FQDN).To(Equal("manual.example.com"))
	g.Expect(hub.Annotations).NotTo(HaveKey("sreportal.io/v1alpha2-dnsrecord-spec"))
}

func TestDNSRecordRoundTrip_AutoUnchanged(t *testing.T) {
	g := NewWithT(t)
	src := &v1alpha2.DNSRecord{Spec: v1alpha2.DNSRecordSpec{
		Origin: v1alpha2.DNSRecordOriginAuto, PortalRef: "main", SourceType: v1alpha2.SourceTypeService,
	}}

	var spoke v1alpha1.DNSRecord
	g.Expect(spoke.ConvertFrom(src)).To(Succeed())
	var hub v1alpha2.DNSRecord
	g.Expect(spoke.ConvertTo(&hub)).To(Succeed())
	g.Expect(hub.Spec.Origin).To(Equal(v1alpha2.DNSRecordOriginAuto))
	g.Expect(hub.Spec.SourceType).To(Equal(v1alpha2.SourceTypeService))
	g.Expect(hub.Spec.Entries).To(BeEmpty())
}
```

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement annotation stash/restore**

In `api/v1alpha1/dnsrecord_types.go`:
```go
const annotationV1Alpha2DNSRecordSpec = "sreportal.io/v1alpha2-dnsrecord-spec"

type preservedDNSRecordSpec struct {
	Origin  v1alpha2.DNSRecordOrigin `json:"origin"`
	Entries []v1alpha2.DNSRecordEntry `json:"entries,omitempty"`
}
```

Update `ConvertFrom` (v1alpha2 → v1alpha1) — after copying `PortalRef`/`SourceType`:
```go
preserved := preservedDNSRecordSpec{Origin: src.Spec.Origin, Entries: src.Spec.Entries}
raw, err := json.Marshal(preserved)
if err != nil {
	return fmt.Errorf("marshal v1alpha2-only DNSRecordSpec for %s/%s: %w", src.Namespace, src.Name, err)
}
if dst.Annotations == nil {
	dst.Annotations = make(map[string]string, 1)
}
dst.Annotations[annotationV1Alpha2DNSRecordSpec] = string(raw)
// v1alpha1 has no SourceType for manual records; force empty so the
// conversion does not produce an invalid v1alpha1 object.
if src.Spec.Origin == v1alpha2.DNSRecordOriginManual {
	dst.Spec.SourceType = ""
}
```

Update `ConvertTo` (v1alpha1 → v1alpha2) — after copying `PortalRef`/`SourceType`:
```go
dst.Spec.Origin = v1alpha2.DNSRecordOriginAuto // default for legacy v1alpha1
if raw, ok := src.Annotations[annotationV1Alpha2DNSRecordSpec]; ok && raw != "" {
	var p preservedDNSRecordSpec
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return fmt.Errorf("unmarshal v1alpha2 DNSRecordSpec annotation on %s/%s: %w", src.Namespace, src.Name, err)
	}
	dst.Spec.Origin = p.Origin
	dst.Spec.Entries = p.Entries
	if p.Origin == v1alpha2.DNSRecordOriginManual {
		dst.Spec.SourceType = "" // mutex with entries
	}
	delete(dst.Annotations, annotationV1Alpha2DNSRecordSpec)
}
```

Add `import "fmt"` if not already imported. Note: v1alpha1 `SourceType` is `string`, so the empty string assignment is fine.

- [ ] **Step 4: Run new tests — expect PASS**

- [ ] **Step 5: Run full suite**

```
go test ./api/v1alpha1/...
```

- [ ] **Step 6: Commit**

```
git add api/v1alpha1/dnsrecord_types.go api/v1alpha1/dnsrecord_conversion_test.go
git commit -m "fix(api/v1alpha1): preserve DNSRecord origin and entries via annotation on round-trip"
```

### Task 2.3: Defensive webhook — warn on v1alpha1 PATCH of manual DNSRecord

**Files:**
- Create: `internal/webhook/v1alpha1/dnsrecord_webhook.go`
- Test: `internal/webhook/v1alpha1/dnsrecord_webhook_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestV1Alpha1DNSRecord_ManualPatchEmitsWarning(t *testing.T) {
	g := NewWithT(t)
	w := &DNSRecordWebhook{}
	old := &v1alpha1.DNSRecord{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{"sreportal.io/v1alpha2-dnsrecord-spec": `{"origin":"manual","entries":[{"fqdn":"a.example.com"}]}`},
	}}
	new := old.DeepCopy()
	new.Spec.SourceType = "service" // user editing through v1alpha1 — about to clobber manual entries

	warnings, err := w.ValidateUpdate(context.Background(), old, new)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("manual DNSRecord cannot be modified via v1alpha1"))
	g.Expect(warnings).To(BeEmpty())
}
```

- [ ] **Step 2: Implement minimal webhook**

```go
package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	apiv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

const annotationV1Alpha2DNSRecordSpec = "sreportal.io/v1alpha2-dnsrecord-spec"

// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha1-dnsrecord,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=dnsrecords,verbs=update,versions=v1alpha1,name=vdnsrecord-v1alpha1.kb.io,admissionReviewVersions=v1

type DNSRecordWebhook struct{}

var (
	_ webhook.CustomValidator = (*DNSRecordWebhook)(nil)
)

func (w *DNSRecordWebhook) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (w *DNSRecordWebhook) ValidateUpdate(_ context.Context, oldObj, _ runtime.Object) (admission.Warnings, error) {
	old, ok := oldObj.(*apiv1alpha1.DNSRecord)
	if !ok || old == nil {
		return nil, nil
	}
	if _, manual := old.Annotations[annotationV1Alpha2DNSRecordSpec]; !manual {
		return nil, nil
	}
	return nil, fmt.Errorf("manual DNSRecord cannot be modified via v1alpha1; use sreportal.io/v1alpha2")
}

func (w *DNSRecordWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
```

- [ ] **Step 3: Wire it up in `cmd/main.go`**

Locate where v1alpha2 webhooks register (`internal/webhook/v1alpha2/...`); add a parallel registration for `v1alpha1.DNSRecordWebhook` via `ctrl.NewWebhookManagedBy(mgr).For(&v1alpha1.DNSRecord{}).WithValidator(&v1alpha1webhook.DNSRecordWebhook{}).Complete()`.

- [ ] **Step 4: Run test — PASS**

- [ ] **Step 5: Regenerate webhook manifest**

```
make helm
```

- [ ] **Step 6: Commit**

```
git add internal/webhook/v1alpha1 cmd/main.go config/webhook helm/templates/validating-webhook-configuration.yaml
git commit -m "feat(webhook/v1alpha1): reject DNSRecord updates that would clobber manual entries"
```

---

## Phase 3 — Multi-DNS source aggregation

Goal: deliver the per-DNS config distribution promise. `SourceReconciler` must:
1. Merge sources/groupMapping across all DNS CRs deterministically.
2. Scope `reconcileOne(portal)` so a notify for portal A does not rebuild portal B's sources.
3. Surface a degraded condition + log when configs conflict in ways that cannot be merged (e.g., two DNS CRs both define `Sources.Service` with incompatible `Namespace`).

### Task 3.1: Define merged-config builder + ordering rule

**Files:**
- Create: `internal/controller/source/merged_config.go`
- Test: `internal/controller/source/merged_config_test.go`

- [ ] **Step 1: Decide ordering**

Order DNS CRs deterministically by `Namespace + "/" + Name`. Document at the top of `merged_config.go`:

> When two DNS CRs both define a non-nil `Sources.Service` (or any other source), the lexicographically-first DNS wins. The reconciler also sets `Status.Conditions[Type=SourceConflict]=True` on the *loser* so operators see the collision.

- [ ] **Step 2: Write the test first**

```go
func TestMergedConfig_DeterministicByName(t *testing.T) {
	g := NewWithT(t)

	cfgA := &ResolvedDNSConfig{DNSName: "portal-a", Namespace: "ns1",
		Sources: v1alpha2.SourcesSpec{
			Service: &v1alpha2.ServiceSourceSpec{CommonSourceSpec: v1alpha2.CommonSourceSpec{Enabled: true, Namespace: "team-a"}},
		},
		GroupMapping: v1alpha2.GroupMappingSpec{DefaultGroup: "A"},
	}
	cfgB := &ResolvedDNSConfig{DNSName: "portal-b", Namespace: "ns1",
		Sources: v1alpha2.SourcesSpec{
			Ingress: &v1alpha2.IngressSourceSpec{CommonSourceSpec: v1alpha2.CommonSourceSpec{Enabled: true}},
		},
		GroupMapping: v1alpha2.GroupMappingSpec{DefaultGroup: "B"},
	}

	merged, conflicts := MergeConfigs(map[string]*ResolvedDNSConfig{"portal-b": cfgB, "portal-a": cfgA})
	g.Expect(conflicts).To(BeEmpty())
	g.Expect(merged.Sources.Service).NotTo(BeNil())
	g.Expect(merged.Sources.Service.Namespace).To(Equal("team-a"))
	g.Expect(merged.Sources.Ingress).NotTo(BeNil())
	// First-by-name wins the GroupMapping
	g.Expect(merged.GroupMapping.DefaultGroup).To(Equal("A"))
}

func TestMergedConfig_ConflictReportsLoser(t *testing.T) {
	g := NewWithT(t)
	cfgA := &ResolvedDNSConfig{DNSName: "portal-a", Sources: v1alpha2.SourcesSpec{
		Service: &v1alpha2.ServiceSourceSpec{CommonSourceSpec: v1alpha2.CommonSourceSpec{Enabled: true, Namespace: "team-a"}},
	}}
	cfgZ := &ResolvedDNSConfig{DNSName: "portal-z", Sources: v1alpha2.SourcesSpec{
		Service: &v1alpha2.ServiceSourceSpec{CommonSourceSpec: v1alpha2.CommonSourceSpec{Enabled: true, Namespace: "team-z"}},
	}}
	_, conflicts := MergeConfigs(map[string]*ResolvedDNSConfig{"portal-z": cfgZ, "portal-a": cfgA})
	g.Expect(conflicts).To(HaveKeyWithValue("portal-z", ConsistOf("Sources.Service")))
}
```

- [ ] **Step 3: Implement `MergeConfigs`**

```go
package source

import (
	"sort"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

// MergeConfigs combines a per-portal map of ResolvedDNSConfig into a single
// effective config. Ordering: by Name lexicographically. The first non-nil
// value for each source kind wins; subsequent definitions are recorded in the
// returned conflict map (key: losing portal name, value: list of fields).
func MergeConfigs(configs map[string]*ResolvedDNSConfig) (effective ResolvedDNSConfig, conflicts map[string][]string) {
	keys := make([]string, 0, len(configs))
	for k := range configs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	conflicts = map[string][]string{}
	for _, k := range keys {
		cfg := configs[k]
		mergeSource(&effective.Sources.Service, cfg.Sources.Service, "Sources.Service", k, conflicts)
		mergeSource(&effective.Sources.Ingress, cfg.Sources.Ingress, "Sources.Ingress", k, conflicts)
		mergeSource(&effective.Sources.DNSEndpoint, cfg.Sources.DNSEndpoint, "Sources.DNSEndpoint", k, conflicts)
		mergeSource(&effective.Sources.IstioGateway, cfg.Sources.IstioGateway, "Sources.IstioGateway", k, conflicts)
		mergeSource(&effective.Sources.IstioVirtualService, cfg.Sources.IstioVirtualService, "Sources.IstioVirtualService", k, conflicts)
		mergeSource(&effective.Sources.GatewayHTTPRoute, cfg.Sources.GatewayHTTPRoute, "Sources.GatewayHTTPRoute", k, conflicts)
		mergeSource(&effective.Sources.GatewayGRPCRoute, cfg.Sources.GatewayGRPCRoute, "Sources.GatewayGRPCRoute", k, conflicts)
		mergeSource(&effective.Sources.GatewayTLSRoute, cfg.Sources.GatewayTLSRoute, "Sources.GatewayTLSRoute", k, conflicts)
		mergeSource(&effective.Sources.GatewayTCPRoute, cfg.Sources.GatewayTCPRoute, "Sources.GatewayTCPRoute", k, conflicts)
		mergeSource(&effective.Sources.GatewayUDPRoute, cfg.Sources.GatewayUDPRoute, "Sources.GatewayUDPRoute", k, conflicts)
		mergeSource(&effective.Sources.CrossplaneScalewayRecord, cfg.Sources.CrossplaneScalewayRecord, "Sources.CrossplaneScalewayRecord", k, conflicts)
		// Priority: concatenate de-dup preserving first-seen order
		for _, p := range cfg.Sources.Priority {
			if !containsSourceType(effective.Sources.Priority, p) {
				effective.Sources.Priority = append(effective.Sources.Priority, p)
			}
		}
		// GroupMapping/Reconciliation: first-wins
		if effective.GroupMapping.DefaultGroup == "" {
			effective.GroupMapping = cfg.GroupMapping
		}
		if effective.Reconciliation.Interval.Duration == 0 {
			effective.Reconciliation = cfg.Reconciliation
		}
	}
	return effective, conflicts
}

func mergeSource[T any](dst **T, src *T, field, owner string, conflicts map[string][]string) {
	if src == nil {
		return
	}
	if *dst == nil {
		*dst = src
		return
	}
	conflicts[owner] = append(conflicts[owner], field)
}

func containsSourceType(s []v1alpha2.SourceType, v v1alpha2.SourceType) bool {
	for _, e := range s {
		if e == v {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests — PASS**

```
go test ./internal/controller/source/... -run TestMergedConfig
```

- [ ] **Step 5: Commit**

```
git add internal/controller/source/merged_config.go internal/controller/source/merged_config_test.go
git commit -m "feat(source): deterministic multi-DNS config merging"
```

### Task 3.2: Replace `synthesizeConfig` with merged builder

**Files:**
- Modify: `internal/controller/source/source_controller.go`

- [ ] **Step 1: Test first — multi-DNS now picks both sources**

In `internal/controller/source/source_controller_test.go`, add:
```go
var _ = Describe("SourceReconciler multi-DNS", func() {
	It("enables sources from every DNS CR (no first-from-map flap)", func() {
		r := &SourceReconciler{configs: map[string]*ResolvedDNSConfig{}}
		r.configs["portal-a"] = &ResolvedDNSConfig{DNSName: "portal-a", Sources: v1alpha2.SourcesSpec{
			Service: &v1alpha2.ServiceSourceSpec{CommonSourceSpec: v1alpha2.CommonSourceSpec{Enabled: true}},
		}}
		r.configs["portal-b"] = &ResolvedDNSConfig{DNSName: "portal-b", Sources: v1alpha2.SourcesSpec{
			Ingress: &v1alpha2.IngressSourceSpec{CommonSourceSpec: v1alpha2.CommonSourceSpec{Enabled: true}},
		}}
		oc := r.synthesizeConfig()
		Expect(oc.Sources.Service).NotTo(BeNil())
		Expect(oc.Sources.Service.Enabled).To(BeTrue())
		Expect(oc.Sources.Ingress).NotTo(BeNil())
		Expect(oc.Sources.Ingress.Enabled).To(BeTrue())
	})
})
```

- [ ] **Step 2: Run — expect FAIL (current synthesizeConfig only picks one)**

- [ ] **Step 3: Replace `synthesizeConfig` body**

```go
func (r *SourceReconciler) synthesizeConfig() *config.OperatorConfig {
	r.configsMu.RLock()
	defer r.configsMu.RUnlock()
	if len(r.configs) == 0 {
		return &config.OperatorConfig{}
	}
	merged, conflicts := MergeConfigs(r.configs)
	if len(conflicts) > 0 {
		logger := log.Default().WithName("source")
		for owner, fields := range conflicts {
			logger.Info("DNS config conflict — losing definitions ignored",
				"portal", owner, "fields", fields)
		}
		// Stash conflicts on the receiver for status reporting.
		r.lastConflictsMu.Lock()
		r.lastConflicts = conflicts
		r.lastConflictsMu.Unlock()
	}
	return v1alpha2ToOperatorConfig(&merged)
}
```

Add the fields to `SourceReconciler`:
```go
lastConflictsMu sync.RWMutex
lastConflicts   map[string][]string
```

- [ ] **Step 4: Run test — PASS**

- [ ] **Step 5: Commit**

```
git add internal/controller/source/source_controller.go internal/controller/source/source_controller_test.go
git commit -m "fix(source): merge configs from all DNS CRs in synthesizeConfig"
```

### Task 3.3: Surface merge conflicts on DNS status

**Files:**
- Modify: `internal/controller/source/source_controller.go` (in `reconcile` or a dedicated step)
- Tests added in Task 3.5.

- [ ] **Step 1: After `reconcile()` succeeds, patch a `SourceConflict` condition on each losing DNS**

Add a helper:
```go
func (r *SourceReconciler) publishConflicts(ctx context.Context) {
	r.lastConflictsMu.RLock()
	conflicts := r.lastConflicts
	r.lastConflictsMu.RUnlock()
	if len(conflicts) == 0 {
		return
	}
	for portal, fields := range conflicts {
		var dns v1alpha2.DNS
		ns := r.getDNSNamespace(portal)
		if ns == "" {
			continue
		}
		if err := r.Get(ctx, client.ObjectKey{Name: portal, Namespace: ns}, &dns); err != nil {
			continue
		}
		base := dns.DeepCopy()
		meta.SetStatusCondition(&dns.Status.Conditions, metav1.Condition{
			Type:    "SourceConflict",
			Status:  metav1.ConditionTrue,
			Reason:  "ConfigOverriddenByLexFirstDNS",
			Message: fmt.Sprintf("Fields ignored due to conflict with earlier DNS: %s", strings.Join(fields, ", ")),
		})
		_ = r.Status().Patch(ctx, &dns, client.MergeFrom(base))
	}
}
```

Call from `Start()` after each successful reconcileAll/reconcileOne.

- [ ] **Step 2: Commit**

```
git add internal/controller/source/source_controller.go
git commit -m "feat(source): publish SourceConflict condition on losing DNS CRs"
```

### Task 3.4: Scope `reconcileOne(portal)` to actually use the portal

For now, the minimum-viable change is to rebuild sources from the merged config *and* call `MarkDegraded`/`publishConflicts` only for the notifying portal's records. Full per-portal source isolation is a follow-up; document it in the source file header.

**Files:**
- Modify: `internal/controller/source/source_controller.go`

- [ ] **Step 1: Update `reconcileOne` signature**

```go
func (r *SourceReconciler) reconcileOne(ctx context.Context, portalName string) error {
	// Per-DNS scoping: rebuild sources from the merged config (cheap), then run
	// the chain. The chain itself filters DNSRecord targets by portalRef when
	// projecting, so unrelated portals are not touched.
	if err := r.RebuildSources(ctx); err != nil {
		return fmt.Errorf("rebuild sources for portal %q: %w", portalName, err)
	}
	return r.reconcile(ctx)
}
```

- [ ] **Step 2: Add a regression test** that calls `reconcileOne` and verifies `RebuildSources` was invoked.

(Test impl: inject a recording `sourceFactory` double, count `BuildTypedSources` calls.)

- [ ] **Step 3: Commit**

```
git add internal/controller/source/source_controller.go internal/controller/source/source_controller_test.go
git commit -m "fix(source): reconcileOne rebuilds sources from merged config (was no-op)"
```

### Task 3.5: Config lifecycle tests for `reloadAllDNSConfigs` / `reloadDNSConfig`

**Files:**
- Modify: `internal/controller/source/source_controller_test.go`

- [ ] **Step 1: Add tests**

```go
var _ = Describe("SourceReconciler config lifecycle", func() {
	var (
		r       *SourceReconciler
		fakeCli client.Client
	)
	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(v1alpha2.AddToScheme(scheme)).To(Succeed())
		fakeCli = fake.NewClientBuilder().WithScheme(scheme).Build()
		r = &SourceReconciler{Client: fakeCli, configs: map[string]*ResolvedDNSConfig{}}
	})

	It("reloadAllDNSConfigs populates configs map and skips IsRemote", func() {
		Expect(fakeCli.Create(ctx, &v1alpha2.DNS{
			ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "ns"},
			Spec:       v1alpha2.DNSSpec{PortalRef: "p1"},
		})).To(Succeed())
		Expect(fakeCli.Create(ctx, &v1alpha2.DNS{
			ObjectMeta: metav1.ObjectMeta{Name: "p2", Namespace: "ns"},
			Spec:       v1alpha2.DNSSpec{PortalRef: "p2", IsRemote: true},
		})).To(Succeed())

		Expect(r.reloadAllDNSConfigs(ctx)).To(Succeed())
		Expect(r.configs).To(HaveKey("p1"))
		Expect(r.configs).NotTo(HaveKey("p2"))
	})

	It("reloadDNSConfig deletes from cache on NotFound", func() {
		r.configs["gone"] = &ResolvedDNSConfig{DNSName: "gone", Namespace: "ns"}
		Expect(r.reloadDNSConfig(ctx, "gone")).To(Succeed())
		Expect(r.configs).NotTo(HaveKey("gone"))
	})

	It("reloadDNSConfig no-op when generation unchanged", func() {
		Expect(fakeCli.Create(ctx, &v1alpha2.DNS{
			ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "ns", Generation: 7},
			Spec:       v1alpha2.DNSSpec{PortalRef: "p1"},
		})).To(Succeed())
		r.configs["p1"] = &ResolvedDNSConfig{DNSName: "p1", Namespace: "ns", Generation: 7}
		before := r.configs["p1"]
		Expect(r.reloadDNSConfig(ctx, "p1")).To(Succeed())
		Expect(r.configs["p1"]).To(BeIdenticalTo(before))
	})
})

var _ = Describe("SourceReconciler Notify", func() {
	It("drops events without blocking when channel is full", func() {
		r := &SourceReconciler{configChanged: make(chan string, 1)}
		r.Notify("a") // fills buffer
		// must not block:
		done := make(chan struct{})
		go func() { r.Notify("b"); close(done) }()
		Eventually(done, "100ms").Should(BeClosed())
	})
})
```

- [ ] **Step 2: Run — PASS**

- [ ] **Step 3: Commit**

```
git add internal/controller/source/source_controller_test.go
git commit -m "test(source): config lifecycle and Notify non-blocking coverage"
```

### Task 3.6: DNSConfigNotifier uses GenerationChangedPredicate

**Files:**
- Modify: `internal/controller/source/dns_config_notifier.go`

- [ ] **Step 1: Add predicate to the builder**

```go
return ctrl.NewControllerManagedBy(mgr).
	For(&v1alpha2.DNS{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
	Named("dns-config-notifier").
	Complete(r)
```

- [ ] **Step 2: Add a unit test**

```go
func TestDNSConfigNotifier_NotifiesOnGenerationChange(t *testing.T) {
	g := NewWithT(t)
	var notified []string
	n := &DNSConfigNotifier{notify: func(name string) { notified = append(notified, name) }}
	_, err := n.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "p1"}})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(notified).To(ConsistOf("p1"))
}
```

- [ ] **Step 3: Commit**

```
git add internal/controller/source/dns_config_notifier.go internal/controller/source/dns_config_notifier_test.go
git commit -m "fix(source): DNSConfigNotifier uses GenerationChangedPredicate"
```

---

## Phase 4 — Migration CLI hardening

### Task 4.1: Extract testable `Migrate()` and `slug()` into a package

**Files:**
- Create: `hack/migrate-dns-v2/migrate.go`
- Create: `hack/migrate-dns-v2/slug.go`
- Modify: `hack/migrate-dns-v2/main.go`

- [ ] **Step 1: Move `slug` into its own file with package `migratedns`**

(`hack/migrate-dns-v2/slug.go`):
```go
package main

import "strings"

func slug(s string) string {
	result := make([]byte, 0, len(s))
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
			result = append(result, byte(c))
		case c >= '0' && c <= '9':
			result = append(result, byte(c))
		case c >= 'A' && c <= 'Z':
			result = append(result, byte(c+32))
		default:
			result = append(result, '-')
		}
	}
	out := strings.Trim(string(result), "-")
	if out == "" {
		return "default"
	}
	return out
}
```

- [ ] **Step 2: Create `slug_test.go`**

```go
package main

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"":          "default",
		"Apps":      "apps",
		"My Group":  "my-group",
		"-leading":  "leading",
		"trailing-": "trailing",
		"___":       "default",
		"A B C":     "a-b-c",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 3: Run — PASS**

```
go test ./hack/migrate-dns-v2/...
```

- [ ] **Step 4: Commit**

```
git add hack/migrate-dns-v2/slug.go hack/migrate-dns-v2/slug_test.go
git commit -m "refactor(migrate-dns-v2): extract slug() for unit testing"
```

### Task 4.2: Failure tracking + non-zero exit + DryRunAll

**Files:**
- Modify: `hack/migrate-dns-v2/main.go`
- Create: `hack/migrate-dns-v2/migrate_test.go`

- [ ] **Step 1: Write the failing test (table-driven)**

```go
func TestMigrate_PartialFailure_KeepsAnnotation_NonZeroExit(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme()
	dns := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "ns",
			Annotations: map[string]string{annotationV1Alpha1Groups: `[{"name":"Apps","entries":[{"fqdn":"a.example.com"}]},{"name":"","entries":[{"fqdn":"bad"}]}]`}},
		Spec: v1alpha2.DNSSpec{PortalRef: "p1"},
	}
	// pre-create a colliding record so the second loop iteration errors
	existing := &v1alpha2.DNSRecord{ObjectMeta: metav1.ObjectMeta{Name: "p1-manual-default", Namespace: "ns"}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dns, existing).Build()

	sum, err := Migrate(context.Background(), cli, false)
	g.Expect(err).To(HaveOccurred())
	g.Expect(sum.Failures).To(BeNumerically(">", 0))

	var after v1alpha2.DNS
	g.Expect(cli.Get(context.Background(), client.ObjectKey{Name: "p1", Namespace: "ns"}, &after)).To(Succeed())
	g.Expect(after.Annotations).To(HaveKey(annotationV1Alpha1Groups))
}

func TestMigrate_DryRun_ValidatesViaServer(t *testing.T) { /* asserts client.DryRunAll passed to Create */ }
```

- [ ] **Step 2: Implement `Migrate` and `Summary`**

(`hack/migrate-dns-v2/migrate.go`):
```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

const annotationV1Alpha1Groups = "sreportal.io/v1alpha1-groups"

type Summary struct {
	DNSProcessed int
	Created      int
	AlreadyExist int
	Skipped      int
	Failures     int
}

func Migrate(ctx context.Context, c client.Client, dryRun bool) (Summary, error) {
	var (
		sum     Summary
		dnsList v1alpha2.DNSList
		errAggr []error
	)
	if err := c.List(ctx, &dnsList); err != nil {
		return sum, fmt.Errorf("list DNS: %w", err)
	}

	for i := range dnsList.Items {
		sum.DNSProcessed++
		dns := &dnsList.Items[i]
		raw, ok := dns.Annotations[annotationV1Alpha1Groups]
		if !ok || raw == "" {
			sum.Skipped++
			fmt.Printf("DNS %s/%s: no v1alpha1 groups annotation, skipping\n", dns.Namespace, dns.Name)
			continue
		}
		var groups []v1alpha1.DNSGroup
		if err := json.Unmarshal([]byte(raw), &groups); err != nil {
			sum.Failures++
			errAggr = append(errAggr, fmt.Errorf("DNS %s/%s: parse groups: %w", dns.Namespace, dns.Name, err))
			continue
		}

		groupCount, perDNSCreated, perDNSFailures := 0, 0, 0
		for _, g := range groups {
			if len(g.Entries) == 0 {
				continue
			}
			groupCount++
			recordName := dns.Name + "-manual-" + slug(g.Name)
			entries := make([]v1alpha2.DNSRecordEntry, 0, len(g.Entries))
			for _, e := range g.Entries {
				entries = append(entries, v1alpha2.DNSRecordEntry{
					FQDN: e.FQDN, Group: g.Name, Description: e.Description, RecordType: "A",
				})
			}
			record := &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: dns.Namespace},
				Spec: v1alpha2.DNSRecordSpec{
					Origin: v1alpha2.DNSRecordOriginManual, PortalRef: dns.Spec.PortalRef, Entries: entries,
				},
			}
			opts := []client.CreateOption{}
			if dryRun {
				opts = append(opts, client.DryRunAll)
			}
			if err := c.Create(ctx, record, opts...); err != nil {
				if apierrors.IsAlreadyExists(err) {
					sum.AlreadyExist++
					fmt.Printf("DNSRecord %s/%s already exists, leaving in place\n", dns.Namespace, recordName)
					continue
				}
				sum.Failures++
				perDNSFailures++
				errAggr = append(errAggr, fmt.Errorf("create %s/%s: %w", dns.Namespace, recordName, err))
				continue
			}
			perDNSCreated++
			sum.Created++
			if dryRun {
				fmt.Printf("[dry-run] would create DNSRecord %s/%s (%d entries)\n", dns.Namespace, recordName, len(entries))
			} else {
				fmt.Printf("created DNSRecord %s/%s\n", dns.Namespace, recordName)
			}
		}

		// Only strip annotation if every non-empty group succeeded.
		if !dryRun && perDNSFailures == 0 && perDNSCreated == groupCount && groupCount > 0 {
			patch := client.MergeFrom(dns.DeepCopy())
			delete(dns.Annotations, annotationV1Alpha1Groups)
			if err := c.Patch(ctx, dns, patch); err != nil {
				sum.Failures++
				errAggr = append(errAggr, fmt.Errorf("remove annotation %s/%s: %w", dns.Namespace, dns.Name, err))
			}
		}
	}
	if len(errAggr) > 0 {
		return sum, errors.Join(errAggr...)
	}
	return sum, nil
}
```

- [ ] **Step 3: Update `main.go` to call `Migrate` and exit non-zero on failure**

```go
func main() {
	// flag parsing unchanged
	...
	sum, err := Migrate(ctx, c, *dryRun)
	fmt.Printf("summary: processed=%d created=%d alreadyExist=%d skipped=%d failures=%d\n",
		sum.DNSProcessed, sum.Created, sum.AlreadyExist, sum.Skipped, sum.Failures)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if sum.Failures > 0 {
		os.Exit(2)
	}
}
```

- [ ] **Step 4: Run tests — PASS**

```
go test ./hack/migrate-dns-v2/...
```

- [ ] **Step 5: Commit**

```
git add hack/migrate-dns-v2
git commit -m "fix(migrate-dns-v2): failure tracking, non-zero exit, dry-run server-validated"
```

---

## Phase 5 — Silent failure sweep

Each task is short and isolated; commit per task.

### Task 5.1: Wrap conversion annotation unmarshal errors with context

Already handled by Phase 2 (errors now include `%s/%s` and `%w`). No-op task; verify via `git grep "fmt.Errorf(\"unmarshal" api/v1alpha1/`.

### Task 5.2: `MarkDegraded` distinguish NotFound from real errors

**Files:**
- Modify: `internal/controller/source/source_controller.go:561-564`

- [ ] **Step 1: Change the error branch**

```go
var rec sreportalv1alpha1.DNSRecord
if err := r.Get(ctx, client.ObjectKey{Namespace: portal.Namespace, Name: name}, &rec); err != nil {
	if apierrors.IsNotFound(err) {
		return
	}
	logger.Error(err, "failed to load DNSRecord for degraded condition", "dnsRecord", name)
	return
}
```

- [ ] **Step 2: Commit**

```
git add internal/controller/source/source_controller.go
git commit -m "fix(source): MarkDegraded distinguishes NotFound from real errors"
```

### Task 5.3: DNSRecord controller returns error on read-store delete

**Files:**
- Modify: `internal/controller/dnsrecords/dnsrecord_controller.go:99-105`

- [ ] **Step 1: Return wErr to trigger requeue**

```go
if err := r.Get(ctx, req.NamespacedName, &record); err != nil {
	if client.IgnoreNotFound(err) == nil && r.fqdnWriter != nil {
		resourceKey := req.Namespace + "/" + req.Name
		if wErr := r.fqdnWriter.Delete(ctx, resourceKey); wErr != nil {
			logger.Error(wErr, "failed to delete FQDN views from read store")
			return ctrl.Result{}, wErr
		}
	}
	return ctrl.Result{}, client.IgnoreNotFound(err)
}
```

- [ ] **Step 2: Commit**

```
git add internal/controller/dnsrecords/dnsrecord_controller.go
git commit -m "fix(dnsrecords): requeue when read-store delete fails on tombstoned record"
```

### Task 5.4: Watch enqueue list errors return synthetic requeue

**Files:**
- Modify: `internal/controller/dnsrecords/dnsrecord_controller.go:186-188, 212-214`

- [ ] **Step 1: On `List` error, enqueue the source object so it retries**

Replace the `return nil` after `Error` with:
```go
return []ctrl.Request{{NamespacedName: client.ObjectKeyFromObject(portal)}}
// or for DNS watch:
return []ctrl.Request{{NamespacedName: client.ObjectKeyFromObject(dns)}}
```
Rationale: re-enqueueing the trigger object causes the watch handler to retry. The actual reconcile target (DNSRecord) will be re-listed on next attempt.

Alternatively, leave the body but add an annotation-based "force resync" via `mgr.GetEventRecorderFor(...).Event(...)`. Simpler is better — pick option 1.

- [ ] **Step 2: Commit**

```
git add internal/controller/dnsrecords/dnsrecord_controller.go
git commit -m "fix(dnsrecords): re-enqueue source object on watch list errors"
```

### Task 5.5: `ProjectStoreHandler` returns store-write errors

**Files:**
- Modify: `internal/controller/dnsrecords/chain/project_store.go:48-51`

- [ ] **Step 1: Return the error**

Replace the `return nil` (line 51) with `return err`. Add a wrapping `fmt.Errorf("project store: %w", err)` if not already present.

- [ ] **Step 2: Update test if it asserts nil**

- [ ] **Step 3: Commit**

```
git add internal/controller/dnsrecords/chain/project_store.go internal/controller/dnsrecords/chain/project_store_test.go
git commit -m "fix(dnsrecords): return read-store write errors instead of swallowing"
```

### Task 5.6: `LoadDNSConfigHandler` short-circuits on missing DNS

**Files:**
- Modify: `internal/controller/dnsrecords/chain/load_dns_config.go:50-53`

- [ ] **Step 1: Change behaviour**

```go
if err := h.client.Get(ctx, key, &dns); err != nil {
	if client.IgnoreNotFound(err) == nil {
		rc.Result = ctrl.Result{Requeue: true}
		return reconciler.ErrShortCircuit
	}
	return fmt.Errorf("load DNS config for portal %q: %w", record.Spec.PortalRef, err)
}
```

If `ErrShortCircuit` doesn't exist in `internal/reconciler`, add it:
```go
var ErrShortCircuit = errors.New("reconciler: short circuit")
```
And update `Chain.Execute` to treat it as a successful early exit (no error returned). If introducing this sentinel is intrusive, alternatively set `rc.Data.SkipRemainder = true` and check it in downstream handlers — pick whichever fits the existing chain idiom.

- [ ] **Step 2: Add a test case**

```go
func TestLoadDNSConfig_MissingDNS_ShortCircuits(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(v1alpha2.AddToScheme(scheme)).To(Succeed())
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	h := NewLoadDNSConfigHandler(cli)

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, ChainData]{
		Resource: &v1alpha2.DNSRecord{Spec: v1alpha2.DNSRecordSpec{PortalRef: "missing"}},
	}
	err := h.Handle(context.Background(), rc)
	g.Expect(errors.Is(err, reconciler.ErrShortCircuit)).To(BeTrue())
	g.Expect(rc.Result.Requeue).To(BeTrue())
}
```

- [ ] **Step 3: Commit**

```
git add internal/controller/dnsrecords/chain/load_dns_config.go internal/controller/dnsrecords/chain/load_dns_config_test.go internal/reconciler/handler.go
git commit -m "fix(dnsrecords): short-circuit chain when DNS CR is absent (avoid running with defaults)"
```

### Task 5.7: Clear `EndpointsHash` when endpoints become empty

**Files:**
- Modify: `internal/controller/dnsrecords/chain/sync_hash.go:46-54`

- [ ] **Step 1: Drop the empty-endpoints early-return**

The current code early-returns when `len(Status.Endpoints) == 0`, leaving the previously-computed hash in place. Replace with: compute the hash unconditionally (or set it to "" when empty). Pick the simpler one — `""` on empty:

```go
if len(record.Status.Endpoints) == 0 {
	if record.Status.EndpointsHash == "" {
		return nil
	}
	base := record.DeepCopy()
	record.Status.EndpointsHash = ""
	return h.client.Status().Patch(ctx, record, client.MergeFrom(base))
}
// ...rest unchanged
```

- [ ] **Step 2: Add a regression test**

```go
func TestSyncHash_ClearedOnEmptyEndpoints(t *testing.T) { /* assert hash empties after endpoints removed */ }
```

- [ ] **Step 3: Commit**

```
git add internal/controller/dnsrecords/chain/sync_hash.go internal/controller/dnsrecords/chain/sync_hash_test.go
git commit -m "fix(dnsrecords): clear EndpointsHash when endpoints become empty"
```

### Task 5.8: `EnrichEndpoints` log NotFound differently from real errors

**Files:**
- Modify: `internal/controller/source/source_controller.go:514-519`

- [ ] **Step 1: Distinguish**

```go
obj, err := r.dynamicClient.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
if err != nil {
	if apierrors.IsNotFound(err) {
		logger.V(2).Info("source resource gone, skipping annotation enrichment", "resource", res)
		continue
	}
	logger.Error(err, "failed to get resource for annotation enrichment", "resource", res)
	continue
}
```

- [ ] **Step 2: Commit**

```
git add internal/controller/source/source_controller.go
git commit -m "fix(source): distinguish NotFound from real errors in EnrichEndpoints"
```

### Task 5.9: `Notify` drop counter

**Files:**
- Modify: `internal/controller/source/source_controller.go:435-440`

- [ ] **Step 1: Add counter metric + log**

```go
func (r *SourceReconciler) Notify(portalName string) {
	select {
	case r.configChanged <- portalName:
	default:
		log.Default().WithName("source").V(1).Info("config-change notify channel full, dropped", "portal", portalName)
		metrics.SourceNotifyDropped.WithLabelValues(portalName).Inc()
	}
}
```

Register `SourceNotifyDropped` in `internal/metrics/`.

- [ ] **Step 2: Commit**

```
git add internal/controller/source/source_controller.go internal/metrics
git commit -m "fix(source): log + meter Notify drops"
```

---

## Phase 6 — Test backfill

### Task 6.1: CEL validation envtest for DNSRecord

**Files:**
- Create: `internal/webhook/v1alpha2/dnsrecord_cel_test.go` (alongside existing webhook tests; runs in same envtest suite)

- [ ] **Step 1: Add Ginkgo specs that POST malformed specs**

```go
var _ = Describe("DNSRecord CEL validation", func() {
	It("rejects origin=auto without sourceType", func() {
		rec := &v1alpha2.DNSRecord{ObjectMeta: metav1.ObjectMeta{
			Name: "bad-auto", Namespace: ns,
		}, Spec: v1alpha2.DNSRecordSpec{Origin: v1alpha2.DNSRecordOriginAuto, PortalRef: "p"}}
		err := k8sClient.Create(ctx, rec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("auto records require sourceType"))
	})

	It("rejects origin=manual with sourceType", func() { /* ... */ })
	It("rejects mutation of immutable origin", func() { /* create auto, patch to manual */ })
	It("rejects malformed FQDN", func() { /* entries: [{fqdn: "no-dot"}] */ })
})
```

- [ ] **Step 2: Run with envtest assets**

```
make test
```

- [ ] **Step 3: Commit**

```
git add internal/webhook/v1alpha2/dnsrecord_cel_test.go
git commit -m "test(webhook/v1alpha2): cover CEL validation paths via envtest"
```

### Task 6.2: ResolveDNSHandler coverage backfill

**Files:**
- Modify: `internal/controller/dnsrecords/chain/resolve_dns_test.go`

- [ ] **Step 1: Add the missing cases**

- nil resolver short-circuits
- empty endpoints short-circuits
- per-endpoint resolver error sets SyncStatus and logs (capture log via `logr.New(testr.NewWithOptions(...))`)
- patch failure surfaces wrapped error

(Concrete code mirrors existing test fixtures — write each case as a small `t.Run` block.)

- [ ] **Step 2: Commit**

```
git add internal/controller/dnsrecords/chain/resolve_dns_test.go
git commit -m "test(dnsrecords): restore ResolveDNS coverage lost during v1alpha2 refactor"
```

### Task 6.3: `MaterialiseManualEntries` edge cases

**Files:**
- Modify: `internal/controller/dnsrecords/chain/materialise_manual_test.go`

- [ ] **Step 1: Add cases**

- empty `Entries` (no panic, no spurious endpoints)
- `RecordType` defaults (A) and explicit AAAA/CNAME/TXT
- idempotency: running twice does not duplicate endpoints
- LastReconcileTime is set

- [ ] **Step 2: Commit**

```
git add internal/controller/dnsrecords/chain/materialise_manual_test.go
git commit -m "test(dnsrecords): cover MaterialiseManualEntries edge cases"
```

---

## Phase 7 — Final verification

### Task 7.1: Full regeneration

- [ ] **Step 1**

```
make helm
make doc
```

- [ ] **Step 2: Inspect**

```
git status
git diff
```
Expected: only auto-generated files; no surprise drift.

### Task 7.2: Full test suite + lint

- [ ] **Step 1**

```
make test
make lint
```

- [ ] **Step 2: Build**

```
go build ./...
```

- [ ] **Step 3: Final commit if regeneration produced changes**

```
git add config/ helm/ docs/ api/v1alpha2/zz_generated.deepcopy.go
git commit -m "chore: final regeneration"
```

### Task 7.3: Push and update PR

- [ ] **Step 1**

```
git push origin feat/dns-v1alpha2
```

- [ ] **Step 2: Add a comment to PR #274 summarising the fix-pack** (reference each phase and which review finding it addresses).

---

## Self-Review Notes

**Spec coverage check (review findings → task):**
- Multi-DNS aggregation broken → Phase 3 (Tasks 3.1–3.5)
- reconcileOne ignores portal → Task 3.4
- Migration CLI data loss + dry-run + tests → Phase 4
- DNS ConvertTo drops Sources/GroupMapping/Reconciliation → Task 2.1
- DNSRecord ConvertFrom drops Origin/Entries → Task 2.2
- Source config lifecycle untested → Task 3.5
- Annotation unmarshal context → covered by Phase 2 error wrapping
- `MarkDegraded` swallow → Task 5.2
- `Notify` drop logging → Task 5.9
- `LoadDNSConfigHandler` runs with defaults → Task 5.6
- `dnsrecord_controller.go` read-store delete swallow → Task 5.3
- Watch list errors swallowed → Task 5.4
- `ProjectStoreHandler` error swallow → Task 5.5
- `ResolveDNSHandler` coverage regression → Task 6.2
- CEL validation untested → Task 6.1
- `MaterialiseManualEntries` edge cases → Task 6.3
- Empty hash treated as in-sync → Task 5.7
- `EnrichEndpoints` NotFound conflation → Task 5.8
- DNSConfigNotifier no predicate → Task 3.6
- v1alpha1 PATCH erases manual entries → Task 2.3
- Type design: typed aliases + CommonSourceSpec → Phase 1
- CEL `has()` vs `size()` → Task 1.3 Step 2

**Out of scope (deferred / not addressed here):**
- `DNSRecordSpec` constructor helpers for shift-left of discriminated union (low-confidence improvement; can land in a follow-up).
- `name == portalRef` invariant at object-level CEL (defense-in-depth; current webhook is sufficient).
- `Entries` listMapKey single-record-per-FQDN restriction (intentional per current design — confirm separately).
- Full per-DNS source isolation in `SourceReconciler` (Task 3.4 implements the safe minimum; full split is a follow-up tracked in plan header).
