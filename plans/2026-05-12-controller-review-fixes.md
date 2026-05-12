# Controller Review Fixes — Action Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply the fixes identified by the 13-controller code review of 2026-05-12, prioritized by severity and grouped by controller.

**Architecture:** Each task is scoped to a single controller (or a single transverse concern) so it can be implemented, tested and committed independently. Cross-cutting concerns (silent error patterns, RBAC consistency) are grouped into dedicated tasks. We follow DDD / Clean Architecture / Chain of Responsibility per `CLAUDE.md`.

**Tech Stack:** Go 1.26, controller-runtime v0.23, Kubebuilder, Ginkgo v2 + Gomega + envtest. `make helm` regenerates `config/rbac/role.yaml` from `+kubebuilder:rbac` markers. `make test` runs the suite; `make lint` runs golangci-lint; `make doc` updates generated docs.

**Validation gates (run after each task that touches code generation or RBAC):**
- `make helm` (regenerates RBAC + CRDs)
- `go build ./...`
- `make test` (or scoped Ginkgo run for the touched package)
- `make lint`

**Note on the imageinventory loop-variable finding:** `go.mod` declares `go 1.26.1`, so loop-variable capture is safe (Go ≥ 1.22). That finding is dropped from this plan.

---

## P0 — Critical (correctness/security/runtime panic)

### Task 1: portal — guard nil RemoteClient in health-check and fetch handlers

**Files:**
- Modify: `internal/controller/portal/chain/health_check_remote.go`
- Modify: `internal/controller/portal/chain/fetch_remote_data.go`
- Test: `internal/controller/portal/chain/health_check_remote_test.go` (create or extend)
- Test: `internal/controller/portal/chain/fetch_remote_data_test.go` (create or extend)

- [ ] **Step 1: Write failing tests**

In each `*_test.go`, add a Ginkgo spec that constructs the handler, sets `rc.Data.RemoteClient = nil` and `rc.Resource.Spec.Remote = &…{}` (a remote portal), then calls `Handle(ctx, rc)`. Expect: `err == nil`, no panic, and the chain proceeds (no `RequeueAfter` set by the handler).

```go
It("returns nil without panic when RemoteClient is nil", func() {
    rc := &reconciler.ReconcileContext[…]{ … }
    rc.Data.RemoteClient = nil
    rc.Resource.Spec.Remote = &v1alpha1.RemoteSpec{URL: "https://example"}
    Expect(handler.Handle(ctx, rc)).To(Succeed())
})
```

- [ ] **Step 2: Run tests to verify they panic / fail**

Run: `go test ./internal/controller/portal/chain/... -run TestHandlers -v`
Expected: panic (`nil pointer dereference`) or failure.

- [ ] **Step 3: Add nil guard at the top of each `Handle`**

In both files, after the `portal.Spec.Remote == nil` check, add:

```go
if rc.Data.RemoteClient == nil {
    return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/controller/portal/chain/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/portal/chain/health_check_remote.go internal/controller/portal/chain/fetch_remote_data.go internal/controller/portal/chain/*_test.go
git commit -m "fix(portal): guard nil RemoteClient in remote chain handlers"
```

---

### Task 2: source — guard nil idx.Main before MarkDegraded

**Files:**
- Modify: `internal/controller/source/chain/collect_endpoints.go:77`
- Test: `internal/controller/source/chain/collect_endpoints_test.go`

- [ ] **Step 1: Write failing test**

```go
It("does not panic when idx.Main is nil and threshold is reached", func() {
    idx := &portalindex.Index{Main: nil}
    h := chain.NewCollectEndpointsHandler(/* with a stub tracker */)
    // Inject failures up to maxSourceConsecutiveFailures via the tracker stub.
    Expect(func() { _ = h.Handle(ctx, rc) }).NotTo(Panic())
})
```

- [ ] **Step 2: Run test to confirm it panics**

Run: `go test ./internal/controller/source/chain/... -run TestCollectEndpoints -v`

