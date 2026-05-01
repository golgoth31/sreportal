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
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1"}}},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(dep).Build()
	h := NewScanWorkloadsHandler(c)

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
	if len(rc.Data.ByWorkload) != 1 {
		t.Fatalf("ByWorkload entries=%d want 1", len(rc.Data.ByWorkload))
	}
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "api"}
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
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1"}}},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(dep).Build()
	h := NewScanWorkloadsHandler(c)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:    "portal-a",
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
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:     "portal-a",
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
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "sre", Name: "inv"}, &got); err != nil {
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

// --- ProjectImagesHandler tests ---

func TestProjectImagesHandlerWritesToStore(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: "portal-a"},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv).Build()
	writer := &fakeImageWriter{}
	h := NewProjectImagesHandler(c, writer)

	byWorkload := map[domainimage.WorkloadKey][]domainimage.ImageView{
		{Kind: "Deployment", Namespace: "default", Name: "api"}: {
			{Registry: "ghcr.io", Repository: "acme/api", Tag: "v1.2.3", TagType: domainimage.TagTypeSemver},
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
	if call.portalRef != "portal-a" {
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
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: "portal-a"},
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
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: "portal-a"},
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
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "sre", Name: "inv"}, &got); err != nil {
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

// --- Integration: Scan then Project ---

func TestScanThenProjectEndToEnd(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "ghcr.io/acme/api:v1.0.0"}}},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(dep).Build()
	writer := &fakeImageWriter{}

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "sre"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:    "portal-a",
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
	if call.portalRef != "portal-a" {
		t.Fatalf("portalRef=%q want portal-a", call.portalRef)
	}
	if len(call.byWorkload) != 1 {
		t.Fatalf("byWorkload entries=%d want 1", len(call.byWorkload))
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
