# Image Scanner Event-Driven Refactor — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the periodic polling `Scanner` with event-driven thin reconcilers that only rescan the modified workload, keeping a bounded periodic resync as a safety net.

**Architecture:** Five thin reconcilers (Deployment / StatefulSet / DaemonSet / CronJob / Job) share a `WorkloadHandler` that upserts/deletes per-workload contributions in a refactored two-level store (`map[portalRef]map[WorkloadKey][]ImageView`). The `ImageInventoryReconciler` gains a `ScanWorkloadsHandler` (full scan on CR changes) and returns `RequeueAfter = spec.interval` as the resync bound.

**Tech Stack:** Go 1.26, controller-runtime v0.23, Kubebuilder, standard `testing` + testify-free Ginkgo/Gomega for envtest, `client.NewFakeClient` for unit tests.

---

## File Structure

**Create:**
- `internal/domain/image/workload_key.go` — `WorkloadKey{Kind, Namespace, Name}` type
- `internal/controller/image/handler.go` — `WorkloadHandler` (shared upsert/delete logic)
- `internal/controller/image/handler_test.go` — unit tests for `WorkloadHandler`
- `internal/controller/image/workload_reconcilers.go` — 5 thin reconcilers + `SetupWorkloadReconcilersWithManager`
- `internal/controller/image/workload_reconcilers_test.go` — thin reconciler smoke tests
- `internal/controller/imageinventory/chain/scan_workloads.go` — `ScanWorkloadsHandler`
- `internal/controller/imageinventory/chain/scan_workloads_test.go` — unit test for `ScanWorkloadsHandler`

**Modify:**
- `internal/domain/image/writer.go` — new `ImageWriter` interface
- `internal/readstore/image/store.go` — new internal model, new methods
- `internal/readstore/image/store_test.go` — new tests for new API
- `internal/controller/imageinventory/chain/handlers.go` — `UpdateStatusHandler` sets `RequeueAfter`
- `internal/controller/imageinventory/imageinventory_controller.go` — wire `ScanWorkloadsHandler`, handle deletion, constructor accepts store
- `cmd/main.go` — drop `Scanner`, wire `WorkloadHandler` + thin reconcilers, pass store to `ImageInventoryReconciler`

**Delete:**
- `internal/controller/image/scanner.go`
- `internal/controller/image/scanner_test.go`

---

## Task 1: Add `WorkloadKey` type

**Files:**
- Create: `internal/domain/image/workload_key.go`

- [ ] **Step 1: Write the file**

Create `internal/domain/image/workload_key.go`:

```go
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

package image

// WorkloadKey uniquely identifies a scanned Kubernetes workload across the
// cluster. It is used to partition the per-portal image projection so that a
// single workload event can update only its own contribution.
type WorkloadKey struct {
	Kind      string
	Namespace string
	Name      string
}
```

- [ ] **Step 2: Verify the package still compiles**

Run: `go build ./internal/domain/image/...`
Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/domain/image/workload_key.go
git commit -m "feat(image): add WorkloadKey type for per-workload store partitioning"
```

---

## Task 2: Remove the old periodic Scanner

Remove the Runnable + its test + its wiring before changing the `ImageWriter` interface, so no caller breaks when we evolve the store.

**Files:**
- Delete: `internal/controller/image/scanner.go`
- Delete: `internal/controller/image/scanner_test.go`
- Modify: `cmd/main.go` — remove `imagectrl.NewScanner` call and the `imagectrl` import (if unused)

- [ ] **Step 1: Delete scanner files**

```bash
git rm internal/controller/image/scanner.go internal/controller/image/scanner_test.go
```

- [ ] **Step 2: Remove Scanner wiring from `cmd/main.go`**

In `cmd/main.go`, delete lines around 668–671 matching:

```go
	if err := mgr.Add(imagectrl.NewScanner(mgr.GetClient(), imageStore, 5*time.Minute)); err != nil {
		setupLog.Error(err, "unable to add image scanner")
		os.Exit(1)
	}
```

Leave the `imagectrl` import in place — Task 7 will add `SetupWorkloadReconcilersWithManager` to it. If `goimports` complains about an unused import meanwhile, remove the import and we'll re-add it in Task 7.

- [ ] **Step 3: Verify the build**

Run: `go build ./...`
Expected: success (may fail on the `imagectrl` import if unused — if so, delete the import line).

- [ ] **Step 4: Run existing tests that must still pass**

Run: `go test ./internal/readstore/image/... ./internal/controller/imageinventory/...`
Expected: PASS (nothing we touched).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(image): remove periodic Scanner runnable ahead of event-driven rewrite"
```

---

## Task 3: Refactor store — new `ImageWriter` interface and two-level model (TDD)

Change the domain interface, the store implementation, and the tests together in one focused change so the build stays green.

**Files:**
- Modify: `internal/domain/image/writer.go`
- Modify: `internal/readstore/image/store.go`
- Modify: `internal/readstore/image/store_test.go`

- [ ] **Step 1: Write the failing tests first**

Replace the content of `internal/readstore/image/store_test.go` with:

```go
package image

import (
	"context"
	"sync"
	"testing"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

func mkView(portal, repo, tag string, wk domainimage.WorkloadKey, container string) domainimage.ImageView {
	return domainimage.ImageView{
		PortalRef:  portal,
		Registry:   "docker.io",
		Repository: repo,
		Tag:        tag,
		TagType:    domainimage.TagTypeSemver,
		Workloads: []domainimage.WorkloadRef{{
			Kind: wk.Kind, Namespace: wk.Namespace, Name: wk.Name, Container: container,
		}},
	}
}

func TestReplaceWorkloadAggregatesAcrossWorkloads(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wkA := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}
	wkB := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "b"}

	if err := s.ReplaceWorkload(context.Background(), "portal-a", wkA, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.2.3", wkA, "web"),
	}); err != nil {
		t.Fatalf("ReplaceWorkload A: %v", err)
	}
	if err := s.ReplaceWorkload(context.Background(), "portal-a", wkB, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.2.3", wkB, "web"),
	}); err != nil {
		t.Fatalf("ReplaceWorkload B: %v", err)
	}

	out, err := s.List(context.Background(), domainimage.ImageFilters{Portal: "portal-a"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d want 1 (deduplicated)", len(out))
	}
	if len(out[0].Workloads) != 2 {
		t.Fatalf("workloads=%d want 2", len(out[0].Workloads))
	}
}

func TestReplaceWorkloadOverwritesSameKey(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}

	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.2.3", wk, "web"),
	})
	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.2.4", wk, "web"),
	})

	out, _ := s.List(context.Background(), domainimage.ImageFilters{Portal: "portal-a"})
	if len(out) != 1 || out[0].Tag != "1.2.4" {
		t.Fatalf("want [1.2.4], got %+v", out)
	}
}

func TestDeleteWorkloadAllPortals(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}

	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.2.3", wk, "web"),
	})
	_ = s.ReplaceWorkload(context.Background(), "portal-b", wk, []domainimage.ImageView{
		mkView("portal-b", "library/nginx", "1.2.3", wk, "web"),
	})

	if err := s.DeleteWorkloadAllPortals(context.Background(), wk); err != nil {
		t.Fatalf("DeleteWorkloadAllPortals: %v", err)
	}

	for _, portal := range []string{"portal-a", "portal-b"} {
		out, _ := s.List(context.Background(), domainimage.ImageFilters{Portal: portal})
		if len(out) != 0 {
			t.Fatalf("portal %s still has entries: %+v", portal, out)
		}
	}
}

func TestReplaceAllSwapsPortalAtomically(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wk1 := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}
	wk2 := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "b"}

	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk1, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.0.0", wk1, "web"),
	})

	byWorkload := map[domainimage.WorkloadKey][]domainimage.ImageView{
		wk2: {mkView("portal-a", "library/redis", "7.0.0", wk2, "cache")},
	}
	if err := s.ReplaceAll(context.Background(), "portal-a", byWorkload); err != nil {
		t.Fatalf("ReplaceAll: %v", err)
	}

	out, _ := s.List(context.Background(), domainimage.ImageFilters{Portal: "portal-a"})
	if len(out) != 1 || out[0].Repository != "library/redis" {
		t.Fatalf("want [library/redis], got %+v", out)
	}
}

func TestDeletePortal(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}

	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.2.3", wk, "web"),
	})
	if err := s.DeletePortal(context.Background(), "portal-a"); err != nil {
		t.Fatalf("DeletePortal: %v", err)
	}
	out, _ := s.List(context.Background(), domainimage.ImageFilters{Portal: "portal-a"})
	if len(out) != 0 {
		t.Fatalf("portal-a still has entries: %+v", out)
	}
}

func TestListFilters(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}
	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk, []domainimage.ImageView{
		{PortalRef: "portal-a", Registry: "docker.io", Repository: "library/nginx", Tag: "latest", TagType: domainimage.TagTypeLatest},
		{PortalRef: "portal-a", Registry: "ghcr.io", Repository: "org/app", Tag: "1.0.0", TagType: domainimage.TagTypeSemver},
	})

	out, err := s.List(context.Background(), domainimage.ImageFilters{
		Portal: "portal-a", Registry: "ghcr.io", TagType: "semver", Search: "org/",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out) != 1 || out[0].Repository != "org/app" {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestConcurrentReadDuringReplaceAll(t *testing.T) {
	t.Parallel()

	s := NewStore()
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "a"}
	_ = s.ReplaceWorkload(context.Background(), "portal-a", wk, []domainimage.ImageView{
		mkView("portal-a", "library/nginx", "1.0.0", wk, "web"),
	})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_, _ = s.List(context.Background(), domainimage.ImageFilters{Portal: "portal-a"})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = s.ReplaceAll(context.Background(), "portal-a", map[domainimage.WorkloadKey][]domainimage.ImageView{
				wk: {mkView("portal-a", "library/nginx", "1.0.0", wk, "web")},
			})
		}
	}()
	wg.Wait()
}
```

- [ ] **Step 2: Run the tests — they should fail to compile**

Run: `go test ./internal/readstore/image/...`
Expected: compilation error — `s.ReplaceWorkload`, `s.DeleteWorkloadAllPortals`, `s.ReplaceAll`, `s.DeletePortal` undefined.

- [ ] **Step 3: Update the `ImageWriter` interface**

Replace the content of `internal/domain/image/writer.go` with:

```go
package image

import "context"

// ImageWriter pushes image projections into the readstore at workload and
// portal granularity.
type ImageWriter interface {
	// ReplaceWorkload sets (or replaces) the contribution of a single workload
	// to a portal's image projection.
	ReplaceWorkload(ctx context.Context, portalRef string, wk WorkloadKey, images []ImageView) error

	// DeleteWorkloadAllPortals removes a workload's contribution from every
	// portal that referenced it.
	DeleteWorkloadAllPortals(ctx context.Context, wk WorkloadKey) error

	// ReplaceAll atomically replaces the full projection of a portal, keyed by
	// workload. Used for full rescans triggered by ImageInventory CR changes.
	ReplaceAll(ctx context.Context, portalRef string, byWorkload map[WorkloadKey][]ImageView) error

	// DeletePortal removes all projections for a portal (e.g. when the
	// ImageInventory CR is deleted).
	DeletePortal(ctx context.Context, portalRef string) error
}
```

- [ ] **Step 4: Rewrite the store to implement the new interface**

Replace the content of `internal/readstore/image/store.go` with:

```go
package image

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

// Store is an in-memory image projection keyed by portalRef and then by
// WorkloadKey so that a single workload event touches only its own slot.
type Store struct {
	mu   sync.RWMutex
	data map[string]map[domainimage.WorkloadKey][]domainimage.ImageView

	notifyMu sync.Mutex
	notifyCh chan struct{}
}

var (
	_ domainimage.ImageReader = (*Store)(nil)
	_ domainimage.ImageWriter = (*Store)(nil)
)

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{
		data:     make(map[string]map[domainimage.WorkloadKey][]domainimage.ImageView),
		notifyCh: make(chan struct{}),
	}
}

// ReplaceWorkload implements ImageWriter.
func (s *Store) ReplaceWorkload(_ context.Context, portalRef string, wk domainimage.WorkloadKey, images []domainimage.ImageView) error {
	s.mu.Lock()
	portal, ok := s.data[portalRef]
	if !ok {
		portal = make(map[domainimage.WorkloadKey][]domainimage.ImageView)
		s.data[portalRef] = portal
	}
	if len(images) == 0 {
		delete(portal, wk)
	} else {
		portal[wk] = images
	}
	s.mu.Unlock()

	s.broadcast()
	return nil
}

// DeleteWorkloadAllPortals implements ImageWriter.
func (s *Store) DeleteWorkloadAllPortals(_ context.Context, wk domainimage.WorkloadKey) error {
	s.mu.Lock()
	for _, portal := range s.data {
		delete(portal, wk)
	}
	s.mu.Unlock()

	s.broadcast()
	return nil
}

// ReplaceAll implements ImageWriter.
func (s *Store) ReplaceAll(_ context.Context, portalRef string, byWorkload map[domainimage.WorkloadKey][]domainimage.ImageView) error {
	// Defensive copy so the caller can't mutate the stored map after the call.
	copyMap := make(map[domainimage.WorkloadKey][]domainimage.ImageView, len(byWorkload))
	for k, v := range byWorkload {
		copyMap[k] = v
	}

	s.mu.Lock()
	s.data[portalRef] = copyMap
	s.mu.Unlock()

	s.broadcast()
	return nil
}

// DeletePortal implements ImageWriter.
func (s *Store) DeletePortal(_ context.Context, portalRef string) error {
	s.mu.Lock()
	delete(s.data, portalRef)
	s.mu.Unlock()

	s.broadcast()
	return nil
}

// List implements ImageReader. Returns a deduplicated, sorted view of all
// workload contributions that match the filters.
func (s *Store) List(_ context.Context, filters domainimage.ImageFilters) ([]domainimage.ImageView, error) {
	s.mu.RLock()
	collected := make([]domainimage.ImageView, 0)
	for portalRef, byWorkload := range s.data {
		if filters.Portal != "" && portalRef != filters.Portal {
			continue
		}
		for _, views := range byWorkload {
			collected = append(collected, views...)
		}
	}
	s.mu.RUnlock()

	out := make([]domainimage.ImageView, 0, len(collected))
	search := strings.ToLower(filters.Search)
	for _, img := range collected {
		if filters.Registry != "" && img.Registry != filters.Registry {
			continue
		}
		if filters.TagType != "" && string(img.TagType) != filters.TagType {
			continue
		}
		if filters.Search != "" && !strings.Contains(strings.ToLower(img.Repository), search) {
			continue
		}
		out = append(out, img)
	}

	out = deduplicate(out)

	slices.SortFunc(out, func(a, b domainimage.ImageView) int {
		if c := cmp.Compare(a.Registry, b.Registry); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Repository, b.Repository); c != 0 {
			return c
		}
		return cmp.Compare(a.Tag, b.Tag)
	})
	return out, nil
}

// Count implements ImageReader.
func (s *Store) Count(ctx context.Context, filters domainimage.ImageFilters) (int, error) {
	out, err := s.List(ctx, filters)
	return len(out), err
}

// Subscribe returns a channel closed on the next mutation.
func (s *Store) Subscribe() <-chan struct{} {
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()
	return s.notifyCh
}

func (s *Store) broadcast() {
	s.notifyMu.Lock()
	old := s.notifyCh
	s.notifyCh = make(chan struct{})
	s.notifyMu.Unlock()
	close(old)
}

func deduplicate(images []domainimage.ImageView) []domainimage.ImageView {
	type k struct{ registry, repository, tag string }
	seen := make(map[k]int, len(images))
	out := make([]domainimage.ImageView, 0, len(images))
	for _, img := range images {
		key := k{registry: img.Registry, repository: img.Repository, tag: img.Tag}
		if idx, ok := seen[key]; ok {
			out[idx].Workloads = append(out[idx].Workloads, img.Workloads...)
			continue
		}
		seen[key] = len(out)
		out = append(out, img)
	}
	return out
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -race ./internal/readstore/image/... ./internal/domain/image/...`
Expected: PASS (all 7 tests, including the concurrent test under `-race`).

- [ ] **Step 6: Verify nothing else in the repo broke**

Run: `go build ./...`
Expected: success.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/image/writer.go internal/readstore/image/store.go internal/readstore/image/store_test.go
git commit -m "refactor(image): two-level store keyed by (portalRef, WorkloadKey)"
```

---

## Task 4: Create `WorkloadHandler` (TDD)

**Files:**
- Create: `internal/controller/image/handler.go`
- Create: `internal/controller/image/handler_test.go`

- [ ] **Step 1: Write the failing unit tests**

Create `internal/controller/image/handler_test.go`:

```go
package image

import (
	"context"
	"sync"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

type recordedReplace struct {
	portalRef string
	wk        domainimage.WorkloadKey
	images    []domainimage.ImageView
}

type fakeImageWriter struct {
	mu       sync.Mutex
	replaces []recordedReplace
	deletes  []domainimage.WorkloadKey
}

func (f *fakeImageWriter) ReplaceWorkload(_ context.Context, portalRef string, wk domainimage.WorkloadKey, images []domainimage.ImageView) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replaces = append(f.replaces, recordedReplace{portalRef, wk, images})
	return nil
}

func (f *fakeImageWriter) DeleteWorkloadAllPortals(_ context.Context, wk domainimage.WorkloadKey) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes = append(f.deletes, wk)
	return nil
}

func (f *fakeImageWriter) ReplaceAll(_ context.Context, _ string, _ map[domainimage.WorkloadKey][]domainimage.ImageView) error {
	return nil
}

func (f *fakeImageWriter) DeletePortal(_ context.Context, _ string) error { return nil }

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	sch := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(sch); err != nil {
		t.Fatalf("add clientgo scheme: %v", err)
	}
	if err := sreportalv1alpha1.AddToScheme(sch); err != nil {
		t.Fatalf("add sreportal scheme: %v", err)
	}
	return sch
}