- [ ] **Step 3: Add nil guard at call site**

```go
if count >= maxSourceConsecutiveFailures && idx.Main != nil {
    h.failureTracker.MarkDegraded(ctx, idx.Main, ts.Type, err, count)
}
```

- [ ] **Step 4: Run test to verify pass**

- [ ] **Step 5: Commit**

```bash
git commit -am "fix(source): guard nil idx.Main before MarkDegraded"
```

---

### Task 3: release — propagate Delete error on TTL expiry

**Files:**
- Modify: `internal/controller/release/release_controller.go:85,103`
- Test: `internal/controller/release/release_controller_test.go`

- [ ] **Step 1: Write failing test**

Inject a fake client that returns a transient error on `Delete` of an expired CR. Assert `Reconcile` returns the error (so controller-runtime requeues).

- [ ] **Step 2: Run test, observe it currently returns nil**

- [ ] **Step 3: Fix the two call sites**

At `release_controller.go:103`:

```go
if err := r.Delete(ctx, &rel); err != nil && !apierrors.IsNotFound(err) {
    return ctrl.Result{}, fmt.Errorf("delete expired release CR: %w", err)
}
```

At `release_controller.go:85`, replace `_ = r.releaseWriter.Delete(...)` with a logged-and-propagated error:

```go
if err := r.releaseWriter.Delete(ctx, resourceKey); err != nil {
    log.Error(err, "delete release from readstore")
    return ctrl.Result{}, err
}
```

- [ ] **Step 4: Run test to verify pass**

- [ ] **Step 5: Commit**

```bash
git commit -am "fix(release): propagate delete errors so reconcile retries"
```

---

### Task 4: imageregistry — fix jitter loop dropping deferred images

**Files:**
- Modify: `internal/controller/imageregistry/chain/select_due_images.go:114-123`
- Test: `internal/controller/imageregistry/chain/select_due_images_test.go`

- [ ] **Step 1: Write failing test**

Construct a `ChainData` with N candidate images and a `JitterFn` that returns increasing positive delays. Assert that all N images are placed on a deferred schedule (not silently dropped) and that `RequeueAfter` equals the minimum delay.

- [ ] **Step 2: Run test, observe deferred images are lost**

- [ ] **Step 3: Replace the `else if` block with a tracked slice**

```go
type deferred struct {
    delay time.Duration
    image DueImage
}
var deferredList []deferred
for _, candidate := range candidates {
    delay := h.jitter(candidate)
    if delay == 0 {
        rc.Data.DueImages = append(rc.Data.DueImages, candidate)
        continue
    }
    deferredList = append(deferredList, deferred{delay: delay, image: candidate})
}
if len(deferredList) > 0 {
    sort.Slice(deferredList, func(i, j int) bool {
        return deferredList[i].delay < deferredList[j].delay
    })
    rc.Result.RequeueAfter = deferredList[0].delay
}
```

- [ ] **Step 4: Run test to verify**

- [ ] **Step 5: Commit**

```bash
git commit -am "fix(imageregistry): track all deferred images, not just minDelay"
```

---

### Task 5: alertmanager — add Secrets RBAC marker

**Files:**
- Modify: `internal/controller/alertmanager/alertmanager_controller.go` (add marker above `Reconcile`)
- Regenerate: `config/rbac/role.yaml` via `make helm`

- [ ] **Step 1: Add RBAC marker**

Above the existing `+kubebuilder:rbac` block on the Alertmanager reconciler:

```go
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
```

- [ ] **Step 2: Regenerate RBAC**

Run: `make helm`
Expected: `config/rbac/role.yaml` diff includes the new `secrets` rule (verbs get/list/watch).

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit (single commit including regen)**

```bash
git add internal/controller/alertmanager/alertmanager_controller.go config/rbac/role.yaml charts/
git commit -m "fix(alertmanager): grant secrets read RBAC for TLS remote clients"
```

---

### Task 6: component — add incidents RBAC marker

