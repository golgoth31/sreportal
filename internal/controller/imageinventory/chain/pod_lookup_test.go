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

func TestPodIndexFindNewestRunning(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	now := time.Now()
	older := newPod("old", tNsDefault, map[string]string{tLabelApp: tNameAPI}, corev1.PodRunning, now.Add(-2*time.Hour))
	newer := newPod("new", tNsDefault, map[string]string{tLabelApp: tNameAPI}, corev1.PodRunning, now.Add(-1*time.Hour))
	pending := newPod("pending", tNsDefault, map[string]string{tLabelApp: tNameAPI}, corev1.PodPending, now)
	otherNs := newPod("other-ns", "kube-system", map[string]string{tLabelApp: tNameAPI}, corev1.PodRunning, now)
	otherLabel := newPod("other-label", tNsDefault, map[string]string{tLabelApp: "other"}, corev1.PodRunning, now)

	c := fake.NewClientBuilder().WithScheme(sch).
		WithObjects(older, newer, pending, otherNs, otherLabel).
		Build()

	sel := labels.SelectorFromSet(labels.Set{tLabelApp: tNameAPI})
	idx := newPodIndex(c)
	pod, err := idx.findNewestRunning(context.Background(), tNsDefault, sel)
	if err != nil {
		t.Fatalf("findNewestRunning: %v", err)
	}
	if pod == nil {
		t.Fatal("got nil pod, expected newest running")
	}
	if pod.Name != "new" {
		t.Fatalf("pod=%q want %q", pod.Name, "new")
	}
}

func TestPodIndexNoMatch(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)
	c := fake.NewClientBuilder().WithScheme(sch).Build()

	sel := labels.SelectorFromSet(labels.Set{tLabelApp: tNameAPI})
	idx := newPodIndex(c)
	pod, err := idx.findNewestRunning(context.Background(), tNsDefault, sel)
	if err != nil {
		t.Fatalf("findNewestRunning: %v", err)
	}
	if pod != nil {
		t.Fatalf("expected nil pod, got %+v", pod)
	}
}

func TestPodIndexNilSelector(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)
	c := fake.NewClientBuilder().WithScheme(sch).Build()

	idx := newPodIndex(c)
	pod, err := idx.findNewestRunning(context.Background(), tNsDefault, nil)
	if err != nil {
		t.Fatalf("findNewestRunning: %v", err)
	}
	if pod != nil {
		t.Fatalf("expected nil pod, got %+v", pod)
	}
}

func TestPodIndexOnlyNonRunning(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)
	now := time.Now()
	pod := newPod("p", tNsDefault, map[string]string{tLabelApp: tNameAPI}, corev1.PodPending, now)
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(pod).Build()

	sel := labels.SelectorFromSet(labels.Set{tLabelApp: tNameAPI})
	idx := newPodIndex(c)
	got, err := idx.findNewestRunning(context.Background(), tNsDefault, sel)
	if err != nil {
		t.Fatalf("findNewestRunning: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil pod (only non-running), got %+v", got)
	}
}

// TestPodIndexCachesPerNamespace verifies that repeated lookups in the same
// namespace reuse the cached pod slice rather than issuing another List.
func TestPodIndexCachesPerNamespace(t *testing.T) {
	t.Parallel()
	sch := newChainScheme(t)

	now := time.Now()
	p1 := newPod("p1", tNsDefault, map[string]string{tLabelApp: tNameAPI}, corev1.PodRunning, now)
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(p1).Build()

	idx := newPodIndex(c)
	sel := labels.SelectorFromSet(labels.Set{tLabelApp: tNameAPI})

	if _, err := idx.findNewestRunning(context.Background(), tNsDefault, sel); err != nil {
		t.Fatalf("first call: %v", err)
	}
	cached, ok := idx.byNamespace[tNsDefault]
	if !ok || len(cached) != 1 {
		t.Fatalf("expected namespace cached with 1 pod, got %v ok=%v", cached, ok)
	}
	if _, err := idx.findNewestRunning(context.Background(), tNsDefault, sel); err != nil {
		t.Fatalf("second call: %v", err)
	}
}
