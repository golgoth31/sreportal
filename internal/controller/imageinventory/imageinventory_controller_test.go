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

package imageinventory

import (
	"context"
	"slices"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

type trackingImageWriter struct {
	mu      sync.Mutex
	deleted []string
}

func (w *trackingImageWriter) ReplaceWorkload(_ context.Context, _ string, _ domainimage.WorkloadKey, _ []domainimage.ImageView) error {
	return nil
}
func (w *trackingImageWriter) DeleteWorkloadAllPortals(_ context.Context, _ domainimage.WorkloadKey) error {
	return nil
}
func (w *trackingImageWriter) ReplaceAll(_ context.Context, _ string, _ map[domainimage.WorkloadKey][]domainimage.ImageView) error {
	return nil
}
func (w *trackingImageWriter) DeletePortal(_ context.Context, portalRef string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.deleted = append(w.deleted, portalRef)
	return nil
}

func newControllerScheme(t *testing.T) *runtime.Scheme {
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

func TestReconcileAddsFinalizerOnFirstPass(t *testing.T) {
	t.Parallel()
	sch := newControllerScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: "portal-a"},
	}
	c := fake.NewClientBuilder().WithScheme(sch).
		WithObjects(inv).
		WithStatusSubresource(&sreportalv1alpha1.ImageInventory{}).
		Build()
	writer := &trackingImageWriter{}

	r := NewImageInventoryReconciler(c, writer)
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "sre", Name: "inv"}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got sreportalv1alpha1.ImageInventory
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "sre", Name: "inv"}, &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !slices.Contains(got.Finalizers, finalizerName) {
		t.Fatalf("finalizers=%v want to contain %q", got.Finalizers, finalizerName)
	}
}

func TestReconcileDeletesPortalProjectionOnDeletion(t *testing.T) {
	t.Parallel()
	sch := newControllerScheme(t)

	now := metav1.Now()
	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "inv",
			Namespace:         "sre",
			Finalizers:        []string{finalizerName},
			DeletionTimestamp: &now,
		},
		Spec: sreportalv1alpha1.ImageInventorySpec{PortalRef: "portal-a"},
	}
	c := fake.NewClientBuilder().WithScheme(sch).
		WithObjects(inv).
		WithStatusSubresource(&sreportalv1alpha1.ImageInventory{}).
		Build()
	writer := &trackingImageWriter{}

	r := NewImageInventoryReconciler(c, writer)
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "sre", Name: "inv"}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(writer.deleted) != 1 || writer.deleted[0] != "portal-a" {
		t.Fatalf("deleted=%v want [portal-a]", writer.deleted)
	}
}