**Files:**
- Modify: `internal/controller/component/component_controller.go:71`
- Regenerate: `config/rbac/role.yaml` via `make helm`

- [ ] **Step 1: Add marker alongside existing RBAC**

```go
// +kubebuilder:rbac:groups=sreportal.io,resources=incidents,verbs=get;list;watch
```

- [ ] **Step 2: `make helm`**

- [ ] **Step 3: Verify build + test**

Run: `make test` (envtest covers component controller).

- [ ] **Step 4: Commit**

```bash
git add internal/controller/component/component_controller.go config/rbac/role.yaml charts/
git commit -m "fix(component): add incidents RBAC for ComputeStatusHandler"
```

---

### Task 7: imageinventory — replace Status().Update with Status().Patch

**Files:**
- Modify: `internal/controller/imageinventory/chain/handlers.go:138-145`
- Test: `internal/controller/imageinventory/chain/handlers_test.go`

- [ ] **Step 1: Write a failing test**

Two reconcilers patch status concurrently on the same CR. The current `Status().Update` overwrites; the fix uses `Status().Patch(MergeFrom)` which merges. Assert both `ObservedGeneration` and a foreign condition (set by another simulated controller) coexist after the patch.

- [ ] **Step 2: Run test, observe foreign condition lost**

- [ ] **Step 3: Replace `Update` with `Patch`**

```go
base := inv.DeepCopy()
inv.Status.ObservedGeneration = inv.Generation
if err := h.client.Status().Patch(ctx, inv, client.MergeFrom(base)); err != nil {
    return fmt.Errorf("patch observed generation: %w", err)
}
```

Remove the subsequent re-`Get` since `Patch` updates `inv` in place. Keep the existing `statusutil.SetConditionAndPatch` call.

- [ ] **Step 4: Run test to verify**

- [ ] **Step 5: Commit**

```bash
git commit -am "fix(imageinventory): use Status().Patch to avoid overwriting concurrent writes"
```

---

## P1 — Important (correctness / observability)

### Task 8: transverse silent-error fix on Portal feature-gate lookups

**Files:**
- Modify: `internal/controller/alertmanager/alertmanager_controller.go:104`
- Modify: `internal/controller/dnsrecords/dnsrecord_controller.go:101-106`
- Modify: `internal/controller/dns/dns_controller.go:122`
- Modify: `internal/controller/networkflowdiscovery/networkflowdiscovery.go:115`
- Test: one table-driven test per file

- [ ] **Step 1: Extract the lookup pattern**

Create `internal/controller/internal/featuregate/portal.go` with a single helper:

```go
// LookupPortalFeature fetches the referenced Portal and returns (enabled, requeue, err).
// NotFound => (false, false, nil). Transient error => (false, true, err).
func LookupPortalFeature(
    ctx context.Context,
    c client.Client,
    namespace, name string,
    isEnabled func(v1alpha1.PortalFeatures) bool,
) (enabled bool, err error) {
    var p v1alpha1.Portal
    if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &p); err != nil {
        if apierrors.IsNotFound(err) {
            return false, nil
        }
        return false, fmt.Errorf("get portal %s/%s: %w", namespace, name, err)
    }
    return isEnabled(p.Spec.Features), nil
}
```

- [ ] **Step 2: Write tests for the helper**

Three cases: NotFound → `(false, nil)`; transient error → `(false, err)`; success disabled → `(false, nil)`; success enabled → `(true, nil)`.

- [ ] **Step 3: Replace `if err == nil` patterns** in the four controllers with calls to this helper. Return the error from `Reconcile` so the workqueue retries.

- [ ] **Step 4: Run `make test`**

- [ ] **Step 5: Commit**

```bash
git commit -am "fix(controllers): surface transient errors on portal feature-gate lookups"
```

---

### Task 9: transverse silent-error fix on ReadStore writers

