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

	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/remoteclient"
)

// noopLabelReader satisfies chain.ImageLabelReader for tests that don't
// exercise the deploy-status projection path (their chains short-circuit at
// portal validation before projection runs).
type noopLabelReader struct{}

func (noopLabelReader) ImageConfigLabels(_ context.Context, _, _, _ string) (map[string]string, error) {
	return nil, nil
}

// trackingImageWriter records calls to the ImageWriter interface so tests can
// assert which scopes were created/cleared during reconciliation.
type trackingImageWriter struct {
	mu       sync.Mutex
	replaces []scopeCall
	removes  []scopeCall
}

type scopeCall struct {
	Portal, Host, Namespace string
	Views                   []domainimage.ImageView
}

func (w *trackingImageWriter) ReplaceForNamespace(_ context.Context, portal, host, namespace string, views []domainimage.ImageView) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.replaces = append(w.replaces, scopeCall{Portal: portal, Host: host, Namespace: namespace, Views: views})
	return nil
}

func (w *trackingImageWriter) RemoveForNamespace(_ context.Context, portal, host, namespace string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.removes = append(w.removes, scopeCall{Portal: portal, Host: host, Namespace: namespace})
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
		ObjectMeta: metav1.ObjectMeta{Name: tNameInv, Namespace: tNsSre},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalA},
	}
	c := fake.NewClientBuilder().WithScheme(sch).
		WithObjects(inv).
		WithStatusSubresource(&sreportalv1alpha1.ImageInventory{}).
		Build()
	writer := &trackingImageWriter{}

	r := NewImageInventoryReconciler(c, writer, remoteclient.NewCache(), noopLabelReader{})
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: tNsSre, Name: tNameInv}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var got sreportalv1alpha1.ImageInventory
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: tNsSre, Name: tNameInv}, &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !slices.Contains(got.Finalizers, finalizerName) {
		t.Fatalf("finalizers=%v want to contain %q", got.Finalizers, finalizerName)
	}
}

func TestReconcileRemovesScopesOnDeletion(t *testing.T) {
	t.Parallel()
	sch := newControllerScheme(t)

	now := metav1.Now()
	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              tNameInv,
			Namespace:         tNsSre,
			Finalizers:        []string{finalizerName},
			DeletionTimestamp: &now,
		},
		Spec: sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalA},
		Status: sreportalv1alpha1.ImageInventoryStatus{
			Registries: []sreportalv1alpha1.ImageRegistryRef{
				{Hash: "h1", Host: "ghcr.io", Namespace: "default"},
				{Hash: "h2", Host: "docker.io", Namespace: "kube-system"},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).
		WithObjects(inv).
		WithStatusSubresource(&sreportalv1alpha1.ImageInventory{}).
		Build()
	writer := &trackingImageWriter{}

	r := NewImageInventoryReconciler(c, writer, remoteclient.NewCache(), noopLabelReader{})
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: tNsSre, Name: tNameInv}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(writer.removes) != 2 {
		t.Fatalf("removes=%d want 2", len(writer.removes))
	}
	for _, r := range writer.removes {
		if r.Portal != tPortalA {
			t.Errorf("remove call portal=%q want %q", r.Portal, tPortalA)
		}
	}
}

func TestReconcileDeletesMetricsOnDeletion(t *testing.T) {
	// NOTE: this test mutates global Prometheus metric state, so it MUST NOT
	// run with t.Parallel(). Unique label values prevent collision with the
	// other tests in this package that also touch image metrics.
	const (
		portalRef     = "metrics-cleanup-portal"
		inventoryName = "metrics-cleanup-inv"
	)

	// Pre-seed the metrics so we can later assert they were cleaned up.
	metrics.ImageImagesTotal.WithLabelValues(portalRef, "semver").Set(7)
	metrics.ImageImagesTotal.WithLabelValues(portalRef, "digest").Set(3)
	metrics.ImageInventorySyncTotal.WithLabelValues(inventoryName, "success").Inc()

	// Sanity check: the gauges/counters are populated before reconciliation.
	if got := testutil.ToFloat64(metrics.ImageImagesTotal.WithLabelValues(portalRef, "semver")); got != 7 {
		t.Fatalf("pre-condition: ImageImagesTotal{semver}=%v want 7", got)
	}
	if got := testutil.ToFloat64(metrics.ImageInventorySyncTotal.WithLabelValues(inventoryName, "success")); got != 1 {
		t.Fatalf("pre-condition: ImageInventorySyncTotal{success}=%v want 1", got)
	}

	sch := newControllerScheme(t)
	now := metav1.Now()
	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:              inventoryName,
			Namespace:         tNsSre,
			Finalizers:        []string{finalizerName},
			DeletionTimestamp: &now,
		},
		Spec: sreportalv1alpha1.ImageInventorySpec{PortalRef: portalRef},
	}
	c := fake.NewClientBuilder().WithScheme(sch).
		WithObjects(inv).
		WithStatusSubresource(&sreportalv1alpha1.ImageInventory{}).
		Build()
	writer := &trackingImageWriter{}

	r := NewImageInventoryReconciler(c, writer, remoteclient.NewCache(), noopLabelReader{})
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: tNsSre, Name: inventoryName}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// After DeletePartialMatch, every child labelled with the deleted portal
	// must be gone. We probe this directly via partial-match deletion: a
	// second call must return 0 because the entries were already removed.
	// We must NOT call WithLabelValues here — that would re-create a zeroed
	// child and mask the bug we are guarding against.
	if remaining := metrics.ImageImagesTotal.DeletePartialMatch(map[string]string{"portal": portalRef}); remaining != 0 {
		t.Fatalf("ImageImagesTotal still had %d children for portal=%q after deletion", remaining, portalRef)
	}
	if remaining := metrics.ImageInventorySyncTotal.DeletePartialMatch(map[string]string{"inventory": inventoryName}); remaining != 0 {
		t.Fatalf("ImageInventorySyncTotal still had %d children for inventory=%q after deletion", remaining, inventoryName)
	}
}
