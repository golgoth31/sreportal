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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	testDeployWorkflow = "deploy.yml"
	testRunURL         = "https://github.com/acme/app-a/actions/runs/42"
)

// workflowTrackingFake records LatestWorkflowRun invocations so tests can
// assert which keys were enriched (and which were skipped).
type workflowTrackingFake struct {
	runURL    string
	runErr    error
	callCount int
}

func (f *workflowTrackingFake) DefaultBranch(_ context.Context, _ forge.RepoRef) (string, error) {
	return testDefaultBranch, nil
}

func (f *workflowTrackingFake) Compare(_ context.Context, _ forge.RepoRef, _, _ string) (forge.CompareResult, error) {
	return forge.CompareResult{}, nil
}

func (f *workflowTrackingFake) LatestWorkflowRun(_ context.Context, _ forge.RepoRef, _, _ string) (string, error) {
	f.callCount++
	return f.runURL, f.runErr
}

func newDeployRunRC(computed []ComputedEntry, due []WorkItem) *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData] {
	return &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: &sreportalv1alpha1.DeployStatus{ObjectMeta: metav1.ObjectMeta{Name: "ds-run"}},
		Data: ChainData{
			Due:      due,
			Computed: computed,
		},
	}
}

// TestResolveDeployRun_EnrichesOkAndBehindEntries verifies that entries with
// state ok or behind get their DeployRunURL populated from LatestWorkflowRun.
func TestResolveDeployRun_EnrichesOkAndBehindEntries(t *testing.T) {
	fc := &workflowTrackingFake{runURL: testRunURL}
	clientFor := func(string) forge.Client { return fc }

	forges := []config.ForgeConfig{{Host: testForgeHost, DeployWorkflow: testDeployWorkflow}}
	h := NewResolveDeployRunHandler(clientFor, forges)

	rc := newDeployRunRC(
		[]ComputedEntry{
			{Key: testKeyA, State: stateBehind, DefaultBranch: testDefaultBranch},
			{Key: testKeyB, State: stateOk, DefaultBranch: testDefaultBranch},
		},
		[]WorkItem{
			{Key: testKeyA, Workload: forge.RepoRef{Host: testForgeHost, Owner: testRepoOwner, Repo: testRepoNameA}},
			{Key: testKeyB, Workload: forge.RepoRef{Host: testForgeHost, Owner: testRepoOwner, Repo: "app-b"}},
		},
	)

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if fc.callCount != 2 {
		t.Errorf("LatestWorkflowRun called %d times, want 2 (one per ok/behind entry)", fc.callCount)
	}
	for i, e := range rc.Data.Computed {
		if e.DeployRunURL != testRunURL {
			t.Errorf("Computed[%d].DeployRunURL = %q, want %q", i, e.DeployRunURL, testRunURL)
		}
	}
}

// TestResolveDeployRun_SkipsErrorAndUnresolvedEntries verifies that entries
// with state error or unresolved are never passed to LatestWorkflowRun — the
// forge client must not be called for them.
func TestResolveDeployRun_SkipsErrorAndUnresolvedEntries(t *testing.T) {
	fc := &workflowTrackingFake{runURL: testRunURL}
	clientFor := func(string) forge.Client { return fc }

	forges := []config.ForgeConfig{{Host: testForgeHost, DeployWorkflow: testDeployWorkflow}}
	h := NewResolveDeployRunHandler(clientFor, forges)

	rc := newDeployRunRC(
		[]ComputedEntry{
			{Key: testErrKey, State: stateError},
			{Key: testUnresKey, State: stateUnresolved},
		},
		[]WorkItem{
			{Key: testErrKey, Workload: forge.RepoRef{Host: testForgeHost, Owner: testRepoOwner, Repo: "app-c"}},
			{Key: testUnresKey, Workload: forge.RepoRef{Host: testForgeHost, Owner: testRepoOwner, Repo: "app-d"}},
		},
	)

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if fc.callCount != 0 {
		t.Errorf("LatestWorkflowRun called %d times for error/unresolved entries, want 0", fc.callCount)
	}
	for _, e := range rc.Data.Computed {
		if e.DeployRunURL != "" {
			t.Errorf("entry %q DeployRunURL = %q, want empty (should not be enriched)", e.Key, e.DeployRunURL)
		}
	}
}