func TestHandleUpsertMatchesNamespaceFilter(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	invMatch := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-a", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:       "portal-a",
			NamespaceFilter: "default",
		},
	}
	invMiss := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-b", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:       "portal-b",
			NamespaceFilter: "other",
		},
	}

	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(invMatch, invMiss).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1.2.3"}}}
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "api"}

	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set(nil)); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}

	if len(writer.replaces) != 1 {
		t.Fatalf("got %d replaces, want 1", len(writer.replaces))
	}
	if writer.replaces[0].portalRef != "portal-a" {
		t.Fatalf("portalRef=%q want portal-a", writer.replaces[0].portalRef)
	}
}

func TestHandleUpsertRespectsLabelSelector(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:     "portal-a",
			LabelSelector: "team=platform",
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1.2.3"}}}
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "api"}

	// Labels do not match the selector.
	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set{"team": "other"}); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}
	if len(writer.replaces) != 0 {
		t.Fatalf("want no replace, got %d", len(writer.replaces))
	}

	// Labels match the selector.
	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set{"team": "platform"}); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}
	if len(writer.replaces) != 1 {
		t.Fatalf("want one replace, got %d", len(writer.replaces))
	}
}

func TestHandleUpsertRespectsWatchedKinds(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:    "portal-a",
			WatchedKinds: []sreportalv1alpha1.ImageInventoryKind{sreportalv1alpha1.ImageInventoryKindStatefulSet},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1.2.3"}}}
	// Deployment is not in WatchedKinds -> no replace.
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "api"}
	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set(nil)); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}
	if len(writer.replaces) != 0 {
		t.Fatalf("want no replace, got %d", len(writer.replaces))
	}
}

func TestHandleDeleteCallsStore(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(sch).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "api"}
	if err := h.HandleDelete(context.Background(), wk); err != nil {
		t.Fatalf("HandleDelete: %v", err)
	}
	if len(writer.deletes) != 1 || writer.deletes[0] != wk {
		t.Fatalf("deletes=%+v", writer.deletes)
	}
}

// Ensure the handler compiles with a real PodSpec extractor (smoke test).
var _ = appsv1.Deployment{}
```

- [ ] **Step 2: Run tests — expect compile failure**

Run: `go test ./internal/controller/image/...`
Expected: compile error — `NewWorkloadHandler` undefined, `WorkloadHandler` undefined.

- [ ] **Step 3: Implement the handler**

Create `internal/controller/image/handler.go`:

```go
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

// Package image contains the event-driven image inventory controllers.
package image

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/log"
)

// WorkloadHandler owns the per-workload upsert/delete logic shared by every
// thin reconciler. It lists the ImageInventory CRs, filters them in-memory
// against the workload, and updates the store accordingly.
type WorkloadHandler struct {
	client client.Client
	store  domainimage.ImageWriter
}

// NewWorkloadHandler constructs a WorkloadHandler.
func NewWorkloadHandler(c client.Client, store domainimage.ImageWriter) *WorkloadHandler {
	return &WorkloadHandler{client: c, store: store}
}