**Files:**
- Modify: `internal/controller/release/release_controller.go:85` (already covered in Task 3 — skip here)
- Modify: `internal/controller/alertmanager/alertmanager_controller.go:94,127`
- Modify: `internal/controller/incident/incident_controller.go:75`
- Modify: `internal/controller/maintenance/maintenance_controller.go:75`

- [ ] **Step 1: Decide the policy**

Choice: log at `Error` level + increment a `readstore_writer_errors_total{controller=...}` metric, but DO NOT fail the reconcile (the store is in-memory; failures are rare and the next successful reconcile will re-sync).

- [ ] **Step 2: Add the metric**

Modify `internal/metrics/controllers.go` (or equivalent) to add the counter and register it on the controller-runtime registry.

- [ ] **Step 3: Replace each `_ = writer.Delete(...)` site**

```go
if err := r.someWriter.Delete(ctx, key); err != nil {
    log.Error(err, "delete from readstore", "key", key)
    metrics.ReadstoreWriterErrors.WithLabelValues("incident").Inc()
}
```

- [ ] **Step 4: Run `make test` + `make lint`**

- [ ] **Step 5: Commit**

```bash
git commit -am "observability(controllers): log and meter readstore writer errors"
```

---

### Task 10: source — add missing RBAC markers (DNSRecords + Portals)

**Files:**
- Modify: `internal/controller/source/source_controller.go:120-122`
- Regenerate: `config/rbac/role.yaml` via `make helm`

- [ ] **Step 1: Add markers**

```go
// +kubebuilder:rbac:groups=sreportal.io,resources=dnsrecords,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=dnsrecords/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=portals,verbs=get;list;watch
```

- [ ] **Step 2: `make helm`**

- [ ] **Step 3: Verify**: `go build ./...` + `make test`.

- [ ] **Step 4: Commit**

```bash
git add internal/controller/source/source_controller.go config/rbac/role.yaml charts/
git commit -m "fix(source): co-locate RBAC markers for resources it reads/writes"
```

---

### Task 11: dnsrecords — co-locate RBAC markers + add missing portals marker

**Files:**
- Modify: `internal/controller/dnsrecords/dnsrecord_controller.go:74`
- Regenerate via `make helm`

- [ ] **Step 1: Add markers**

```go
// +kubebuilder:rbac:groups=sreportal.io,resources=dnsrecords,verbs=get;list;watch
// +kubebuilder:rbac:groups=sreportal.io,resources=dnsrecords/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=portals,verbs=get;list;watch
```

- [ ] **Step 2: `make helm` → verify role.yaml diff is empty or idempotent (rules deduplicated).**

- [ ] **Step 3: Commit**

```bash
git add internal/controller/dnsrecords/dnsrecord_controller.go config/rbac/role.yaml charts/
git commit -m "fix(dnsrecords): co-locate RBAC markers on the reconciler"
```

---

### Task 12: networkflowdiscovery — fix multi-namespace list

**Files:**
- Modify: `internal/controller/networkflowdiscovery/chain/build_graph.go:100-105`
- Test: `internal/controller/networkflowdiscovery/chain/build_graph_test.go`

- [ ] **Step 1: Write failing test**

Spec with `namespaces: ["ns-a", "ns-b"]`. Stub client records the `ListOptions` used. Assert that the controller lists each namespace separately (or uses a non-nil restriction), and that pods in `ns-c` are NOT in the result.

- [ ] **Step 2: Run test, observe cluster-wide list**

- [ ] **Step 3: Replace `buildListOpts` behavior**

When `len(namespaces) > 1`, iterate and call `List` per namespace, merging results. Or use a field selector if available. Simplest:

```go
func (h *Handler) listScoped(ctx context.Context, obj client.ObjectList, namespaces []string, extra ...client.ListOption) error {
    if len(namespaces) == 0 {
        return h.client.List(ctx, obj, extra...)
    }
    if len(namespaces) == 1 {
        return h.client.List(ctx, obj, append(extra, client.InNamespace(namespaces[0]))...)
    }
    // multi: iterate and merge into obj
    // (helper that uses reflection or per-type code, depending on existing style)
}
```

