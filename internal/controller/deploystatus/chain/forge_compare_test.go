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
	"errors"
	"testing"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// fakeForgeClient is a test double for forge.Client that uses the real RepoRef-based API.
type fakeForgeClient struct {
	branch string
	cmp    forge.CompareResult
	runURL string
	err    error
}

func (f *fakeForgeClient) DefaultBranch(_ context.Context, ref forge.RepoRef) (string, error) {
	return f.branch, f.err
}

func (f *fakeForgeClient) Compare(_ context.Context, ref forge.RepoRef, base, head string) (forge.CompareResult, error) {
	return f.cmp, f.err
}

func (f *fakeForgeClient) LatestWorkflowRun(_ context.Context, ref forge.RepoRef, workflowFile, branch string) (string, error) {
	return f.runURL, nil
}

func TestForgeCompare_ProducesBehindEntry(t *testing.T) {
	fc := &fakeForgeClient{
		branch: testDefaultBranch,
		cmp: forge.CompareResult{
			AheadBy: 2,
			Commits: []forge.Commit{
				{SHA: "c1", Message: "feat: first"},
				{SHA: "c2", Message: "feat: second"},
			},
		},
	}
	h := NewForgeCompareHandler(func(string) forge.Client { return fc })
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: &sreportalv1alpha1.DeployStatus{},
		Data: ChainData{Due: []WorkItem{
			{
				Key:         "b",
				DeployedRef: "v1",
				Workload:    forge.RepoRef{Host: testForgeHost, Owner: "o", Repo: "r"},
			},
		}},
	}
	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatal(err)
	}
	if len(rc.Data.Computed) != 1 {
		t.Fatalf("computed = %d, want 1", len(rc.Data.Computed))
	}
	e := rc.Data.Computed[0]
	if e.State != stateBehind {
		t.Errorf("state = %q, want behind", e.State)
	}
	if e.AheadBy != 2 {
		t.Errorf("aheadBy = %d, want 2", e.AheadBy)
	}
	if e.DefaultBranch != testDefaultBranch {
		t.Errorf("defaultBranch = %q, want main", e.DefaultBranch)
	}
	if len(e.PendingCommits) != 2 {
		t.Errorf("pendingCommits = %d, want 2", len(e.PendingCommits))
	}
	if e.PendingTrunc {
		t.Error("expected PendingTrunc = false")
	}
}

func TestForgeCompare_OkStateWhenNotBehind(t *testing.T) {
	fc := &fakeForgeClient{
		branch: testDefaultBranch,
		cmp:    forge.CompareResult{AheadBy: 0, Commits: nil},
	}
	h := NewForgeCompareHandler(func(string) forge.Client { return fc })
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: &sreportalv1alpha1.DeployStatus{},
		Data: ChainData{Due: []WorkItem{
			{Key: "x", Workload: forge.RepoRef{Host: testForgeHost, Owner: "o", Repo: "r"}},
		}},
	}
	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatal(err)
	}
	if rc.Data.Computed[0].State != "ok" {
		t.Errorf("state = %q, want ok", rc.Data.Computed[0].State)
	}
}

func TestForgeCompare_ErrorMarksErrorState_DoesNotFailChain(t *testing.T) {
	fc := &fakeForgeClient{err: errors.New("forge timeout")}
	h := NewForgeCompareHandler(func(string) forge.Client { return fc })
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: &sreportalv1alpha1.DeployStatus{},
		Data: ChainData{Due: []WorkItem{
			{Key: "b", Workload: forge.RepoRef{Host: testForgeHost, Owner: "o", Repo: "r"}},
		}},
	}
	// The handler MUST return nil even on per-entry errors.
	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("handler must not fail the chain on per-entry error: %v", err)
	}
	if len(rc.Data.Computed) != 1 {
		t.Fatalf("expected one computed entry even on error, got %d", len(rc.Data.Computed))
	}
	if rc.Data.Computed[0].State != stateError {
		t.Errorf("state = %q, want error", rc.Data.Computed[0].State)
	}
	if rc.Data.Computed[0].Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestForgeCompare_CompareErrorMarksErrorState(t *testing.T) {
	// DefaultBranch succeeds but Compare fails.
	callCount := 0
	h := NewForgeCompareHandler(func(string) forge.Client {
		return &callTrackingFake{onCall: &callCount}
	})
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: &sreportalv1alpha1.DeployStatus{},
		Data: ChainData{Due: []WorkItem{
			{Key: "z", Workload: forge.RepoRef{Host: testForgeHost, Owner: "o", Repo: "r"}},
		}},
	}
	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("handler must not fail the chain: %v", err)
	}
	if rc.Data.Computed[0].State != stateError {
		t.Errorf("state = %q, want error", rc.Data.Computed[0].State)
	}
}

// callTrackingFake returns "main" for DefaultBranch and an error for Compare.
type callTrackingFake struct {
	onCall *int
}

func (f *callTrackingFake) DefaultBranch(_ context.Context, _ forge.RepoRef) (string, error) {
	return testDefaultBranch, nil
}

func (f *callTrackingFake) Compare(_ context.Context, _ forge.RepoRef, _, _ string) (forge.CompareResult, error) {
	return forge.CompareResult{}, errors.New("compare failed")
}

func (f *callTrackingFake) LatestWorkflowRun(_ context.Context, _ forge.RepoRef, _, _ string) (string, error) {
	return "", nil
}
