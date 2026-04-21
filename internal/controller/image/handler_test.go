package image

import (
	"context"
	"sync"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

type recordedReplace struct {
	portalRef string
	wk        domainimage.WorkloadKey
	images    []domainimage.ImageView
}

type fakeImageWriter struct {
	mu       sync.Mutex
	replaces []recordedReplace
	deletes  []domainimage.WorkloadKey
}

func (f *fakeImageWriter) ReplaceWorkload(_ context.Context, portalRef string, wk domainimage.WorkloadKey, images []domainimage.ImageView) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replaces = append(f.replaces, recordedReplace{portalRef, wk, images})
	return nil
}

func (f *fakeImageWriter) DeleteWorkloadAllPortals(_ context.Context, wk domainimage.WorkloadKey) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes = append(f.deletes, wk)
	return nil
}

func (f *fakeImageWriter) ReplaceAll(_ context.Context, _ string, _ map[domainimage.WorkloadKey][]domainimage.ImageView) error {
	return nil
}

func (f *fakeImageWriter) DeletePortal(_ context.Context, _ string) error { return nil }

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	sch := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(sch); err != nil {
		t.Fatalf("add clientgo scheme: %v", err)
	}
	if err := sreportalv1alpha1.AddToScheme(sch); err != nil {
		t.Fatalf("add sreportal scheme: %v", err)
	}
	return sch
}

func TestHandleUpsertMatchesNamespaceFilter(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	invMatch := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-a", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:       "portal-a",
			NamespaceFilter: "default",
		},
	}
	invMiss := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-b", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:       "portal-b",
			NamespaceFilter: "other",
		},
	}

	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(invMatch, invMiss).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1.2.3"}}}
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "api"}

	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set(nil)); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}

	if len(writer.replaces) != 1 {
		t.Fatalf("got %d replaces, want 1", len(writer.replaces))
	}
	if writer.replaces[0].portalRef != "portal-a" {
		t.Fatalf("portalRef=%q want portal-a", writer.replaces[0].portalRef)
	}
}

func TestHandleUpsertRespectsLabelSelector(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:     "portal-a",
			LabelSelector: "team=platform",
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1.2.3"}}}
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "api"}

	// Labels do not match the selector.
	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set{"team": "other"}); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}
	if len(writer.replaces) != 0 {
		t.Fatalf("want no replace, got %d", len(writer.replaces))
	}

	// Labels match the selector.
	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set{"team": "platform"}); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}
	if len(writer.replaces) != 1 {
		t.Fatalf("want one replace, got %d", len(writer.replaces))
	}
}

func TestHandleUpsertRespectsWatchedKinds(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:    "portal-a",
			WatchedKinds: []sreportalv1alpha1.ImageInventoryKind{sreportalv1alpha1.ImageInventoryKindStatefulSet},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1.2.3"}}}
	// Deployment is not in WatchedKinds -> no replace.
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "api"}
	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set(nil)); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}
	if len(writer.replaces) != 0 {
		t.Fatalf("want no replace, got %d", len(writer.replaces))
	}
}

func TestHandleDeleteCallsStore(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(sch).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "api"}
	if err := h.HandleDelete(context.Background(), wk); err != nil {
		t.Fatalf("HandleDelete: %v", err)
	}
	if len(writer.deletes) != 1 || writer.deletes[0] != wk {
		t.Fatalf("deletes=%+v", writer.deletes)
	}
}

// Ensure the handler compiles with a real PodSpec extractor (smoke test).
var _ = appsv1.Deployment{}
