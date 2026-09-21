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

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	dom "github.com/golgoth31/sreportal/internal/domain/deploystatus"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// fakeWriter is a test double for dom.Writer that captures the last
// ReplaceForNamespace call for assertion.
type fakeWriter struct {
	lastPortalRef string
	lastNamespace string
	lastEntries   []dom.Entry
	callCount     int
}

func (f *fakeWriter) ReplaceForNamespace(portalRef, namespace string, entries []dom.Entry) {
	f.lastPortalRef = portalRef
	f.lastNamespace = namespace
	f.lastEntries = entries
	f.callCount++
}

func (f *fakeWriter) RemoveForNamespace(_, _ string) {}

// TestUpdateReadStore_MapsStatusEntries verifies that every CRD DeployStatusEntry
// field in Status.Services maps correctly to dom.Entry — including the full
// Workload ref, DeployedAt/LastCheckedAt times, and the Sha mapping.
func TestUpdateReadStore_MapsStatusEntries(t *testing.T) {
	commitDate := time.Date(2026, 6, 19, 9, 0, 0, 0, time.UTC)
	checkedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	deployedAt := time.Date(2026, 6, 18, 8, 0, 0, 0, time.UTC)

	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "ds-map", Namespace: testOperatorNs},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: testPortalRef,
			Namespace: testNamespace,
		},
		Status: sreportalv1alpha1.DeployStatusStatus{
			Services: []sreportalv1alpha1.DeployStatusEntry{
				{
					Key: testKeyA,
					Workload: sreportalv1alpha1.DeployStatusWorkloadRef{
						Kind:      testKindDeployment,
						Namespace: testNamespace,
						Name:      testRepoNameA,
						Container: testWorkloadApp,
					},
					Image:         testImageA,
					SourceRepo:    testSourceRepoA,
					DeployedRef:   testDeployedSHA,
					DefaultBranch: testDefaultBranch,
					AheadBy:       2,
					PendingCommits: []sreportalv1alpha1.DeployStatusCommit{
						{
							Sha:     testCommitSHA,
							Message: "feat: something",
							Author:  "alice",
							Date:    metav1.NewTime(commitDate),
							URL:     "https://github.com/" + testRepoOwner + "/app-a/commit/" + testCommitSHA,
						},
					},
					PendingTruncated: true,
					DeployedAt:       metav1.NewTime(deployedAt),
					DeployRunURL:     "https://github.com/" + testRepoOwner + "/app-a/actions/runs/42",
					State:            stateBehind,
					Error:            "",
					LastCheckedAt:    metav1.NewTime(checkedAt),
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

	writer := &fakeWriter{}
	h := NewUpdateReadStoreHandler(writer)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: cr,
	}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// Exactly one ReplaceForNamespace call.
	if writer.callCount != 1 {
		t.Fatalf("ReplaceForNamespace called %d times, want 1", writer.callCount)
	}

	// Verify (portalRef, namespace) args.
	if writer.lastPortalRef != testPortalRef {
		t.Errorf("portalRef = %q, want %q", writer.lastPortalRef, testPortalRef)
	}
	if writer.lastNamespace != testNamespace {
		t.Errorf("namespace = %q, want %q", writer.lastNamespace, testNamespace)
	}

	// Verify entry count.
	if len(writer.lastEntries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(writer.lastEntries))
	}

	// Locate entry A and verify field mapping.
	var entryA dom.Entry
	for _, e := range writer.lastEntries {
		if e.Key == testKeyA {
			entryA = e
		}
	}
	if entryA.Key == "" {
		t.Fatal("entry A not found in readstore entries")
	}
	if entryA.Workload.Kind != testKindDeployment || entryA.Workload.Name != testRepoNameA ||
		entryA.Workload.Namespace != testNamespace || entryA.Workload.Container != testWorkloadApp {
		t.Errorf("entryA.Workload = %+v, want full workload ref", entryA.Workload)
	}
	if entryA.AheadBy != 2 {
		t.Errorf("entryA.AheadBy = %d, want 2", entryA.AheadBy)
	}
	if !entryA.PendingTruncated {
		t.Error("entryA.PendingTruncated = false, want true")
	}
	if !entryA.DeployedAt.Equal(deployedAt) {
		t.Errorf("entryA.DeployedAt = %v, want %v", entryA.DeployedAt, deployedAt)
	}
	if !entryA.LastCheckedAt.Equal(checkedAt) {
		t.Errorf("entryA.LastCheckedAt = %v, want %v", entryA.LastCheckedAt, checkedAt)
	}
	wantDeployRunURL := "https://github.com/" + testRepoOwner + "/app-a/actions/runs/42"
	if entryA.DeployRunURL != wantDeployRunURL {
		t.Errorf("entryA.DeployRunURL = %q, want %q", entryA.DeployRunURL, wantDeployRunURL)
	}
	if entryA.State != stateBehind {
		t.Errorf("entryA.State = %q, want behind", entryA.State)
	}

	// DeployStatusCommit.Sha must map to dom.Commit.Sha.
	if len(entryA.PendingCommits) != 1 {
		t.Fatalf("entryA.PendingCommits len = %d, want 1", len(entryA.PendingCommits))
	}
	domCommit := entryA.PendingCommits[0]
	if domCommit.Sha != testCommitSHA {
		t.Errorf("dom commit Sha = %q, want %q", domCommit.Sha, testCommitSHA)
	}
	if domCommit.Message != "feat: something" {
		t.Errorf("dom commit Message = %q", domCommit.Message)
	}
	if domCommit.Author != "alice" {
		t.Errorf("dom commit Author = %q", domCommit.Author)
	}
	if !domCommit.Date.Equal(commitDate) {
		t.Errorf("dom commit Date = %v, want %v", domCommit.Date, commitDate)
	}
	wantCommitURL := "https://github.com/" + testRepoOwner + "/app-a/commit/" + testCommitSHA
	if domCommit.URL != wantCommitURL {
		t.Errorf("dom commit URL = %q, want %q", domCommit.URL, wantCommitURL)
	}
}

