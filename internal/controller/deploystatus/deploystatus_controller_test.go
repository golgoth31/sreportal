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

package deploystatus

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	readstoredeploystatus "github.com/golgoth31/sreportal/internal/readstore/deploystatus"
)

// fakeForge is a forge.Client test double returning canned compare results.
type fakeForge struct {
	branch string
	cmp    forge.CompareResult
}

func (f *fakeForge) DefaultBranch(_ context.Context, _ forge.RepoRef) (string, error) {
	return f.branch, nil
}

func (f *fakeForge) Compare(_ context.Context, _ forge.RepoRef, _, _ string) (forge.CompareResult, error) {
	return f.cmp, nil
}

func (f *fakeForge) LatestWorkflowRun(_ context.Context, _ forge.RepoRef, _, _ string) (string, error) {
	return "https://github.com/acme/widget/actions/runs/1", nil
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := sreportalv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	return s
}

func objectMeta(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: namespace}
}

func TestReconcile_PopulatesStatusAndReadStore(t *testing.T) {
	scheme := newScheme(t)
	store := readstoredeploystatus.NewStore()

	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: objectMeta("portal-a-default", "sreportal"),
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: "portal-a",
			Namespace: "default",
			Services: []sreportalv1alpha1.DeployStatusEntry{
				{
					Key: "k1",
					Workload: sreportalv1alpha1.DeployStatusWorkloadRef{
						Kind: "Deployment", Namespace: "default", Name: "widget", Container: "app",
					},
					Image:       "github.com/acme/widget@sha256:abc",
					SourceRepo:  "https://github.com/acme/widget",
					DeployedRef: "deadbeef",
				},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(cr).
		Build()

	fc := &fakeForge{
		branch: "main",
		cmp: forge.CompareResult{
			AheadBy: 2,
			Commits: []forge.Commit{
				{SHA: "c1", Message: "feat: one"},
				{SHA: "c2", Message: "feat: two"},
			},
		},
	}
	clientFor := func(string) forge.Client { return fc }

	cfg := &config.DeployStatusConfig{
		Enabled: true,
		Forges:  []config.ForgeConfig{{Host: "github.com", Kind: "github"}},
	}

	r := NewDeployStatusReconciler(cl, store, clientFor, cfg)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}}
	ctx := context.Background()

	// First reconcile adds the finalizer (and re-fetches), then runs the chain.
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Verify Status.Services populated with the computed "behind" state.
	var got sreportalv1alpha1.DeployStatus
	if err := cl.Get(ctx, req.NamespacedName, &got); err != nil {
		t.Fatalf("get after reconcile: %v", err)
	}
	if len(got.Status.Services) != 1 {
		t.Fatalf("expected 1 status service, got %d", len(got.Status.Services))
	}
	gotEntry := got.Status.Services[0]
	if gotEntry.State != "behind" {
		t.Errorf("expected state=behind, got %q", gotEntry.State)
	}
	if gotEntry.AheadBy != 2 {
		t.Errorf("expected aheadBy=2, got %d", gotEntry.AheadBy)
	}
	if gotEntry.DefaultBranch != "main" {
		t.Errorf("expected defaultBranch=main, got %q", gotEntry.DefaultBranch)
	}
	if len(gotEntry.PendingCommits) != 2 {
		t.Errorf("expected 2 pending commits, got %d", len(gotEntry.PendingCommits))
	}
	if got.Status.ServiceCount != 1 {
		t.Errorf("expected serviceCount=1, got %d", got.Status.ServiceCount)
	}

	// Verify the read store received the entry for (portalRef, namespace).
	entries := store.List("portal-a")
	if len(entries) != 1 {
		t.Fatalf("expected 1 readstore entry, got %d", len(entries))
	}
	if entries[0].Key != "k1" {
		t.Errorf("expected readstore entry key=k1, got %q", entries[0].Key)
	}
	if entries[0].State != "behind" {
		t.Errorf("expected readstore state=behind, got %q", entries[0].State)
	}
}

func TestReconcile_SkipsRemoteCR(t *testing.T) {
	scheme := newScheme(t)
	store := readstoredeploystatus.NewStore()

	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: objectMeta("remote-portal-b", "sreportal"),
		Spec: sreportalv1alpha1.DeployStatusSpec{
			PortalRef: "portal-b",
			Namespace: "default",
			IsRemote:  true,
			Services: []sreportalv1alpha1.DeployStatusEntry{
				{
					Key:         "k1",
					Workload:    sreportalv1alpha1.DeployStatusWorkloadRef{Kind: "Deployment", Namespace: "default", Name: "w", Container: "c"},
					Image:       "github.com/acme/widget@sha256:abc",
					SourceRepo:  "https://github.com/acme/widget",
					DeployedRef: "deadbeef",
				},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(cr).
		Build()

	// A forge client that would panic if called — proves no compute runs.
	clientFor := func(string) forge.Client {
		t.Fatal("clientFor must not be called for a remote CR")
		return nil
	}

	cfg := &config.DeployStatusConfig{Enabled: true, Forges: []config.ForgeConfig{{Host: "github.com"}}}
	r := NewDeployStatusReconciler(cl, store, clientFor, cfg)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}}
	ctx := context.Background()

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var got sreportalv1alpha1.DeployStatus
	if err := cl.Get(ctx, req.NamespacedName, &got); err != nil {
		t.Fatalf("get after reconcile: %v", err)
	}
	if len(got.Status.Services) != 0 {
		t.Errorf("expected no status services for remote CR, got %d", len(got.Status.Services))
	}
	if len(store.List("portal-b")) != 0 {
		t.Errorf("expected empty readstore for remote CR")
	}
}
