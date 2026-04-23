package chain

import (
	"context"
	"errors"
	"sync"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

type replaceAllCall struct {
	portalRef  string
	byWorkload map[domainimage.WorkloadKey][]domainimage.ImageView
}

type fakeImageWriter struct {
	mu          sync.Mutex
	replaceAlls []replaceAllCall
	replaceErr  error
}

func (f *fakeImageWriter) ReplaceWorkload(_ context.Context, _ string, _ domainimage.WorkloadKey, _ []domainimage.ImageView) error {
	return nil
}
func (f *fakeImageWriter) DeleteWorkloadAllPortals(_ context.Context, _ domainimage.WorkloadKey) error {
	return nil
}
func (f *fakeImageWriter) ReplaceAll(_ context.Context, portalRef string, byWorkload map[domainimage.WorkloadKey][]domainimage.ImageView) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replaceErr != nil {
		return f.replaceErr
	}
	f.replaceAlls = append(f.replaceAlls, replaceAllCall{portalRef, byWorkload})
	return nil
}
func (f *fakeImageWriter) DeletePortal(_ context.Context, _ string) error { return nil }

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

func TestScanWorkloadsReplacesAll(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1"}}},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(dep).Build()
	writer := &fakeImageWriter{}
	h := NewScanWorkloadsHandler(c, writer)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:    "portal-a",
			WatchedKinds: []sreportalv1alpha1.ImageInventoryKind{sreportalv1alpha1.ImageInventoryKindDeployment},
		},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}

	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(writer.replaceAlls) != 1 {
		t.Fatalf("want 1 ReplaceAll, got %d", len(writer.replaceAlls))
	}
	call := writer.replaceAlls[0]
	if call.portalRef != "portal-a" {
		t.Fatalf("portalRef=%q want portal-a", call.portalRef)
	}
	if len(call.byWorkload) != 1 {
		t.Fatalf("byWorkload entries=%d want 1", len(call.byWorkload))
	}
}

func TestScanWorkloadsPropagatesErrorWithoutReplace(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)
	c := fake.NewClientBuilder().WithScheme(sch).Build()
	writer := &fakeImageWriter{replaceErr: errors.New("boom")}
	h := NewScanWorkloadsHandler(c, writer)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: "portal-a"},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}

	err := h.Handle(context.Background(), rc)
	if err == nil {
		t.Fatalf("expected error")
	}
}