If the per-type code is simpler, inline the loop at each call site rather than building a generic helper.

- [ ] **Step 4: Verify test passes**

- [ ] **Step 5: Commit**

```bash
git commit -am "fix(networkflowdiscovery): restrict list to declared namespaces"
```

---

### Task 13: dns — set owner reference + guard empty PortalRef in deleteManaged

**Files:**
- Modify: `internal/controller/dns/chain/reconcile_manual_components.go:88-111,135-152`

- [ ] **Step 1: Thread scheme into the handler**

If the handler does not yet hold a `*runtime.Scheme`, add it to the handler struct and wire it from `SetupWithManager` (use `mgr.GetScheme()`).

- [ ] **Step 2: Add owner reference before Create**

```go
if err := ctrl.SetControllerReference(resource, comp, h.scheme); err != nil {
    return fmt.Errorf("set owner reference on component %s: %w", comp.Name, err)
}
if err := h.client.Create(ctx, comp); err != nil { … }
```

- [ ] **Step 3: Guard deleteManaged on empty PortalRef**

```go
func (h *Handler) deleteManaged(ctx context.Context, resource *v1alpha1.DNS) error {
    if resource.Spec.PortalRef == "" {
        return nil
    }
    // …existing list+delete by label
}
```

- [ ] **Step 4: Add tests** for both fixes (envtest spec for owner reference; unit test for empty PortalRef guard).

- [ ] **Step 5: Commit**

```bash
git commit -am "fix(dns): set owner ref on auto-created Components, guard empty PortalRef"
```

---

### Task 14: source — set owner reference on auto-created Components

**Files:**
- Modify: `internal/controller/source/chain/reconcile_components.go:103-124`

- [ ] **Step 1: Thread scheme into the handler** (same pattern as Task 13).

- [ ] **Step 2: Decide owner**

The owner should be the Portal referenced by the Source (or the Source itself if Source is the parent). Confirm by reading `CLAUDE.md` memory `project_component_auto_creation.md` — design decision says annotation-driven, owner is the originating resource (Source). Use Source as owner.

- [ ] **Step 3: Call `ctrl.SetControllerReference(source, comp, scheme)` before `Create`.**

- [ ] **Step 4: Add envtest verifying GC cascade** (delete Source → Component disappears).

- [ ] **Step 5: Commit**

```bash
git commit -am "fix(source): set owner ref on auto-created Components for GC"
```

---

### Task 15: maintenance — set Condition on invalid schedule + extract RequeueAfter from chain

**Files:**
- Modify: `internal/controller/maintenance/maintenance_controller.go:88-93`
- Modify: `internal/controller/maintenance/chain/handlers.go:98`

- [ ] **Step 1: On chain error, set a False/InvalidSchedule condition**

In `Reconcile`, before returning the requeue result on chain error:

```go
if statusErr := statusutil.SetConditionAndPatch(
    ctx, r.Client, &maintenance,
    conditions.TypeReady, metav1.ConditionFalse,
    "InvalidSchedule", err.Error(),
); statusErr != nil {
    log.Error(statusErr, "patch invalid-schedule condition")
}
return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
```

- [ ] **Step 2: Move `RequeueAfter` computation out of `UpdateStatusHandler`**

In `Reconcile`, after `chain.Execute`, call `ComputeRequeue(maintenance.Spec.Schedule)` and set on `ctrl.Result`. Remove the duplicate assignment inside `UpdateStatusHandler`.

- [ ] **Step 3: Update / add tests** for both the invalid-schedule condition path and the unified RequeueAfter computation.

- [ ] **Step 4: Commit**

```bash
git commit -am "fix(maintenance): surface invalid-schedule via condition and unify requeue"
```

---

### Task 16: incident — single status+label write, build view from computed

**Files:**
- Modify: `internal/controller/incident/chain/handlers.go:85-109`