// TestResolveDeployRun_SwallowsLatestWorkflowRunError verifies that a
// LatestWorkflowRun error does not propagate as a chain error — the entry's
// DeployRunURL stays empty and Handle returns nil.
func TestResolveDeployRun_SwallowsLatestWorkflowRunError(t *testing.T) {
	fc := &workflowTrackingFake{runErr: errors.New("GitHub rate-limited")}
	clientFor := func(string) forge.Client { return fc }

	forges := []config.ForgeConfig{{Host: testForgeHost, DeployWorkflow: testDeployWorkflow}}
	h := NewResolveDeployRunHandler(clientFor, forges)

	rc := newDeployRunRC(
		[]ComputedEntry{
			{Key: testKeyA, State: stateBehind, DefaultBranch: testDefaultBranch},
		},
		[]WorkItem{
			{Key: testKeyA, Workload: forge.RepoRef{Host: testForgeHost, Owner: testRepoOwner, Repo: testRepoNameA}},
		},
	)

	err := h.Handle(context.Background(), rc)
	if err != nil {
		t.Fatalf("Handle must return nil even when LatestWorkflowRun fails, got: %v", err)
	}

	if rc.Data.Computed[0].DeployRunURL != "" {
		t.Errorf("DeployRunURL = %q, want empty (error swallowed)", rc.Data.Computed[0].DeployRunURL)
	}
}

// TestResolveDeployRun_MixedStates verifies the full mixed-state scenario:
// ok and behind entries are enriched, error and unresolved entries are skipped.
func TestResolveDeployRun_MixedStates(t *testing.T) {
	fc := &workflowTrackingFake{runURL: testRunURL}
	clientFor := func(string) forge.Client { return fc }

	forges := []config.ForgeConfig{{Host: testForgeHost, DeployWorkflow: testDeployWorkflow}}
	h := NewResolveDeployRunHandler(clientFor, forges)

	rc := newDeployRunRC(
		[]ComputedEntry{
			{Key: "ok-key", State: stateOk, DefaultBranch: testDefaultBranch},
			{Key: "behind-key", State: stateBehind, DefaultBranch: testDefaultBranch},
			{Key: testErrKey, State: stateError},
			{Key: testUnresKey, State: stateUnresolved},
		},
		[]WorkItem{
			{Key: "ok-key", Workload: forge.RepoRef{Host: testForgeHost, Owner: testRepoOwner, Repo: "ok"}},
			{Key: "behind-key", Workload: forge.RepoRef{Host: testForgeHost, Owner: testRepoOwner, Repo: "behind"}},
			{Key: testErrKey, Workload: forge.RepoRef{Host: testForgeHost, Owner: testRepoOwner, Repo: "err"}},
			{Key: testUnresKey, Workload: forge.RepoRef{Host: testForgeHost, Owner: testRepoOwner, Repo: "unres"}},
		},
	)

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// Only ok and behind entries should trigger a workflow run lookup.
	if fc.callCount != 2 {
		t.Errorf("LatestWorkflowRun called %d times, want 2 (ok + behind only)", fc.callCount)
	}

	byKey := make(map[string]ComputedEntry, len(rc.Data.Computed))
	for _, e := range rc.Data.Computed {
		byKey[e.Key] = e
	}

	if byKey["ok-key"].DeployRunURL != testRunURL {
		t.Errorf("ok-key DeployRunURL = %q, want %q", byKey["ok-key"].DeployRunURL, testRunURL)
	}
	if byKey["behind-key"].DeployRunURL != testRunURL {
		t.Errorf("behind-key DeployRunURL = %q, want %q", byKey["behind-key"].DeployRunURL, testRunURL)
	}
	if byKey[testErrKey].DeployRunURL != "" {
		t.Errorf("err-key DeployRunURL = %q, want empty", byKey[testErrKey].DeployRunURL)
	}
	if byKey[testUnresKey].DeployRunURL != "" {
		t.Errorf("unres-key DeployRunURL = %q, want empty", byKey[testUnresKey].DeployRunURL)
	}
}
