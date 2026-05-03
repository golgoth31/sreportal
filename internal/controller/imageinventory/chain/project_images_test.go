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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

func TestProjectImagesHandlerWritesToStore(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tNameInv, Namespace: tNsSre},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalA},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv).Build()
	writer := &fakeImageWriter{}
	h := NewProjectImagesHandler(c, writer)

	byWorkload := map[domainimage.WorkloadKey][]domainimage.ImageView{
		{Kind: tKindDeploy, Namespace: tNsDefault, Name: tNameAPI}: {
			{Registry: "ghcr.io", Repository: tRepoAcmeAPI, Tag: "v1.2.3", TagType: domainimage.TagTypeSemver},
		},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{
		Resource: inv,
		Data:     ChainData{ByWorkload: byWorkload},
	}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(writer.replaceAlls) != 1 {
		t.Fatalf("want 1 ReplaceAll, got %d", len(writer.replaceAlls))
	}
	call := writer.replaceAlls[0]
	if call.portalRef != tPortalA {
		t.Fatalf("portalRef=%q want portal-a", call.portalRef)
	}
	if len(call.byWorkload) != 1 {
		t.Fatalf("byWorkload entries=%d want 1", len(call.byWorkload))
	}
}

func TestProjectImagesHandlerNilByWorkloadWritesEmpty(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tNameInv, Namespace: tNsSre},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalA},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv).Build()
	writer := &fakeImageWriter{}
	h := NewProjectImagesHandler(c, writer)

	// ByWorkload is nil — ProjectImagesHandler should substitute an empty map.
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(writer.replaceAlls) != 1 {
		t.Fatalf("want 1 ReplaceAll, got %d", len(writer.replaceAlls))
	}
	if len(writer.replaceAlls[0].byWorkload) != 0 {
		t.Fatalf("expected empty byWorkload, got %d entries", len(writer.replaceAlls[0].byWorkload))
	}
}

func TestProjectImagesHandlerStoreErrorPatchesNotReady(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tNameInv, Namespace: tNsSre},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalA},
	}
	c := fake.NewClientBuilder().WithScheme(sch).
		WithObjects(inv).
		WithStatusSubresource(&sreportalv1alpha1.ImageInventory{}).
		Build()
	writer := &fakeImageWriter{replaceErr: errors.New("boom")}
	h := NewProjectImagesHandler(c, writer)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}

	err := h.Handle(context.Background(), rc)
	if err == nil {
		t.Fatalf("expected error")
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
	if cond.Reason != ReasonProjectionFailed {
		t.Fatalf("Ready reason=%q want %q", cond.Reason, ReasonProjectionFailed)
	}
}

// TestScanThenProjectEndToEnd exercises both ScanWorkloadsHandler and
// ProjectImagesHandler together — placed here as the more recent handler owns the
// write path that closes the pipeline.
func TestScanThenProjectEndToEnd(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: tNameAPI, Namespace: tNsDefault},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: "ghcr.io/acme/api:v1.0.0"}}},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(dep).Build()
	writer := &fakeImageWriter{}

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tNameInv, Namespace: tNsSre},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:    tPortalA,
			WatchedKinds: []sreportalv1alpha1.ImageInventoryKind{sreportalv1alpha1.ImageInventoryKindDeployment},
		},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}

	scanH := NewScanWorkloadsHandler(c)
	projectH := NewProjectImagesHandler(c, writer)

	if err := scanH.Handle(context.Background(), rc); err != nil {
		t.Fatalf("ScanWorkloadsHandler.Handle: %v", err)
	}
	if err := projectH.Handle(context.Background(), rc); err != nil {
		t.Fatalf("ProjectImagesHandler.Handle: %v", err)
	}

	if len(writer.replaceAlls) != 1 {
		t.Fatalf("want 1 ReplaceAll, got %d", len(writer.replaceAlls))
	}
	call := writer.replaceAlls[0]
	if call.portalRef != tPortalA {
		t.Fatalf("portalRef=%q want portal-a", call.portalRef)
	}
	if len(call.byWorkload) != 1 {
		t.Fatalf("byWorkload entries=%d want 1", len(call.byWorkload))
	}
}