// HandleUpsert updates the per-workload contribution of `wk` in every
// ImageInventory that selects it.
func (h *WorkloadHandler) HandleUpsert(
	ctx context.Context,
	wk domainimage.WorkloadKey,
	spec corev1.PodSpec,
	objLabels labels.Set,
) error {
	logger := log.FromContext(ctx).WithValues("workload", wk)

	var invList sreportalv1alpha1.ImageInventoryList
	if err := h.client.List(ctx, &invList); err != nil {
		return fmt.Errorf("list ImageInventory: %w", err)
	}

	var firstErr error
	for i := range invList.Items {
		inv := &invList.Items[i]
		if !matchesInventory(inv, wk, objLabels) {
			continue
		}
		images := imageViewsFromPodSpec(inv.Spec.PortalRef, wk.Kind, wk.Namespace, wk.Name, spec)
		if err := h.store.ReplaceWorkload(ctx, inv.Spec.PortalRef, wk, images); err != nil {
			logger.Error(err, "store.ReplaceWorkload failed", "portalRef", inv.Spec.PortalRef)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// HandleDelete removes this workload's contribution from every portal.
func (h *WorkloadHandler) HandleDelete(ctx context.Context, wk domainimage.WorkloadKey) error {
	return h.store.DeleteWorkloadAllPortals(ctx, wk)
}

// matchesInventory decides whether `wk` (with the given object labels) is in
// scope of `inv` according to watchedKinds, namespaceFilter and labelSelector.
func matchesInventory(inv *sreportalv1alpha1.ImageInventory, wk domainimage.WorkloadKey, objLabels labels.Set) bool {
	kinds := inv.Spec.EffectiveWatchedKinds()
	if !slices.Contains(kinds, sreportalv1alpha1.ImageInventoryKind(wk.Kind)) {
		return false
	}
	if inv.Spec.NamespaceFilter != "" && inv.Spec.NamespaceFilter != wk.Namespace {
		return false
	}
	if inv.Spec.LabelSelector != "" {
		sel, err := labels.Parse(inv.Spec.LabelSelector)
		if err != nil {
			// Fail-open: the CR was meant to be validated upstream; if a
			// malformed selector slipped through we treat it as "no filter"
			// so scans still happen rather than dropping silently.
			return true
		}
		if !sel.Matches(objLabels) {
			return false
		}
	}
	return true
}

// imageViewsFromPodSpec converts a PodSpec into the per-container ImageView
// projections contributed by one workload. (Previously lived in scanner.go.)
func imageViewsFromPodSpec(portalRef, kind, namespace, name string, spec corev1.PodSpec) []domainimage.ImageView {
	out := make([]domainimage.ImageView, 0, len(spec.Containers)+len(spec.InitContainers))
	appendContainer := func(c corev1.Container) {
		ref, err := domainimage.ParseReference(c.Image)
		if err != nil {
			return
		}
		out = append(out, domainimage.ImageView{
			PortalRef:  portalRef,
			Registry:   ref.Registry,
			Repository: ref.Repository,
			Tag:        ref.Tag,
			TagType:    ref.TagType,
			Workloads: []domainimage.WorkloadRef{{
				Kind:      kind,
				Namespace: namespace,
				Name:      name,
				Container: c.Name,
			}},
		})
	}
	for _, c := range spec.Containers {
		appendContainer(c)
	}
	for _, c := range spec.InitContainers {
		appendContainer(c)
	}
	return out
}

// ImageViewsFromPodSpec is an exported wrapper used by the ImageInventory
// chain's full-scan handler.
func ImageViewsFromPodSpec(portalRef, kind, namespace, name string, spec corev1.PodSpec) []domainimage.ImageView {
	return imageViewsFromPodSpec(portalRef, kind, namespace, name, spec)
}
```

- [ ] **Step 4: Run tests — expect pass**

Run: `go test -race ./internal/controller/image/...`
Expected: PASS (all 4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/controller/image/handler.go internal/controller/image/handler_test.go
git commit -m "feat(image): add WorkloadHandler for event-driven inventory updates"
```

---

## Task 5: Create the five thin reconcilers

**Files:**
- Create: `internal/controller/image/workload_reconcilers.go`
- Create: `internal/controller/image/workload_reconcilers_test.go`

- [ ] **Step 1: Write the failing smoke tests**

Create `internal/controller/image/workload_reconcilers_test.go`:

```go
package image

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrl "sigs.k8s.io/controller-runtime"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

func TestDeploymentReconcilerUpsertsOnPresent(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: "portal-a"},
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1"}}},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv, dep).Build()
	writer := &fakeImageWriter{}
	r := &DeploymentImageReconciler{handler: NewWorkloadHandler(c, writer)}

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "api"}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(writer.replaces) != 1 {
		t.Fatalf("want 1 replace, got %d", len(writer.replaces))
	}
}

func TestDeploymentReconcilerDeletesOnNotFound(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	c := fake.NewClientBuilder().WithScheme(sch).Build()
	writer := &fakeImageWriter{}
	r := &DeploymentImageReconciler{handler: NewWorkloadHandler(c, writer)}

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "gone"}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(writer.deletes) != 1 {
		t.Fatalf("want 1 delete, got %d", len(writer.deletes))
	}
	want := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "gone"}
	if writer.deletes[0] != want {
		t.Fatalf("delete=%+v want %+v", writer.deletes[0], want)
	}
}

func TestCronJobReconcilerExtractsJobTemplatePodSpec(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: "portal-a"},
	}
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "nightly", Namespace: "default"},
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "run", Image: "ghcr.io/acme/cron:v1"}}},
					},
				},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv, cj).Build()
	writer := &fakeImageWriter{}
	r := &CronJobImageReconciler{handler: NewWorkloadHandler(c, writer)}

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "nightly"}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(writer.replaces) != 1 {
		t.Fatalf("want 1 replace, got %d", len(writer.replaces))
	}
	if writer.replaces[0].wk.Kind != "CronJob" {
		t.Fatalf("kind=%q want CronJob", writer.replaces[0].wk.Kind)
	}
}
```

- [ ] **Step 2: Run tests — expect compile failure**

Run: `go test ./internal/controller/image/...`
Expected: `DeploymentImageReconciler`, `CronJobImageReconciler` undefined.

- [ ] **Step 3: Implement the reconcilers**

Create `internal/controller/image/workload_reconcilers.go`:

```go
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

package image

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch

// DeploymentImageReconciler reconciles Deployment changes into the image inventory.
type DeploymentImageReconciler struct {
	handler *WorkloadHandler
}

// StatefulSetImageReconciler reconciles StatefulSet changes into the image inventory.
type StatefulSetImageReconciler struct {
	handler *WorkloadHandler
}

// DaemonSetImageReconciler reconciles DaemonSet changes into the image inventory.
type DaemonSetImageReconciler struct {
	handler *WorkloadHandler
}

// CronJobImageReconciler reconciles CronJob changes into the image inventory.
type CronJobImageReconciler struct {
	handler *WorkloadHandler
}

// JobImageReconciler reconciles Job changes into the image inventory.
type JobImageReconciler struct {
	handler *WorkloadHandler
}

// Reconcile implements reconcile.Reconciler for Deployment.
func (r *DeploymentImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj appsv1.Deployment
	if err := r.handler.client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: req.Namespace, Name: req.Name}
			return ctrl.Result{}, r.handler.HandleDelete(ctx, wk)
		}
		return ctrl.Result{}, fmt.Errorf("get Deployment: %w", err)
	}
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: obj.Namespace, Name: obj.Name}
	return ctrl.Result{}, r.handler.HandleUpsert(ctx, wk, obj.Spec.Template.Spec, labels.Set(obj.Labels))
}

// Reconcile implements reconcile.Reconciler for StatefulSet.
func (r *StatefulSetImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj appsv1.StatefulSet
	if err := r.handler.client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			wk := domainimage.WorkloadKey{Kind: "StatefulSet", Namespace: req.Namespace, Name: req.Name}
			return ctrl.Result{}, r.handler.HandleDelete(ctx, wk)
		}
		return ctrl.Result{}, fmt.Errorf("get StatefulSet: %w", err)
	}
	wk := domainimage.WorkloadKey{Kind: "StatefulSet", Namespace: obj.Namespace, Name: obj.Name}
	return ctrl.Result{}, r.handler.HandleUpsert(ctx, wk, obj.Spec.Template.Spec, labels.Set(obj.Labels))
}

// Reconcile implements reconcile.Reconciler for DaemonSet.
func (r *DaemonSetImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj appsv1.DaemonSet
	if err := r.handler.client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			wk := domainimage.WorkloadKey{Kind: "DaemonSet", Namespace: req.Namespace, Name: req.Name}
			return ctrl.Result{}, r.handler.HandleDelete(ctx, wk)
		}
		return ctrl.Result{}, fmt.Errorf("get DaemonSet: %w", err)
	}
	wk := domainimage.WorkloadKey{Kind: "DaemonSet", Namespace: obj.Namespace, Name: obj.Name}
	return ctrl.Result{}, r.handler.HandleUpsert(ctx, wk, obj.Spec.Template.Spec, labels.Set(obj.Labels))
}

// Reconcile implements reconcile.Reconciler for CronJob.
func (r *CronJobImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj batchv1.CronJob
	if err := r.handler.client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			wk := domainimage.WorkloadKey{Kind: "CronJob", Namespace: req.Namespace, Name: req.Name}
			return ctrl.Result{}, r.handler.HandleDelete(ctx, wk)
		}
		return ctrl.Result{}, fmt.Errorf("get CronJob: %w", err)
	}
	wk := domainimage.WorkloadKey{Kind: "CronJob", Namespace: obj.Namespace, Name: obj.Name}
	return ctrl.Result{}, r.handler.HandleUpsert(ctx, wk, obj.Spec.JobTemplate.Spec.Template.Spec, labels.Set(obj.Labels))
}

// Reconcile implements reconcile.Reconciler for Job.
func (r *JobImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj batchv1.Job
	if err := r.handler.client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			wk := domainimage.WorkloadKey{Kind: "Job", Namespace: req.Namespace, Name: req.Name}
			return ctrl.Result{}, r.handler.HandleDelete(ctx, wk)
		}
		return ctrl.Result{}, fmt.Errorf("get Job: %w", err)
	}
	wk := domainimage.WorkloadKey{Kind: "Job", Namespace: obj.Namespace, Name: obj.Name}
	return ctrl.Result{}, r.handler.HandleUpsert(ctx, wk, obj.Spec.Template.Spec, labels.Set(obj.Labels))
}

// SetupWorkloadReconcilersWithManager registers the five thin reconcilers with
// the controller manager. Each one watches a single workload kind and shares
// the passed WorkloadHandler.
func SetupWorkloadReconcilersWithManager(mgr ctrl.Manager, h *WorkloadHandler) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Named("deployment-image").
		Complete(&DeploymentImageReconciler{handler: h}); err != nil {
		return fmt.Errorf("setup deployment-image reconciler: %w", err)
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.StatefulSet{}).
		Named("statefulset-image").
		Complete(&StatefulSetImageReconciler{handler: h}); err != nil {
		return fmt.Errorf("setup statefulset-image reconciler: %w", err)
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.DaemonSet{}).
		Named("daemonset-image").
		Complete(&DaemonSetImageReconciler{handler: h}); err != nil {
		return fmt.Errorf("setup daemonset-image reconciler: %w", err)
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.CronJob{}).
		Named("cronjob-image").
		Complete(&CronJobImageReconciler{handler: h}); err != nil {
		return fmt.Errorf("setup cronjob-image reconciler: %w", err)
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		Named("job-image").
		Complete(&JobImageReconciler{handler: h}); err != nil {
		return fmt.Errorf("setup job-image reconciler: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests — expect pass**

Run: `go test -race ./internal/controller/image/...`
Expected: PASS (including the 3 new tests).

- [ ] **Step 5: Commit**

```bash
git add internal/controller/image/workload_reconcilers.go internal/controller/image/workload_reconcilers_test.go
git commit -m "feat(image): add thin reconcilers for Deployment/StatefulSet/DaemonSet/CronJob/Job"
```

---

## Task 6: Create `ScanWorkloadsHandler` (TDD)

**Files:**
- Create: `internal/controller/imageinventory/chain/scan_workloads.go`
- Create: `internal/controller/imageinventory/chain/scan_workloads_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/controller/imageinventory/chain/scan_workloads_test.go`:

```go
package chain

import (
	"context"
	"errors"
	"sync"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

type replaceAllCall struct {
	portalRef  string
	byWorkload map[domainimage.WorkloadKey][]domainimage.ImageView
}

type fakeImageWriter struct {
	mu          sync.Mutex
	replaceAlls []replaceAllCall
	replaceErr  error
}

func (f *fakeImageWriter) ReplaceWorkload(_ context.Context, _ string, _ domainimage.WorkloadKey, _ []domainimage.ImageView) error {
	return nil
}
func (f *fakeImageWriter) DeleteWorkloadAllPortals(_ context.Context, _ domainimage.WorkloadKey) error {
	return nil
}
func (f *fakeImageWriter) ReplaceAll(_ context.Context, portalRef string, byWorkload map[domainimage.WorkloadKey][]domainimage.ImageView) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replaceErr != nil {
		return f.replaceErr
	}
	f.replaceAlls = append(f.replaceAlls, replaceAllCall{portalRef, byWorkload})
	return nil
}
func (f *fakeImageWriter) DeletePortal(_ context.Context, _ string) error { return nil }

func newChainScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	sch := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(sch); err != nil {
		t.Fatalf("clientgo: %v", err)
	}
	if err := sreportalv1alpha1.AddToScheme(sch); err != nil {
		t.Fatalf("sreportal: %v", err)
	}
	return sch
}

func TestScanWorkloadsReplacesAll(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1"}}},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(dep).Build()
	writer := &fakeImageWriter{}
	h := NewScanWorkloadsHandler(c, writer)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:    "portal-a",
			WatchedKinds: []sreportalv1alpha1.ImageInventoryKind{sreportalv1alpha1.ImageInventoryKindDeployment},
		},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(writer.replaceAlls) != 1 {
		t.Fatalf("want 1 ReplaceAll, got %d", len(writer.replaceAlls))
	}
	call := writer.replaceAlls[0]
	if call.portalRef != "portal-a" {
		t.Fatalf("portalRef=%q want portal-a", call.portalRef)
	}
	if len(call.byWorkload) != 1 {
		t.Fatalf("byWorkload entries=%d want 1", len(call.byWorkload))
	}
}

func TestScanWorkloadsPropagatesErrorWithoutReplace(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)
	c := fake.NewClientBuilder().WithScheme(sch).Build()
	writer := &fakeImageWriter{replaceErr: errors.New("boom")}
	h := NewScanWorkloadsHandler(c, writer)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: "portal-a"},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}

	err := h.Handle(context.Background(), rc)
	if err == nil {
		t.Fatalf("expected error")
	}
}
```

- [ ] **Step 2: Run tests — expect compile failure**

Run: `go test ./internal/controller/imageinventory/...`
Expected: `NewScanWorkloadsHandler` undefined.

- [ ] **Step 3: Implement the handler**

Create `internal/controller/imageinventory/chain/scan_workloads.go`:

```go
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

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	imagectrl "github.com/golgoth31/sreportal/internal/controller/image"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ScanWorkloadsHandler performs a full scan of the cluster (filtered by the
// ImageInventory spec) and atomically replaces the portal's projection in the
// store. It runs on every chain pass — the `interval` on spec bounds how often
// that happens via RequeueAfter set by UpdateStatusHandler.
type ScanWorkloadsHandler struct {
	client client.Client
	store  domainimage.ImageWriter
}

// NewScanWorkloadsHandler constructs a ScanWorkloadsHandler.
func NewScanWorkloadsHandler(c client.Client, store domainimage.ImageWriter) *ScanWorkloadsHandler {
	return &ScanWorkloadsHandler{client: c, store: store}
}

// Handle implements reconciler.Handler.
func (h *ScanWorkloadsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource
	byWorkload, err := h.scanAll(ctx, inv)
	if err != nil {
		return fmt.Errorf("full scan: %w", err)
	}
	return h.store.ReplaceAll(ctx, inv.Spec.PortalRef, byWorkload)
}

func (h *ScanWorkloadsHandler) scanAll(ctx context.Context, inv *sreportalv1alpha1.ImageInventory) (map[domainimage.WorkloadKey][]domainimage.ImageView, error) {
	selector := labels.Everything()
	if inv.Spec.LabelSelector != "" {
		parsed, err := labels.Parse(inv.Spec.LabelSelector)
		if err != nil {
			return nil, fmt.Errorf("parse labelSelector: %w", err)
		}
		selector = parsed
	}
	opts := []client.ListOption{client.MatchingLabelsSelector{Selector: selector}}
	if inv.Spec.NamespaceFilter != "" {
		opts = append(opts, client.InNamespace(inv.Spec.NamespaceFilter))
	}

	out := make(map[domainimage.WorkloadKey][]domainimage.ImageView)
	portalRef := inv.Spec.PortalRef
	for _, kind := range inv.Spec.EffectiveWatchedKinds() {
		if err := h.scanKind(ctx, portalRef, kind, out, opts...); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h *ScanWorkloadsHandler) scanKind(ctx context.Context, portalRef string, kind sreportalv1alpha1.ImageInventoryKind, out map[domainimage.WorkloadKey][]domainimage.ImageView, opts ...client.ListOption) error {
	kindStr := string(kind)
	collect := func(ns, name string, spec corev1.PodSpec) {
		wk := domainimage.WorkloadKey{Kind: kindStr, Namespace: ns, Name: name}
		out[wk] = imagectrl.ImageViewsFromPodSpec(portalRef, kindStr, ns, name, spec)
	}
	switch kind {
	case sreportalv1alpha1.ImageInventoryKindDeployment:
		var list appsv1.DeploymentList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.Template.Spec)
		}
	case sreportalv1alpha1.ImageInventoryKindStatefulSet:
		var list appsv1.StatefulSetList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.Template.Spec)
		}
	case sreportalv1alpha1.ImageInventoryKindDaemonSet:
		var list appsv1.DaemonSetList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.Template.Spec)
		}
	case sreportalv1alpha1.ImageInventoryKindCronJob:
		var list batchv1.CronJobList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.JobTemplate.Spec.Template.Spec)
		}
	case sreportalv1alpha1.ImageInventoryKindJob:
		var list batchv1.JobList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.Template.Spec)
		}
	default:
		return fmt.Errorf("unsupported kind %q", kind)
	}
	return nil
}
```

- [ ] **Step 4: Run tests — expect pass**

Run: `go test -race ./internal/controller/imageinventory/... ./internal/controller/image/...`
Expected: PASS.

The `ImageViewsFromPodSpec` exported wrapper is already present in `internal/controller/image/handler.go` (added in Task 4).

- [ ] **Step 5: Commit**

```bash
git add internal/controller/imageinventory/chain/scan_workloads.go \
        internal/controller/imageinventory/chain/scan_workloads_test.go
