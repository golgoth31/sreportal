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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployStatusSpec is controller-managed — derived from ImageRegistry observations.
type DeployStatusSpec struct {
	// portalRef is the Portal name this deploy status is derived from.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`

	// namespace is the Kubernetes namespace observed (may differ from the CR's own namespace).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`

	// isRemote marks a shadow CR (`remote-<portal>`) whose entries are fetched from a
	// remote portal's DeployStatusService rather than computed locally (federation).
	// +optional
	IsRemote bool `json:"isRemote,omitempty"`

	// services is the list of per-workload deploy status entries.
	// +listType=map
	// +listMapKey=key
	// +optional
	Services []DeployStatusEntry `json:"services,omitempty"`
}

// DeployStatusEntry is one observed workload's deploy status.
type DeployStatusEntry struct {
	// key is sha256(image|workloadKind|workloadNamespace|workloadName|container)[:16].
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`

	// workload identifies the workload+container running the image.
	// +kubebuilder:validation:Required
	Workload DeployStatusWorkloadRef `json:"workload"`

	// image is the deployed image reference observed on the running Pod.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// sourceRepo is the git repo URL from the OCI source label. Empty when unresolved.
	// +optional
	SourceRepo string `json:"sourceRepo,omitempty"`

	// deployedRef is the deployed commit SHA (OCI revision label) or git tag (semver fallback).
	// +optional
	DeployedRef string `json:"deployedRef,omitempty"`

	// defaultBranch is the repo's default branch, read dynamically.
	// +optional
	DefaultBranch string `json:"defaultBranch,omitempty"`

	// aheadBy is the number of commits the default branch is ahead of deployedRef.
	// +optional
	AheadBy int `json:"aheadBy,omitempty"`

	// pendingCommits lists commits not yet deployed (merge commits filtered, capped at 50).
	// +optional
	PendingCommits []DeployStatusCommit `json:"pendingCommits,omitempty"`

	// pendingTruncated is true when more than 50 commits are pending.
	// +optional
	PendingTruncated bool `json:"pendingTruncated,omitempty"`

	// deployedAt is the commit date of the deployed ref (proxy — not the real deploy time).
	// +optional
	DeployedAt metav1.Time `json:"deployedAt,omitempty"`

	// deployRunURL links to the deploy workflow run gating prod (best-effort).
	// +optional
	DeployRunURL string `json:"deployRunUrl,omitempty"`

	// state is ok | behind | unresolved | error. Empty on Spec.Services input
	// entries (controller-managed); always set on Status.Services entries.
	// +optional
	// +kubebuilder:validation:Enum=ok;behind;unresolved;error
	State string `json:"state,omitempty"`

	// error carries the last per-entry error message (set when state=error).
	// +optional
	Error string `json:"error,omitempty"`

	// lastCheckedAt paces re-checks (isDue); set on every attempt, success or error.
	// +optional
	LastCheckedAt metav1.Time `json:"lastCheckedAt,omitempty"`
}

// DeployStatusWorkloadRef identifies a workload+container (mirrors ImageRegistryWorkloadRef).
type DeployStatusWorkloadRef struct {
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Container string `json:"container"`
}

// DeployStatusCommit is one pending commit.
type DeployStatusCommit struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Sha string `json:"sha"`
	// +optional
	Message string `json:"message,omitempty"`
	// +optional
	Author string `json:"author,omitempty"`
	// +optional
	Date metav1.Time `json:"date,omitempty"`
	// +optional
	URL string `json:"url,omitempty"`
}

// DeployStatusStatus defines the observed state of DeployStatus.
type DeployStatusStatus struct {
	// observedGeneration is the most recently observed generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// lastError contains the last reconciliation error, if any.
	// +optional
	LastError string `json:"lastError,omitempty"`

	// serviceCount is the total number of entries in Spec.Services.
	// +optional
	ServiceCount int `json:"serviceCount,omitempty"`

	// services is the observed per-workload deploy status (computed deployment lag).
	// This is the observed-state counterpart of Spec.Services and is the only place
	// the controller writes lag results / LastCheckedAt — Spec.Services carries only
	// the controller-managed input (workload, image, source, deployedRef).
	// +listType=map
	// +listMapKey=key
	// +optional
	Services []DeployStatusEntry `json:"services,omitempty"`

	// conditions represent the current state of the DeployStatus resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Namespace",type=string,JSONPath=`.spec.namespace`
// +kubebuilder:printcolumn:name="Services",type=integer,JSONPath=`.status.serviceCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DeployStatus is the Schema for the deploystatuses API
type DeployStatus struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DeployStatus
	// +required
	Spec DeployStatusSpec `json:"spec"`

	// status defines the observed state of DeployStatus
	// +optional
	Status DeployStatusStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DeployStatusList contains a list of DeployStatus
type DeployStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DeployStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeployStatus{}, &DeployStatusList{})
}
