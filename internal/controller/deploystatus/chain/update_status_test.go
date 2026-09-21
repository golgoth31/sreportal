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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	testPortalRef   = "portal-a"
	testNamespace   = "default"
	testKeyA        = "svc-a"
	testKeyB        = "svc-b"
	testImageA      = "registry.io/app-a:sha256-aaa"
	testImageB      = "registry.io/app-b:sha256-bbb"
	testSourceRepoA = "https://github.com/acme/app-a"
	testSourceRepoB = "https://github.com/acme/app-b"
	testDeployedSHA = "deadbeef"
)

func newSchemeForChain(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := sreportalv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	return s
}

// fixedNow returns a metav1.Time factory pinned to a fixed instant so tests
// can assert exact timestamps without races.
func fixedNow(ts time.Time) func() metav1.Time {
	return func() metav1.Time { return metav1.NewTime(ts) }
}

// TestUpdateStatus_WritesComputedToStatus verifies that Handle projects
// rc.Data.Computed into cr.Status.Services via client.Status().Update, setting
// state, aheadBy, and lastCheckedAt correctly.
func TestUpdateStatus_WritesComputedToStatus(t *testing.T) {
	scheme := newSchemeForChain(t)
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)

	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "ds-a", Namespace: testOperatorNs},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: testPortalRef,
			Namespace: testNamespace,
			Services: []sreportalv1alpha1.DeployStatusEntry{
				{Key: testKeyA, Image: testImageA, SourceRepo: testSourceRepoA, DeployedRef: testDeployedSHA},
				{Key: testKeyB, Image: testImageB, SourceRepo: testSourceRepoB, DeployedRef: testDeployedSHA},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(cr).
		Build()

	h := &UpdateStatusHandler{client: cl, now: fixedNow(now)}

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: cr,
		Data: ChainData{
			Computed: []ComputedEntry{
				{
					Key:           testKeyA,
					Image:         testImageA,
					SourceRepo:    testSourceRepoA,
					DeployedRef:   testDeployedSHA,
					DefaultBranch: testDefaultBranch,
					AheadBy:       3,
					PendingCommits: []forge.Commit{
						{SHA: "c1", Message: "feat: alpha"},
						{SHA: "c2", Message: "feat: beta"},
					},
					State: stateBehind,
				},
				{
					Key:           testKeyB,
					Image:         testImageB,
					SourceRepo:    testSourceRepoB,
					DeployedRef:   testDeployedSHA,
					DefaultBranch: testDefaultBranch,
					AheadBy:       0,
					State:         stateOk,
				},
			},
		},
	}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// Spec must be UNCHANGED — observed state must never be written back to spec.
	if len(cr.Spec.Services) != 2 {
		t.Errorf("Spec.Services len = %d, want 2 (spec must not be modified)", len(cr.Spec.Services))
	}
	for _, s := range cr.Spec.Services {
		if s.State != "" {
			t.Errorf("Spec.Services[%s].State = %q, want empty (must not write computed fields to spec)", s.Key, s.State)
		}
	}

	// Status must have 2 entries.
	if len(cr.Status.Services) != 2 {
		t.Fatalf("Status.Services len = %d, want 2", len(cr.Status.Services))
	}

	// Verify entry A (behind).
	var entryA sreportalv1alpha1.DeployStatusEntry
	for _, s := range cr.Status.Services {
		if s.Key == testKeyA {
			entryA = s
		}
	}
	if entryA.State != stateBehind {
		t.Errorf("entryA.State = %q, want behind", entryA.State)
	}
	if entryA.AheadBy != 3 {
		t.Errorf("entryA.AheadBy = %d, want 3", entryA.AheadBy)
	}
	if len(entryA.PendingCommits) != 2 {
		t.Errorf("entryA.PendingCommits len = %d, want 2", len(entryA.PendingCommits))
	}
	if !entryA.LastCheckedAt.Time.Equal(now) {
		t.Errorf("entryA.LastCheckedAt = %v, want %v", entryA.LastCheckedAt.Time, now)
	}

	// ServiceCount and ObservedGeneration.
	if cr.Status.ServiceCount != 2 {
		t.Errorf("ServiceCount = %d, want 2", cr.Status.ServiceCount)
	}
	if cr.Status.ObservedGeneration != cr.Generation {
		t.Errorf("ObservedGeneration = %d, want %d", cr.Status.ObservedGeneration, cr.Generation)
	}
}