// TestUpdateReadStore_NothingDuePublishesFullStatus is the regression test for
// the readstore-wipe bug: a "nothing computed this cycle" reconcile must still
// publish the FULL prior Status.Services (self-healing) instead of wiping the
// store. ChainData.Computed is empty here, but Status.Services holds the prior
// full set (preserved by UpdateStatusHandler's early return).
func TestUpdateReadStore_NothingDuePublishesFullStatus(t *testing.T) {
	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "ds-heal", Namespace: testOperatorNs},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: testPortalRef,
			Namespace: testNamespace,
		},
		Status: sreportalv1alpha1.DeployStatusStatus{
			Services: []sreportalv1alpha1.DeployStatusEntry{
				{Key: testKeyA, Image: testImageA, State: stateBehind},
				{Key: testKeyB, Image: testImageB, State: stateOk},
			},
		},
	}

	writer := &fakeWriter{}
	h := NewUpdateReadStoreHandler(writer)

	// Nothing computed this cycle.
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: cr,
		Data:     ChainData{Computed: nil},
	}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if writer.callCount != 1 {
		t.Fatalf("ReplaceForNamespace called %d times, want 1", writer.callCount)
	}
	// The full prior set must be republished — NOT wiped.
	if len(writer.lastEntries) != 2 {
		t.Fatalf("entries len = %d, want 2 (full Status republished, not wiped)", len(writer.lastEntries))
	}
}

// TestUpdateReadStore_EmptyStatusCallsReplaceWithEmpty verifies that with an
// empty Status.Services (genuinely no services) the store is updated with an
// empty set.
func TestUpdateReadStore_EmptyStatusCallsReplaceWithEmpty(t *testing.T) {
	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "ds-empty-rs", Namespace: testOperatorNs},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: testPortalRef,
			Namespace: testNamespace,
		},
	}

	writer := &fakeWriter{}
	h := NewUpdateReadStoreHandler(writer)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: cr,
	}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if writer.callCount != 1 {
		t.Fatalf("ReplaceForNamespace called %d times, want 1", writer.callCount)
	}
	if len(writer.lastEntries) != 0 {
		t.Errorf("entries len = %d, want 0", len(writer.lastEntries))
	}
}

// TestUpdateReadStore_NilCommitsProducesNilDomCommits verifies that a Status
// entry with no PendingCommits maps to a dom.Entry with nil (not empty slice)
// PendingCommits.
func TestUpdateReadStore_NilCommitsProducesNilDomCommits(t *testing.T) {
	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "ds-nocommits", Namespace: testOperatorNs},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: testPortalRef,
			Namespace: testNamespace,
		},
		Status: sreportalv1alpha1.DeployStatusStatus{
			Services: []sreportalv1alpha1.DeployStatusEntry{
				{Key: testKeyA, State: stateOk, PendingCommits: nil},
			},
		},
	}

	writer := &fakeWriter{}
	h := NewUpdateReadStoreHandler(writer)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: cr,
	}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(writer.lastEntries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(writer.lastEntries))
	}
	if writer.lastEntries[0].PendingCommits != nil {
		t.Errorf("PendingCommits = %v, want nil for zero commits", writer.lastEntries[0].PendingCommits)
	}
}