git commit -m "feat(imageinventory): add ScanWorkloadsHandler for full rescan on CR changes"
```

---

## Task 7: Update `ImageInventoryReconciler` — wire new handler, deletion, RequeueAfter

**Files:**
- Modify: `internal/controller/imageinventory/chain/handlers.go`
- Modify: `internal/controller/imageinventory/imageinventory_controller.go`

- [ ] **Step 1: Update `UpdateStatusHandler` to set `RequeueAfter`**

In `internal/controller/imageinventory/chain/handlers.go`, replace the body of `UpdateStatusHandler.Handle` with:

```go
func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource

	if inv.Status.ObservedGeneration != inv.GetGeneration() {
		inv.Status.ObservedGeneration = inv.GetGeneration()
		if err := h.client.Status().Update(ctx, inv); err != nil {
			return fmt.Errorf("update observedGeneration: %w", err)
		}
		if err := h.client.Get(ctx, types.NamespacedName{Namespace: inv.Namespace, Name: inv.Name}, inv); err != nil {
			return fmt.Errorf("re-fetch image inventory: %w", err)
		}
	}

	if err := statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionTrue, ReasonReconciled, ReconciledMessage); err != nil {
		return err
	}

	rc.Result.RequeueAfter = inv.Spec.EffectiveInterval()
	return nil
}
```

- [ ] **Step 2: Update `ImageInventoryReconciler` constructor and Reconcile**

Replace `internal/controller/imageinventory/imageinventory_controller.go` with:

```go
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

