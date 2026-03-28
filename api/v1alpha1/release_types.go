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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ReleaseSpec defines the desired state of Release
type ReleaseSpec struct {
	// portalRef is the name of the Portal this release is linked to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`

	// entries is the list of release events for this day
	// +optional
	Entries []ReleaseEntry `json:"entries,omitempty"`
}

// ReleaseEntry represents a single release event
type ReleaseEntry struct {
	// type is the kind of release (e.g., "deployment", "rollback", "hotfix")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Type string `json:"type"`

	// version is the version string of the release
	// +optional
	Version string `json:"version,omitempty"`

	// origin identifies where the release came from (e.g., "ci/cd", "manual", service name)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Origin string `json:"origin"`

	// date is the timestamp of the release
	// +kubebuilder:validation:Required
	Date metav1.Time `json:"date"`

	// author is the author of the release
	// +kubebuilder:validation:Required
	Author string `json:"author"`

	// message is the message of the release
	// +kubebuilder:validation:Required
	Message string `json:"message"`

	// link is the link to the release
	// +kubebuilder:validation:Required
	Link string `json:"link"`
}

// ReleaseStatus defines the observed state of Release.
type ReleaseStatus struct {
	// entryCount is the number of release entries in this CR
	// +optional
	EntryCount int `json:"entryCount,omitempty"`

	// conditions represent the current state of the Release resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Entries",type=integer,JSONPath=`.status.entryCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Release is the Schema for the releases API
type Release struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Release
	// +required
	Spec ReleaseSpec `json:"spec"`

	// status defines the observed state of Release
	// +optional
	Status ReleaseStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ReleaseList contains a list of Release
type ReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Release `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Release{}, &ReleaseList{})
}
