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

// ImageRegistrySpec defines the desired state of an ImageRegistry CR. The
// whole spec is controller-managed by the ImageInventory controller — it is
// not user-edited in v1.
type ImageRegistrySpec struct {
	// host is the registry-of-origin (e.g. "docker.io", "ghcr.io").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`

	// portalRef is the Portal name this registry inventory is derived from.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`

	// namespace is the Kubernetes namespace targeted by this aggregation
	// (NOT the namespace the CR itself lives in — they may differ).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`

	// images is the list of images observed in (portalRef, host, namespace).
	// +listType=map
	// +listMapKey=key
	// +optional
	Images []ImageRegistrySpecEntry `json:"images,omitempty"`
}

// ImageRegistrySpecEntry is one image entry inside an ImageRegistry CR.
type ImageRegistrySpecEntry struct {
	// key is sha256(originalImage|mutatedImage|container)[:16] — used as
	// listMapKey for stable patches.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`

	// originalImage is the image declared in the workload's PodSpec
	// template. Empty when changeType=injected.
	// +optional
	OriginalImage string `json:"originalImage,omitempty"`

	// mutatedImage is the image observed in the running Pod.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	MutatedImage string `json:"mutatedImage"`

	// changeType classifies the relationship between OriginalImage and
	// MutatedImage. Allowed values: none, mutated, injected.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=none;mutated;injected
	ChangeType string `json:"changeType"`

	// repository is the parsed repository name (e.g. "library/nginx") from
	// the lookup target image (originalImage if non-empty, else
	// mutatedImage).
	// +optional
	Repository string `json:"repository,omitempty"`

	// originalTag is the tag of the lookup target image.
	// +optional
	OriginalTag string `json:"originalTag,omitempty"`

	// tagType classifies the OriginalTag (semver/commit/digest/latest/other).
	// +optional
	TagType string `json:"tagType,omitempty"`

	// workloads lists the workloads referencing this entry. No cap in v1.
	// +optional
	Workloads []ImageRegistryWorkloadRef `json:"workloads,omitempty"`
}

// ImageRegistryWorkloadRef identifies a workload+container referencing an entry.
type ImageRegistryWorkloadRef struct {
	// kind of the workload (Deployment, StatefulSet, etc.).
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
	// namespace of the workload (info; may differ for cross-ns mutations).
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
	// name of the workload.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// container is the name of the container/initContainer using the image.
	// +kubebuilder:validation:Required
	Container string `json:"container"`
}

// ImageRegistryStatus defines the observed state of ImageRegistry.
type ImageRegistryStatus struct {
	// observedGeneration is the most recently observed generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// lastError contains the last reconciliation error, if any.
	// +optional
	LastError string `json:"lastError,omitempty"`

	// imageCount is the total number of entries in Spec.Images.
	// +optional
	ImageCount int32 `json:"imageCount,omitempty"`

	// upgradeAvailableCount is the count of entries with UpgradeAvailable=true.
	// +optional
	UpgradeAvailableCount int32 `json:"upgradeAvailableCount,omitempty"`

	// mutatedCount is the count of entries with ChangeType=mutated.
	// +optional
	MutatedCount int32 `json:"mutatedCount,omitempty"`

	// injectedCount is the count of entries with ChangeType=injected.
	// +optional
	InjectedCount int32 `json:"injectedCount,omitempty"`

	// images carries per-entry resolution state, keyed by Spec.Images[].Key.
	// +listType=map
	// +listMapKey=key
	// +optional
	Images []ImageRegistryStatusEntry `json:"images,omitempty"`

	// conditions represents the current state of the ImageRegistry resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ImageRegistryStatusEntry is the per-image lookup result.
type ImageRegistryStatusEntry struct {
	// key joins Spec.Images[].Key.
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// latestVersion is the highest semver tag found on the registry. Empty
	// when not applicable (non-semver tag) or after a failed lookup.
	// +optional
	LatestVersion string `json:"latestVersion,omitempty"`

	// upgradeAvailable is true when latestVersion > originalTag.
	// +optional
	UpgradeAvailable bool `json:"upgradeAvailable,omitempty"`

	// lastCheckedAt is the timestamp of the most recent successful lookup.
	// +optional
	LastCheckedAt *metav1.Time `json:"lastCheckedAt,omitempty"`

	// lastError carries the last lookup error, if any.
	// +optional
	LastError string `json:"lastError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=imageregistries,scope=Namespaced
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.host`
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Namespace",type=string,JSONPath=`.spec.namespace`
// +kubebuilder:printcolumn:name="Images",type=integer,JSONPath=`.status.imageCount`
// +kubebuilder:printcolumn:name="Upgrades",type=integer,JSONPath=`.status.upgradeAvailableCount`
// +kubebuilder:printcolumn:name="Mutated",type=integer,JSONPath=`.status.mutatedCount`,priority=1
// +kubebuilder:printcolumn:name="Injected",type=integer,JSONPath=`.status.injectedCount`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ImageRegistry is the Schema for the imageregistries API.
type ImageRegistry struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ImageRegistry
	// +required
	Spec ImageRegistrySpec `json:"spec"`

	// status defines the observed state of ImageRegistry
	// +optional
	Status ImageRegistryStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ImageRegistryList contains a list of ImageRegistry.
type ImageRegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ImageRegistry `json:"items"`
}

// GetConditions returns the status conditions.
func (r *ImageRegistry) GetConditions() []metav1.Condition { return r.Status.Conditions }

// SetConditions sets the status conditions.
func (r *ImageRegistry) SetConditions(conditions []metav1.Condition) {
	r.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&ImageRegistry{}, &ImageRegistryList{})
}
