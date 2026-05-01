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

package image

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// findRunningPodForWorkload lists pods matching `selector` in `namespace`,
// filters them to PodRunning phase, and returns the most recently created one.
// Returns (nil, nil) when no running pod is found — callers fall back to the
// workload spec.
func findRunningPodForWorkload(
	ctx context.Context,
	c client.Client,
	namespace string,
	selector labels.Selector,
) (*corev1.Pod, error) {
	if selector == nil {
		return nil, nil
	}
	var podList corev1.PodList
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}
	if err := c.List(ctx, &podList, opts...); err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	var newest *corev1.Pod
	for i := range podList.Items {
		p := &podList.Items[i]
		if p.Status.Phase != corev1.PodRunning {
			continue
		}
		if newest == nil || p.CreationTimestamp.After(newest.CreationTimestamp.Time) {
			newest = p
		}
	}
	return newest, nil
}

// FindRunningPodForWorkload is the exported wrapper used by the full-scan handler.
func FindRunningPodForWorkload(
	ctx context.Context,
	c client.Client,
	namespace string,
	selector labels.Selector,
) (*corev1.Pod, error) {
	return findRunningPodForWorkload(ctx, c, namespace, selector)
}
