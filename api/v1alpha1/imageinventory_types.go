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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ImageInventoryKind names a Kubernetes workload kind that can be scanned for images.
// +kubebuilder:validation:Enum=Deployment;StatefulSet;DaemonSet;CronJob;Job
type ImageInventoryKind string

const (
	// ImageInventoryKindDeployment scans apps/v1 Deployments.
	ImageInventoryKindDeployment ImageInventoryKind = "Deployment"
	// ImageInventoryKindStatefulSet scans apps/v1 StatefulSets.
	ImageInventoryKindStatefulSet ImageInventoryKind = "StatefulSet"
	// ImageInventoryKindDaemonSet scans apps/v1 DaemonSets.
	ImageInventoryKindDaemonSet ImageInventoryKind = "DaemonSet"
	// ImageInventoryKindCronJob scans batch/v1 CronJobs.
	ImageInventoryKindCronJob ImageInventoryKind = "CronJob"
	// ImageInventoryKindJob scans batch/v1 Jobs.
	ImageInventoryKindJob ImageInventoryKind = "Job"
)

var defaultImageInventoryWatchedKinds = []ImageInventoryKind{
	ImageInventoryKindDeployment,
	ImageInventoryKindStatefulSet,
	ImageInventoryKindDaemonSet,
	ImageInventoryKindCronJob,
	ImageInventoryKindJob,
}

// ImageInventorySpec defines the desired state of ImageInventory.
type ImageInventorySpec struct {
	// portalRef is the Portal name this inventory belongs to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`

	// watchedKinds declares which workload kinds are scanned for images.
	// Empty means all supported defaults.
	// +optional
	WatchedKinds []ImageInventoryKind `json:"watchedKinds,omitempty"`

	// namespaceFilter restricts scan to a single namespace when set.
	// Empty means all namespaces.
	// +optional
	NamespaceFilter string `json:"namespaceFilter,omitempty"`

	// labelSelector is a Kubernetes label selector string used to filter workloads.
	// Empty means no label filtering.
	// +optional
	LabelSelector string `json:"labelSelector,omitempty"`

	// interval controls how often this inventory is refreshed.
	// Empty means default 5m.
	// +optional
	Interval metav1.Duration `json:"interval,omitempty"`

	// isRemote marks this inventory as a shadow projection of a remote portal.
	// When true, the controller fetches images from the remote portal via the
	// ImageService Connect API instead of scanning the local cluster.
	// +optional
	IsRemote bool `json:"isRemote,omitempty"`

	// remoteURL is the base URL of the remote SRE Portal to fetch images from.
	// Only meaningful when isRemote is true. Populated by the Portal controller
	// from spec.remote.url on the Portal CR.
	// +kubebuilder:validation:Pattern=`^https?://.*`
	// +optional
	RemoteURL string `json:"remoteURL,omitempty"`
}

// ImageInventoryStatus defines the observed state of ImageInventory.
type ImageInventoryStatus struct {
	// observedGeneration is the most recently observed generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// lastScanTime is the timestamp of the latest completed scan.
	// +optional
	LastScanTime *metav1.Time `json:"lastScanTime,omitempty"`

	// lastScanError contains the latest scan error, if any.
	// +optional
	LastScanError string `json:"lastScanError,omitempty"`

	// conditions represent the current state of the ImageInventory resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=imageinventories,scope=Namespaced
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Namespace",type=string,JSONPath=`.spec.namespaceFilter`,priority=1
// +kubebuilder:printcolumn:name="Last Scan",type=date,JSONPath=`.status.lastScanTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ImageInventory is the Schema for the imageinventories API
type ImageInventory struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ImageInventory
	// +required
	Spec ImageInventorySpec `json:"spec"`

	// status defines the observed state of ImageInventory
	// +optional
	Status ImageInventoryStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ImageInventoryList contains a list of ImageInventory
type ImageInventoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ImageInventory `json:"items"`
}

// GetConditions returns the status conditions.
func (i *ImageInventory) GetConditions() []metav1.Condition { return i.Status.Conditions }

// SetConditions sets the status conditions.
func (i *ImageInventory) SetConditions(conditions []metav1.Condition) {
	i.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&ImageInventory{}, &ImageInventoryList{})
}

// EffectiveWatchedKinds returns configured workload kinds or defaults.
func (s ImageInventorySpec) EffectiveWatchedKinds() []ImageInventoryKind {
	if len(s.WatchedKinds) == 0 {
		out := make([]ImageInventoryKind, len(defaultImageInventoryWatchedKinds))
		copy(out, defaultImageInventoryWatchedKinds)
		return out
	}
	out := make([]ImageInventoryKind, len(s.WatchedKinds))
	copy(out, s.WatchedKinds)
	return out
}

// EffectiveInterval returns configured interval or default 5 minutes.
func (s ImageInventorySpec) EffectiveInterval() time.Duration {
	if s.Interval.Duration <= 0 {
		return 5 * time.Minute
	}
	return s.Interval.Duration
}