- [ ] **Step 1: Reorder operations**

Set label on `inc` BEFORE calling `SetConditionAndPatch`, so a single status update flushes everything. Remove the post-patch re-`Get`.

- [ ] **Step 2: Build view from `rc.Data.Computed`**

Replace `ToView(inc)` references to `inc.Status.*` with values from `rc.Data.Computed`, which is the authoritative computed state for this reconcile.

- [ ] **Step 3: Add test** for the "condition unchanged, label changed" case to ensure the label still lands.

- [ ] **Step 4: Commit**

```bash
git commit -am "fix(incident): consolidate label+status write, project view from computed state"
```

---

### Task 17: portal — handle AlreadyExists in EnsureMainPortalRunnable + continue on orphan delete errors

**Files:**
- Modify: `internal/controller/portal/chain/ensure_main.go:99`
- Modify: `internal/controller/portal/chain/sync_remote_alertmanager.go:214`

- [ ] **Step 1: Ignore AlreadyExists on Create**

```go
if err := r.client.Create(ctx, mainPortal); err != nil && !apierrors.IsAlreadyExists(err) {
    return fmt.Errorf("create main portal: %w", err)
}
```

- [ ] **Step 2: Log-and-continue in `cleanupOrphanedAlertmanagers`** (do not break the loop on first error):

```go
for _, am := range orphans {
    if err := r.client.Delete(ctx, &am); err != nil && !apierrors.IsNotFound(err) {
        log.Error(err, "delete orphan alertmanager", "name", am.Name)
        continue
    }
}
```

- [ ] **Step 3: Tests** for both: envtest concurrent Create; fake-client returning errors for one orphan and asserting the others are still deleted.

- [ ] **Step 4: Commit**

```bash
git commit -am "fix(portal): tolerate AlreadyExists on main create, continue orphan cleanup on errors"
```

---

### Task 18: imageinventory — sort injected container names for deterministic output

**Files:**
- Modify: `internal/controller/imageinventory/chain/scan_workloads.go:221-233`

- [ ] **Step 1: Add a failing test**

Build a Pod with two injected containers (`istio-proxy`, `linkerd-proxy`). Run `scanWorkloads` 100 times; assert the resulting `Workloads[*].Containers` slice ordering is identical every run.

- [ ] **Step 2: Sort keys before iterating**

```go
names := make([]string, 0, len(podImageByName))
for k := range podImageByName {
    names = append(names, k)
}
sort.Strings(names)
for _, cname := range names {
    img := podImageByName[cname]
    // …existing append logic
}
```

- [ ] **Step 3: Verify test**

- [ ] **Step 4: Commit**

```bash
git commit -am "fix(imageinventory): sort injected container names for deterministic CR diffs"
```

---

### Task 19: imageregistry — align UpdateStatusHandler with statusutil

**Files:**
- Modify: `internal/controller/imageregistry/chain/update_status.go:96-103`

- [ ] **Step 1: Replace direct `meta.SetStatusCondition` + `Status().Update`** with `statusutil.SetConditionAndPatch` so the no-op-skip optimisation kicks in.

- [ ] **Step 2: Also log the error of `SetConditionAndPatch` in `validate_spec.go:59`** (do not swallow with `_`).

- [ ] **Step 3: Tests**: idempotent reconcile must not bump `resourceVersion` when nothing changes.

- [ ] **Step 4: Commit**

```bash
git commit -am "fix(imageregistry): route status writes through statusutil for no-op skip"
```

---

## P2 — Architecture compliance

### Task 20: release — migrate to Chain of Responsibility

**Files:**
- Modify: `internal/controller/release/release_controller.go` (slim down `Reconcile`)
- Create: `internal/controller/release/chain/handlers.go`
- Create: per-handler files (TTLCheckHandler, FetchPortalHandler, ProjectToStoreHandler, DeleteHandler)
- Tests for each handler