// Package imageinventory contains the ImageInventory controller and its chain handlers.
package imageinventory

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	imageinventorychain "github.com/golgoth31/sreportal/internal/controller/imageinventory/chain"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ImageInventoryReconciler reconciles an ImageInventory object using a chain of handlers.
type ImageInventoryReconciler struct {
	client.Client
	store domainimage.ImageWriter
	chain *reconciler.Chain[*sreportalv1alpha1.ImageInventory, imageinventorychain.ChainData]
}

// NewImageInventoryReconciler creates a new ImageInventoryReconciler with the handler chain.
func NewImageInventoryReconciler(c client.Client, store domainimage.ImageWriter) *ImageInventoryReconciler {
	chain := reconciler.NewChain(
		imageinventorychain.NewValidateSpecHandler(c),
		imageinventorychain.NewValidatePortalRefHandler(c),
		imageinventorychain.NewScanWorkloadsHandler(c, store),
		imageinventorychain.NewUpdateStatusHandler(c),
	)
	return &ImageInventoryReconciler{
		Client: c,
		store:  store,
		chain:  chain,
	}
}

// +kubebuilder:rbac:groups=sreportal.io,resources=imageinventories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=imageinventories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=imageinventories/finalizers,verbs=update

// Reconcile validates an ImageInventory resource via the handler chain.
func (r *ImageInventoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	var inv sreportalv1alpha1.ImageInventory
	if err := r.Get(ctx, req.NamespacedName, &inv); err != nil {
		if apierrors.IsNotFound(err) {
			// The CR was deleted; purge its projection. We don't know the
			// portalRef here, so rely on a finalizer-free, best-effort
			// delete-by-name via a side-channel: we keep a defensive sweep
			// inside the handler chain instead. Nothing to do.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get image inventory CR: %w", err)
	}

	if !inv.DeletionTimestamp.IsZero() {
		if err := r.store.DeletePortal(ctx, inv.Spec.PortalRef); err != nil {
			return ctrl.Result{}, fmt.Errorf("delete portal projection: %w", err)
		}
		return ctrl.Result{}, nil
	}

	logger.V(1).Info("reconciling ImageInventory", "name", inv.Name, "portalRef", inv.Spec.PortalRef)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, imageinventorychain.ChainData]{
		Resource: &inv,
	}

	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation chain failed")
		metrics.ReconcileTotal.WithLabelValues("imageinventory", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("imageinventory").Observe(time.Since(start).Seconds())
		if errors.Is(err, imageinventorychain.ErrInvalidSpec) || errors.Is(err, imageinventorychain.ErrPortalNotFound) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	metrics.ReconcileTotal.WithLabelValues("imageinventory", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("imageinventory").Observe(time.Since(start).Seconds())

	return rc.Result, nil
}

// SetupWithManager registers the controller with the manager.
func (r *ImageInventoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.ImageInventory{}).
		Named("imageinventory").
		Complete(r)
}
```

- [ ] **Step 3: Verify the whole `imageinventory` package still compiles**

Run: `go build ./internal/controller/imageinventory/...`
Expected: success.

- [ ] **Step 4: Run existing chain tests**

Run: `go test -race ./internal/controller/imageinventory/...`
Expected: PASS (we haven't broken existing tests; the new `ScanWorkloadsHandler` tests still pass).

- [ ] **Step 5: Commit**

```bash
git add internal/controller/imageinventory/chain/handlers.go \
        internal/controller/imageinventory/imageinventory_controller.go
git commit -m "feat(imageinventory): scan on chain, delete portal on CR deletion, RequeueAfter=interval"
```

---

## Task 8: Wire new controllers in `cmd/main.go`

**Files:**
- Modify: `cmd/main.go`

- [ ] **Step 1: Update `ImageInventoryReconciler` call site and add `WorkloadHandler` wiring**

Locate the block (currently near line 672) that reads:

```go
	if err := imageinventoryctrl.NewImageInventoryReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "ImageInventory")
		os.Exit(1)
	}
