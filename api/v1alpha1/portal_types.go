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

// PortalSpec defines the desired state of Portal
type PortalSpec struct {
	// title is the display title for this portal
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Title string `json:"title"`

	// main marks this portal as the default portal for unmatched FQDNs
	// +optional
	Main bool `json:"main,omitempty"`

	// subPath is the URL subpath for this portal (defaults to metadata.name)
	// +optional
	SubPath string `json:"subPath,omitempty"`
}

// PortalStatus defines the observed state of Portal.
type PortalStatus struct {
	// ready indicates if the portal is fully configured
	// +optional
	Ready bool `json:"ready,omitempty"`

	// conditions represent the current state of the Portal resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=portals,scope=Namespaced
// +kubebuilder:printcolumn:name="Title",type=string,JSONPath=`.spec.title`
// +kubebuilder:printcolumn:name="Main",type=boolean,JSONPath=`.spec.main`
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Portal is the Schema for the portals API
type Portal struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Portal
	// +required
	Spec PortalSpec `json:"spec"`

	// status defines the observed state of Portal
	// +optional
	Status PortalStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PortalList contains a list of Portal
type PortalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Portal `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Portal{}, &PortalList{})
}
