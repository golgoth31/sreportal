package chain

import (
	"context"
	"sync"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

// --- ScanWorkloadsHandler tests ---

func TestScanWorkloadsPopulatesByWorkload(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: tNameAPI, Namespace: tNsDefault},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: "ghcr.io/acme/api:v1"}}},
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
	if len(rc.Data.ByWorkload) != 1 {
		t.Fatalf("ByWorkload entries=%d want 1", len(rc.Data.ByWorkload))
	}
	wk := domainimage.WorkloadKey{Kind: tKindDeploy, Namespace: tNsDefault, Name: tNameAPI}
	views, ok := rc.Data.ByWorkload[wk]
	if !ok {
		t.Fatalf("expected workload key %+v in ByWorkload", wk)
	}
	if len(views) == 0 {
		t.Fatalf("expected at least one ImageView for workload %+v", wk)
	}
}

func TestScanWorkloadsIsRemoteNoOp(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: tNameAPI, Namespace: tNsDefault},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: "ghcr.io/acme/api:v1"}}},
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
	if rc.Data.ByWorkload != nil {
		t.Fatalf("expected ByWorkload to be nil for remote inventory, got %v", rc.Data.ByWorkload)
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
