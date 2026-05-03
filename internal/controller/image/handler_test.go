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
		ObjectMeta: metav1.ObjectMeta{Name: "inv-a", Namespace: tNsSRE},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:       tPortalA,
			NamespaceFilter: tNsDefault,
		},
	}
	invMiss := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-b", Namespace: tNsSRE},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:       "portal-b",
			NamespaceFilter: tOther,
		},
	}

	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(invMatch, invMiss).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: tImgGhcr}}}
	wk := domainimage.WorkloadKey{Kind: kindDeployment, Namespace: tNsDefault, Name: tNameAPI}

	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set(nil), nil); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}

	if len(writer.replaces) != 1 {
		t.Fatalf("got %d replaces, want 1", len(writer.replaces))
	}
	if writer.replaces[0].portalRef != tPortalA {
		t.Fatalf("portalRef=%q want portal-a", writer.replaces[0].portalRef)
	}
}

func TestHandleUpsertRespectsLabelSelector(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tInvName, Namespace: tNsSRE},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:     tPortalA,
			LabelSelector: "team=platform",
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: tImgGhcr}}}
	wk := domainimage.WorkloadKey{Kind: kindDeployment, Namespace: tNsDefault, Name: tNameAPI}

	// Labels do not match the selector.
	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set{"team": tOther}, nil); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}
	if len(writer.replaces) != 0 {
		t.Fatalf("want no replace, got %d", len(writer.replaces))
	}

	// Labels match the selector.
	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set{"team": "platform"}, nil); err != nil {
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
		ObjectMeta: metav1.ObjectMeta{Name: tInvName, Namespace: tNsSRE},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef:    tPortalA,
			WatchedKinds: []sreportalv1alpha1.ImageInventoryKind{sreportalv1alpha1.ImageInventoryKindStatefulSet},
		},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: tImgGhcr}}}
	// Deployment is not in WatchedKinds -> no replace.
	wk := domainimage.WorkloadKey{Kind: kindDeployment, Namespace: tNsDefault, Name: tNameAPI}
	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set(nil), nil); err != nil {
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

	wk := domainimage.WorkloadKey{Kind: kindDeployment, Namespace: tNsDefault, Name: tNameAPI}
	if err := h.HandleDelete(context.Background(), wk); err != nil {
		t.Fatalf("HandleDelete: %v", err)
	}
	if len(writer.deletes) != 1 || writer.deletes[0] != wk {
		t.Fatalf("deletes=%+v", writer.deletes)
	}
}

func TestHandleUpsertAddsPodInjectedContainers(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tInvName, Namespace: tNsSRE},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalA},
	}
	wkLabels := map[string]string{tNameApp: tNameAPI}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: tNameAPI1, Namespace: tNsDefault, Labels: wkLabels},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: tNameWeb, Image: tImgGhcr},
				{Name: "istio-proxy", Image: "docker.io/istio/proxyv2:1.20.0"},
			},
			InitContainers: []corev1.Container{
				{Name: "istio-init", Image: "docker.io/istio/proxyv2:1.20.0"},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv, pod).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: tImgGhcr}}}
	wk := domainimage.WorkloadKey{Kind: kindDeployment, Namespace: tNsDefault, Name: tNameAPI}
	sel := labels.SelectorFromSet(labels.Set(wkLabels))

	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set(nil), sel); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}

	if len(writer.replaces) != 1 {
		t.Fatalf("replaces=%d want 1", len(writer.replaces))
	}
	images := writer.replaces[0].images
	// Expected: 1 spec container (web) + 2 pod-injected (istio-proxy, istio-init).
	if len(images) != 3 {
		t.Fatalf("images=%d want 3: %+v", len(images), images)
	}

	specCount, podCount := 0, 0
	for _, iv := range images {
		if len(iv.Workloads) != 1 {
			t.Fatalf("each image should have exactly 1 workload ref, got %+v", iv.Workloads)
		}
		switch iv.Workloads[0].Source {
		case domainimage.ContainerSourceSpec:
			specCount++
		case domainimage.ContainerSourcePod:
			podCount++
		default:
			t.Fatalf("unexpected source %q", iv.Workloads[0].Source)
		}
	}
	if specCount != 1 || podCount != 2 {
		t.Fatalf("specCount=%d podCount=%d want 1, 2", specCount, podCount)
	}
}