- [ ] **Step 1: Read** the Chain pattern as used in `internal/controller/portal/chain/`, `internal/reconciler/handler.go`, `internal/reconciler/chain.go` to mirror conventions.

- [ ] **Step 2: Define `ChainData` struct** for release (e.g. `Portal *v1alpha1.Portal`, `Expired bool`).

- [ ] **Step 3: Implement handlers** one-by-one with tests (TDD) — small commits.

- [ ] **Step 4: Wire chain in `Reconcile`**; delete the monolithic logic.

- [ ] **Step 5: Run full suite** (`make test`); ensure release tests still pass.

- [ ] **Step 6: Commit (one per handler ideally)**

```bash
git commit -am "refactor(release): adopt Chain of Responsibility pattern"
```

---

### Task 21: dnsrecords — migrate to Chain of Responsibility

**Files:**
- Modify: `internal/controller/dnsrecords/dnsrecord_controller.go`
- Create: `internal/controller/dnsrecords/chain/` with one handler per logical step:
  - `FetchPortalHandler`
  - `FeatureGateHandler` (or rely on Task 8 helper)
  - `HashResyncHandler`
  - `ResolveEndpointsHandler`
  - `ProjectToReadstoreHandler`
  - `UpdateStatusHandler`

- [ ] **Step 1: Mirror the structure** used by `internal/controller/dns/chain/`.

- [ ] **Step 2: Define ChainData with `Portal *v1alpha1.Portal`, `Endpoints []EndpointStatus`, `Hashes ...`.**

- [ ] **Step 3: Move the existing parallel endpoint resolution** into a dedicated handler that preserves the current concurrency.

- [ ] **Step 4: TDD per handler.**

- [ ] **Step 5: Run `make test`.**

- [ ] **Step 6: Commit per handler.**

```bash
git commit -am "refactor(dnsrecords): adopt Chain of Responsibility pattern"
```

---

## Validation matrix

| Task | `make helm` | `make test` | `make lint` | `make doc` |
|------|---|---|---|---|
| 1, 2, 3, 4 | — | ✓ | ✓ | — |
| 5, 6, 10, 11 | ✓ | ✓ | ✓ | ✓ |
| 7, 8, 9 | — | ✓ | ✓ | — |
| 12, 13, 14, 15, 16, 17, 18, 19 | — | ✓ | ✓ | — |
| 20, 21 | — | ✓ | ✓ | — |

Always run the full quartet (`make helm && make test && make lint && make doc`) before opening a PR.

---

## Suggested PR grouping

Avoid a single mega-PR. Suggested batches:

1. **PR `fix/controller-panic-guards`** — Tasks 1, 2 (nil-pointer guards).
2. **PR `fix/controller-rbac-gaps`** — Tasks 5, 6, 10, 11 (RBAC markers; bundle to do one `make helm` run).
3. **PR `fix/controller-silent-errors`** — Tasks 3, 8, 9 (silent errors on Delete + portal lookup + readstore writers).
4. **PR `fix/imageregistry-jitter-and-status`** — Tasks 4, 19.
5. **PR `fix/imageinventory-patch-and-ordering`** — Tasks 7, 18.
6. **PR `fix/dns-source-component-owner-refs`** — Tasks 13, 14.
7. **PR `fix/maintenance-incident-portal-edges`** — Tasks 15, 16, 17.
8. **PR `fix/networkflowdiscovery-multi-namespace`** — Task 12.
9. **PR `refactor/release-chain`** — Task 20 (largest refactor — keep solo).
10. **PR `refactor/dnsrecords-chain`** — Task 21.

---

## Out of scope (explicit non-goals)

- Adding the Ginkgo `Describe/It` runner to the release suite (test framework deviation flagged; not a correctness bug).
- Timezone normalisation in maintenance `ToView` (UI-layer concern; tracked separately).
- Migrating the emoji runnable to a controller pattern (it is a Runnable, not a reconciler — by design).
- Sub-minute incident duration precision change (`int` minutes → seconds) — needs product input first.
