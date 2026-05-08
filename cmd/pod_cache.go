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

package main

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// stripPodForCache strips a Pod down to the fields the operator actually
// reads, so the controller-runtime cache holds minimal Pod objects instead
// of full ones.
//
// The only consumers are
// internal/controller/imageinventory/chain/{pod_lookup,scan_workloads}.go
// which read:
//   - ObjectMeta: Name, Namespace, Labels, CreationTimestamp, UID, ResourceVersion
//   - Status.Phase
//   - Spec.Containers / Spec.InitContainers — Name + Image only
//
// Everything else (containerStatuses, conditions, hostIP/podIP, volumes,
// env, securityContext, tolerations, nodeSelector, annotations,
// managedFields, …) is dropped — those are the bulk of a typical Pod's
// in-memory footprint.
//
// If a future consumer needs more Pod fields, extend this function rather
// than reading them at the call site — a missing field here will silently
// look like a zero value.
func stripPodForCache(obj any) (any, error) {
	p, ok := obj.(*corev1.Pod)
	if !ok {
		return obj, nil
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              p.Name,
			Namespace:         p.Namespace,
			Labels:            p.Labels,
			CreationTimestamp: p.CreationTimestamp,
			UID:               p.UID,
			ResourceVersion:   p.ResourceVersion,
		},
		Status: corev1.PodStatus{Phase: p.Status.Phase},
		Spec: corev1.PodSpec{
			Containers:     stripContainers(p.Spec.Containers),
			InitContainers: stripContainers(p.Spec.InitContainers),
		},
	}, nil
}

func stripContainers(in []corev1.Container) []corev1.Container {
	if len(in) == 0 {
		return nil
	}
	out := make([]corev1.Container, len(in))
	for i, c := range in {
		out[i] = corev1.Container{Name: c.Name, Image: c.Image}
	}
	return out
}