// TestUpdateStatus_PreservesPriorStatusForUncheckedService verifies that a
// service present in Status.Services but NOT in rc.Data.Computed (i.e., not
// selected for this reconcile cycle) keeps its prior status entry intact.
func TestUpdateStatus_PreservesPriorStatusForUncheckedService(t *testing.T) {
	scheme := newSchemeForChain(t)
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	prior := time.Date(2026, 6, 20, 11, 0, 0, 0, time.UTC)

	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "ds-preserve", Namespace: testOperatorNs},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: testPortalRef,
			Namespace: testNamespace,
			Services: []sreportalv1alpha1.DeployStatusEntry{
				{Key: testKeyA, Image: testImageA, SourceRepo: testSourceRepoA, DeployedRef: testDeployedSHA},
				{Key: testKeyB, Image: testImageB, SourceRepo: testSourceRepoB, DeployedRef: testDeployedSHA},
			},
		},
		Status: sreportalv1alpha1.DeployStatusStatus{
			Services: []sreportalv1alpha1.DeployStatusEntry{
				// testKeyB was checked in a prior cycle.
				{
					Key:           testKeyB,
					Image:         testImageB,
					State:         stateOk,
					AheadBy:       0,
					LastCheckedAt: metav1.NewTime(prior),
					DefaultBranch: testDefaultBranch,
				},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(cr).
		Build()

	h := &UpdateStatusHandler{client: cl, now: fixedNow(now)}

	// Only testKeyA is computed this cycle — testKeyB is not due.
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: cr,
		Data: ChainData{
			Computed: []ComputedEntry{
				{
					Key:           testKeyA,
					Image:         testImageA,
					SourceRepo:    testSourceRepoA,
					DeployedRef:   testDeployedSHA,
					DefaultBranch: testDefaultBranch,
					AheadBy:       1,
					State:         stateBehind,
				},
			},
		},
	}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// Locate testKeyB in the updated status.
	var entryB sreportalv1alpha1.DeployStatusEntry
	var found bool
	for _, s := range cr.Status.Services {
		if s.Key == testKeyB {
			entryB = s
			found = true
		}
	}
	if !found {
		t.Fatal("testKeyB must remain in Status.Services even when not computed this cycle")
	}
	if !entryB.LastCheckedAt.Time.Equal(prior) {
		t.Errorf("entryB.LastCheckedAt = %v, want preserved prior %v", entryB.LastCheckedAt.Time, prior)
	}
	if entryB.State != stateOk {
		t.Errorf("entryB.State = %q, want ok (preserved prior state)", entryB.State)
	}
}

// TestUpdateStatus_NoOpWhenComputedEmpty verifies that the handler returns nil
// and does NOT call client.Status().Update when rc.Data.Computed is empty.
func TestUpdateStatus_NoOpWhenComputedEmpty(t *testing.T) {
	scheme := newSchemeForChain(t)

	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "ds-empty", Namespace: testOperatorNs},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: testPortalRef,
			Namespace: testNamespace,
			Services:  []sreportalv1alpha1.DeployStatusEntry{{Key: testKeyA}},
		},
	}

	// Use a fake client without WithStatusSubresource so any Status().Update call
	// would succeed (we verify by checking Status.Services remains nil/empty).
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		Build()

	h := &UpdateStatusHandler{client: cl, now: fixedNow(time.Now())}

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: cr,
		Data:     ChainData{Computed: nil},
	}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle must return nil on empty Computed: %v", err)
	}

	// Status.Services must be untouched (still zero value).
	if len(cr.Status.Services) != 0 {
		t.Errorf("Status.Services len = %d, want 0 (no-op)", len(cr.Status.Services))
	}
}

// TestUpdateStatus_OnlyCallsStatusUpdate verifies that Handle uses
// client.Status().Update (not client.Update on the spec) by asserting that
// Spec.Services is unchanged after the call and Status.Services is populated.
// This is the "never write Spec" contract: writing Spec bumps Generation and
// would create a reconcile loop.
func TestUpdateStatus_OnlyCallsStatusUpdate(t *testing.T) {
	scheme := newSchemeForChain(t)

	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "ds-spec-guard", Namespace: testOperatorNs},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: testPortalRef,
			Namespace: testNamespace,
			Services: []sreportalv1alpha1.DeployStatusEntry{
				{Key: testKeyA, Image: testImageA, SourceRepo: testSourceRepoA, DeployedRef: testDeployedSHA},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(cr).
		Build()

	h := &UpdateStatusHandler{client: cl, now: fixedNow(time.Now())}

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: cr,
		Data: ChainData{
			Computed: []ComputedEntry{
				{
					Key:           testKeyA,
					DefaultBranch: testDefaultBranch,
					AheadBy:       5,
					State:         stateBehind,
				},
			},
		},
	}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// Re-fetch from the fake store to verify what was actually persisted.
	var persisted sreportalv1alpha1.DeployStatus
	key := types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}
	if err := cl.Get(context.Background(), key, &persisted); err != nil {
		t.Fatalf("Get after Handle: %v", err)
	}

	// Status must be written.
	if len(persisted.Status.Services) != 1 {
		t.Fatalf("Status.Services len = %d, want 1", len(persisted.Status.Services))
	}
	if persisted.Status.Services[0].State != stateBehind {
		t.Errorf("persisted Status state = %q, want behind", persisted.Status.Services[0].State)
	}

	// Spec must be unchanged — no computed fields written to spec.
	if persisted.Spec.Services[0].State != "" {
		t.Errorf("Spec.Services[0].State = %q, must remain empty (spec-write guard)", persisted.Spec.Services[0].State)
	}
	if persisted.Spec.Services[0].AheadBy != 0 {
		t.Errorf("Spec.Services[0].AheadBy = %d, must remain 0 (spec-write guard)", persisted.Spec.Services[0].AheadBy)
	}
}