func TestHandleUpsertDoesNotDuplicateUnchangedContainer(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tInvName, Namespace: tNsSRE},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalA},
	}
	wkLabels := map[string]string{tNameApp: tNameAPI}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: tNameAPI1, Namespace: tNsDefault, Labels: wkLabels},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: tNameWeb, Image: tImgGhcr},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv, pod).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: tImgGhcr}}}
	wk := domainimage.WorkloadKey{Kind: kindDeployment, Namespace: tNsDefault, Name: tNameAPI}
	sel := labels.SelectorFromSet(labels.Set(wkLabels))

	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set(nil), sel); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}

	images := writer.replaces[0].images
	if len(images) != 1 {
		t.Fatalf("images=%d want 1 (no duplicate from pod): %+v", len(images), images)
	}
	if images[0].Workloads[0].Source != domainimage.ContainerSourceSpec {
		t.Fatalf("source=%q want %q", images[0].Workloads[0].Source, domainimage.ContainerSourceSpec)
	}
}

func TestHandleUpsertDetectsImageMutation(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tInvName, Namespace: tNsSRE},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalA},
	}
	wkLabels := map[string]string{tNameApp: tNameAPI}
	// Pod has same container name but a different (mutated) image.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: tNameAPI1, Namespace: tNsDefault, Labels: wkLabels},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: tNameWeb, Image: "ghcr.io/acme/api:v1.2.4-pinned"},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv, pod).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: tImgGhcr}}}
	wk := domainimage.WorkloadKey{Kind: kindDeployment, Namespace: tNsDefault, Name: tNameAPI}
	sel := labels.SelectorFromSet(labels.Set(wkLabels))

	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set(nil), sel); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}

	images := writer.replaces[0].images
	// Expected: spec image (v1.2.3, source=spec) + mutated image (v1.2.4-pinned, source=pod)
	if len(images) != 2 {
		t.Fatalf("images=%d want 2: %+v", len(images), images)
	}
	tags := map[string]domainimage.ContainerSource{}
	for _, iv := range images {
		tags[iv.Tag] = iv.Workloads[0].Source
	}
	if tags["v1.2.3"] != domainimage.ContainerSourceSpec {
		t.Fatalf("v1.2.3 source=%q want spec", tags["v1.2.3"])
	}
	if tags["v1.2.4-pinned"] != domainimage.ContainerSourcePod {
		t.Fatalf("v1.2.4-pinned source=%q want pod", tags["v1.2.4-pinned"])
	}
}

func TestHandleUpsertNoRunningPodFallsBackToSpec(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tInvName, Namespace: tNsSRE},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalA},
	}
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(inv).Build()
	writer := &fakeImageWriter{}
	h := NewWorkloadHandler(c, writer)

	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: tNameWeb, Image: tImgGhcr}}}
	wk := domainimage.WorkloadKey{Kind: kindDeployment, Namespace: tNsDefault, Name: tNameAPI}
	sel := labels.SelectorFromSet(labels.Set{tNameApp: tNameAPI})

	if err := h.HandleUpsert(context.Background(), wk, spec, labels.Set(nil), sel); err != nil {
		t.Fatalf("HandleUpsert: %v", err)
	}

	images := writer.replaces[0].images
	if len(images) != 1 {
		t.Fatalf("images=%d want 1 (spec only): %+v", len(images), images)
	}
	if images[0].Workloads[0].Source != domainimage.ContainerSourceSpec {
		t.Fatalf("source=%q want spec", images[0].Workloads[0].Source)
	}
}

// Ensure the handler compiles with a real PodSpec extractor (smoke test).
var _ = appsv1.Deployment{}
