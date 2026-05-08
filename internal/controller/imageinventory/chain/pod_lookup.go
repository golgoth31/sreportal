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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// podIndex caches running pods per namespace for the duration of a single
// reconcile, so that all workloads in the same namespace share one List call
// instead of doing one per workload. Without it, a 500-workload scan does 500
// LISTs against the pods cache; with it, the same scan does at most one LIST
// per touched namespace.
//
// Not safe for concurrent use — workloads are scanned sequentially.
type podIndex struct {
	c           client.Client
	byNamespace map[string][]*corev1.Pod
}

func newPodIndex(c client.Client) *podIndex {
	return &podIndex{c: c, byNamespace: map[string][]*corev1.Pod{}}
}

// findNewestRunning returns the most-recently-created Running pod in
// `namespace` whose labels match `selector`. Returns (nil, nil) when no
// match exists or selector is nil. The first call for a namespace lists
// all pods; subsequent calls reuse the cached slice.
func (p *podIndex) findNewestRunning(ctx context.Context, namespace string, selector labels.Selector) (*corev1.Pod, error) {
	if selector == nil {
		return nil, nil
	}
	pods, err := p.podsInNamespace(ctx, namespace)
	if err != nil {
		return nil, err
	}
	var newest *corev1.Pod
	for _, pod := range pods {
		if !selector.Matches(labels.Set(pod.Labels)) {
			continue
		}
		if newest == nil || pod.CreationTimestamp.After(newest.CreationTimestamp.Time) {
			newest = pod
		}
	}
	return newest, nil
}

func (p *podIndex) podsInNamespace(ctx context.Context, namespace string) ([]*corev1.Pod, error) {
	if pods, ok := p.byNamespace[namespace]; ok {
		return pods, nil
	}
	var list corev1.PodList
	if err := p.c.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list pods in %q: %w", namespace, err)
	}
	pods := make([]*corev1.Pod, 0, len(list.Items))
	for i := range list.Items {
		if list.Items[i].Status.Phase != corev1.PodRunning {
			continue
		}
		pods = append(pods, &list.Items[i])
	}
	p.byNamespace[namespace] = pods
	return pods, nil
}
