package image

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newPod(name, ns string, lbls map[string]string, phase corev1.PodPhase, created time.Time) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			Labels:            lbls,
			CreationTimestamp: metav1.NewTime(created),
		},
		Status: corev1.PodStatus{Phase: phase},
	}
}

func TestFindRunningPodForWorkloadReturnsNewestRunning(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)

	now := time.Now()
	older := newPod("old", tNsDefault, map[string]string{tNameApp: tNameAPI}, corev1.PodRunning, now.Add(-2*time.Hour))
	newer := newPod("new", tNsDefault, map[string]string{tNameApp: tNameAPI}, corev1.PodRunning, now.Add(-1*time.Hour))
	pending := newPod("pending", tNsDefault, map[string]string{tNameApp: tNameAPI}, corev1.PodPending, now)
	otherNs := newPod("other-ns", "kube-system", map[string]string{tNameApp: tNameAPI}, corev1.PodRunning, now)
	otherLabel := newPod("other-label", tNsDefault, map[string]string{tNameApp: tOther}, corev1.PodRunning, now)

	c := fake.NewClientBuilder().WithScheme(sch).
		WithObjects(older, newer, pending, otherNs, otherLabel).
		Build()

	sel := labels.SelectorFromSet(labels.Set{tNameApp: tNameAPI})
	pod, err := findRunningPodForWorkload(context.Background(), c, tNsDefault, sel)
	if err != nil {
		t.Fatalf("findRunningPodForWorkload: %v", err)
	}
	if pod == nil {
		t.Fatal("got nil pod, expected newest running")
	}
	if pod.Name != "new" {
		t.Fatalf("pod=%q want %q", pod.Name, "new")
	}
}

func TestFindRunningPodForWorkloadNoMatch(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(sch).Build()

	sel := labels.SelectorFromSet(labels.Set{tNameApp: tNameAPI})
	pod, err := findRunningPodForWorkload(context.Background(), c, tNsDefault, sel)
	if err != nil {
		t.Fatalf("findRunningPodForWorkload: %v", err)
	}
	if pod != nil {
		t.Fatalf("expected nil pod, got %+v", pod)
	}
}

func TestFindRunningPodForWorkloadNilSelector(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(sch).Build()

	pod, err := findRunningPodForWorkload(context.Background(), c, tNsDefault, nil)
	if err != nil {
		t.Fatalf("findRunningPodForWorkload: %v", err)
	}
	if pod != nil {
		t.Fatalf("expected nil pod, got %+v", pod)
	}
}

func TestFindRunningPodForWorkloadOnlyNonRunning(t *testing.T) {
	t.Parallel()
	sch := newTestScheme(t)
	now := time.Now()
	pod := newPod("p", tNsDefault, map[string]string{tNameApp: tNameAPI}, corev1.PodPending, now)
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(pod).Build()

	sel := labels.SelectorFromSet(labels.Set{tNameApp: tNameAPI})
	got, err := findRunningPodForWorkload(context.Background(), c, tNsDefault, sel)
	if err != nil {
		t.Fatalf("findRunningPodForWorkload: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil pod (only non-running), got %+v", got)
	}
}
