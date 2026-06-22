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
	"github.com/golgoth31/sreportal/internal/domain/forge"
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

// TestUpdateReadStore_MapsComputedEntries verifies that ComputedEntry fields map
// correctly to dom.Entry, specifically forge.Commit.SHA → dom.Commit.Sha,
// PendingTruncated, and all scalar fields.
func TestUpdateReadStore_MapsComputedEntries(t *testing.T) {
	commitDate := time.Date(2026, 6, 19, 9, 0, 0, 0, time.UTC)
	commitMetaDate := metav1.NewTime(commitDate)

	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "ds-map", Namespace: testOperatorNs},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: testPortalRef,
			Namespace: testNamespace,
		},
	}

	writer := &fakeWriter{}
	h := NewUpdateReadStoreHandler(writer)

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
					AheadBy:       2,
					PendingCommits: []forge.Commit{
						{
							SHA:     testCommitSHA,
							Message: "feat: something",
							Author:  "alice",
							Date:    commitMetaDate.Time,
							URL:     "https://github.com/" + testRepoOwner + "/app-a/commit/" + testCommitSHA,
						},
					},
					PendingTrunc: true,
					DeployRunURL: "https://github.com/" + testRepoOwner + "/app-a/actions/runs/42",
					State:        stateBehind,
					Error:        "",
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
	if entryA.AheadBy != 2 {
		t.Errorf("entryA.AheadBy = %d, want 2", entryA.AheadBy)
	}
	if !entryA.PendingTruncated {
		t.Error("entryA.PendingTruncated = false, want true")
	}
	wantDeployRunURL := "https://github.com/" + testRepoOwner + "/app-a/actions/runs/42"
	if entryA.DeployRunURL != wantDeployRunURL {
		t.Errorf("entryA.DeployRunURL = %q, want %q", entryA.DeployRunURL, wantDeployRunURL)
	}
	if entryA.State != stateBehind {
		t.Errorf("entryA.State = %q, want behind", entryA.State)
	}

	// forge.Commit.SHA must map to dom.Commit.Sha (case change is the key contract).
	if len(entryA.PendingCommits) != 1 {
		t.Fatalf("entryA.PendingCommits len = %d, want 1", len(entryA.PendingCommits))
	}
	domCommit := entryA.PendingCommits[0]
	if domCommit.Sha != testCommitSHA {
		t.Errorf("dom commit Sha = %q, want %q (SHA→Sha mapping)", domCommit.Sha, testCommitSHA)
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

// TestUpdateReadStore_EmptyComputedCallsReplaceWithEmpty verifies that even
// with no computed entries the store is still updated (clearing the previous
// cycle's stale data).
func TestUpdateReadStore_EmptyComputedCallsReplaceWithEmpty(t *testing.T) {
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
		Data:     ChainData{Computed: nil},
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

// TestUpdateReadStore_NilCommitsProducesNilDomCommits verifies that a
// ComputedEntry with no PendingCommits maps to a dom.Entry with nil
// (not empty slice) PendingCommits.
func TestUpdateReadStore_NilCommitsProducesNilDomCommits(t *testing.T) {
	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "ds-nocommits", Namespace: testOperatorNs},
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: testPortalRef,
			Namespace: testNamespace,
		},
	}

	writer := &fakeWriter{}
	h := NewUpdateReadStoreHandler(writer)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: cr,
		Data: ChainData{
			Computed: []ComputedEntry{
				{Key: testKeyA, State: stateOk, PendingCommits: nil},
			},
		},
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
