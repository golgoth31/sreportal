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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

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

// --- ScanWorkloadsHandler tests ---

func TestScanWorkloadsPopulatesObservations(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: tNameAPI, Namespace: tNsDefault},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{tLabelApp: tNameAPI}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: tImgGhcrAPIv1}}},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(dep).Build()
	h := NewScanWorkloadsHandler(c)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tNameInv, Namespace: tNsSre},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:    tPortalA,
			WatchedKinds: []sreportalv1alpha1.ImageInventoryKind{sreportalv1alpha1.ImageInventoryKindDeployment},
		},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(rc.Data.Observations) != 1 {
		t.Fatalf("Observations=%d want 1", len(rc.Data.Observations))
	}
	got := rc.Data.Observations[0]
	if got.WorkloadKind != tKindDeploy {
		t.Errorf("WorkloadKind=%q want %q", got.WorkloadKind, tKindDeploy)
	}
	if got.WorkloadName != tNameAPI {
		t.Errorf("WorkloadName=%q want %q", got.WorkloadName, tNameAPI)
	}
	if got.WorkloadNamespace != tNsDefault {
		t.Errorf("WorkloadNamespace=%q want %q", got.WorkloadNamespace, tNsDefault)
	}
	if got.ContainerName != tNameWeb {
		t.Errorf("ContainerName=%q want %q", got.ContainerName, tNameWeb)
	}
	if got.TemplateImage != tImgGhcrAPIv1 {
		t.Errorf("TemplateImage=%q want %q", got.TemplateImage, tImgGhcrAPIv1)
	}
	// No running pod found — PodImage falls back to template.
	if got.PodImage != tImgGhcrAPIv1 {
		t.Errorf("PodImage=%q want %q (fallback to spec)", got.PodImage, tImgGhcrAPIv1)
	}
}

func TestScanWorkloadsCapturesMutatedAndInjectedFromPod(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: tNameAPI, Namespace: tNsDefault},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{tLabelApp: tNameAPI}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: tImgGhcrAPIv1}}},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "api-1", Namespace: tNsDefault,
			Labels: map[string]string{tLabelApp: tNameAPI},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: tNameWeb, Image: tImgGhcrAPIv2},              // mutated
				{Name: tContainerSidecar, Image: "ghcr.io/x/y:1.0"}, // injected
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(dep, pod).Build()
	h := NewScanWorkloadsHandler(c)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tNameInv, Namespace: tNsSre},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:    tPortalA,
			WatchedKinds: []sreportalv1alpha1.ImageInventoryKind{sreportalv1alpha1.ImageInventoryKindDeployment},
		},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(rc.Data.Observations) != 2 {
		t.Fatalf("Observations=%d want 2", len(rc.Data.Observations))
	}

	var spec, injected *struct {
		template, pod string
	}
	_ = spec
	_ = injected

	for _, o := range rc.Data.Observations {
		switch o.ContainerName {
		case tNameWeb:
			if o.TemplateImage != tImgGhcrAPIv1 {
				t.Errorf("spec container TemplateImage=%q want %q", o.TemplateImage, tImgGhcrAPIv1)
			}
			if o.PodImage != tImgGhcrAPIv2 {
				t.Errorf("spec container PodImage=%q want %q (mutated)", o.PodImage, tImgGhcrAPIv2)
			}
		case tContainerSidecar:
			if o.TemplateImage != "" {
				t.Errorf("injected TemplateImage=%q want empty", o.TemplateImage)
			}
			if o.PodImage != "ghcr.io/x/y:1.0" {
				t.Errorf("injected PodImage=%q want ghcr.io/x/y:1.0", o.PodImage)
			}
		default:
			t.Errorf("unexpected container %q", o.ContainerName)
		}
	}
}

func TestScanWorkloadsIsRemoteNoOp(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: tNameAPI, Namespace: tNsDefault},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: tImgGhcrAPIv1}}},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(dep).Build()
	h := NewScanWorkloadsHandler(c)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tNameInv, Namespace: tNsSre},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:    tPortalA,
			IsRemote:     true,
			WatchedKinds: []sreportalv1alpha1.ImageInventoryKind{sreportalv1alpha1.ImageInventoryKindDeployment},
		},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if rc.Data.Observations != nil {
		t.Fatalf("expected Observations to be nil for remote inventory, got %v", rc.Data.Observations)
	}
}

func TestScanWorkloadsPropagatesScanError(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	// Build a client with an invalid labelSelector so scanAll returns an error.
	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tNameInv, Namespace: tNsSre},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:     tPortalA,
			LabelSelector: "!!!invalid!!!",
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).
		WithObjects(inv).
		WithStatusSubresource(&sreportalv1alpha1.ImageInventory{}).
		Build()
	h := NewScanWorkloadsHandler(c)
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}

	err := h.Handle(context.Background(), rc)
	if err == nil {
		t.Fatalf("expected error from invalid labelSelector")
	}

	var got sreportalv1alpha1.ImageInventory
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: tNsSre, Name: tNameInv}, &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	cond := findCondition(got.Status.Conditions, ReadyConditionType)
	if cond == nil {
		t.Fatalf("want Ready condition, got none")
	}
	if cond.Status != metav1.ConditionFalse {
		t.Fatalf("Ready status=%q want False", cond.Status)
	}
	if cond.Reason != ReasonScanFailed {
		t.Fatalf("Ready reason=%q want %q", cond.Reason, ReasonScanFailed)
	}
}

func findCondition(conds []metav1.Condition, t string) *metav1.Condition {
	for i := range conds {
		if conds[i].Type == t {
			return &conds[i]
		}
	}
	return nil
}