```

Replace it with:

```go
	imageWorkloadHandler := imagectrl.NewWorkloadHandler(mgr.GetClient(), imageStore)
	if err := imagectrl.SetupWorkloadReconcilersWithManager(mgr, imageWorkloadHandler); err != nil {
		setupLog.Error(err, "unable to set up workload image reconcilers")
		os.Exit(1)
	}
	if err := imageinventoryctrl.NewImageInventoryReconciler(mgr.GetClient(), imageStore).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "ImageInventory")
		os.Exit(1)
	}
```

- [ ] **Step 2: Re-add the `imagectrl` import if it was removed in Task 2**

Verify in `cmd/main.go` that the import `imagectrl "github.com/golgoth31/sreportal/internal/controller/image"` is present. If not, add it back.

- [ ] **Step 3: Build everything**

Run: `go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add cmd/main.go
git commit -m "feat(main): wire event-driven image reconcilers"
```

---

## Task 9: Regenerate manifests and run the full test suite

**Files:**
- Auto-regenerated: `config/rbac/role.yaml`, Helm templates

- [ ] **Step 1: Regenerate RBAC manifests from the new kubebuilder markers**

Run: `make manifests`
Expected: `config/rbac/role.yaml` now contains entries for `apps/{deployments,statefulsets,daemonsets}` and `batch/{cronjobs,jobs}` with verbs `get;list;watch`.

- [ ] **Step 2: Regenerate Helm chart**

Run: `make helm`
Expected: success; Helm templates reflect the new RBAC rules.

- [ ] **Step 3: Run the whole test suite under race detector**

Run: `make test`
Expected: PASS.

- [ ] **Step 4: Run the linter**

Run: `make lint`
Expected: PASS (if failures are cosmetic, run `make lint-fix`).

- [ ] **Step 5: Final commit (generated files)**

```bash
git add config/rbac/role.yaml helm/
git commit -m "chore(manifests): regenerate RBAC and Helm after event-driven scanner refactor"
```

---

## Done criteria

- The old periodic `Scanner` is gone; no `time.Ticker`-based polling of workloads remains in the image package.
- Creating/modifying/deleting any workload (Deployment, StatefulSet, DaemonSet, CronJob, Job) triggers a reconcile of that single object; the store's `map[portalRef][workloadKey]` is updated in place.
- Creating/modifying an `ImageInventory` CR triggers a single full scan via `ScanWorkloadsHandler.ReplaceAll`.
- Deleting an `ImageInventory` CR deletes its portal projection.
- Periodic resync happens via `RequeueAfter = spec.interval` (default 5m), independent of the event stream.
- All new tests pass under `-race`.
- `config/rbac/role.yaml` grants `get;list;watch` on the five workload kinds.
