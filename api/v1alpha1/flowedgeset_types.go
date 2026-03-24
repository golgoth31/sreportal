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

// FlowEdgeSetSpec defines the desired state of FlowEdgeSet.
type FlowEdgeSetSpec struct {
	// discoveryRef is the name of the parent NetworkFlowDiscovery resource
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	DiscoveryRef string `json:"discoveryRef"`
}

// FlowEdgeSetStatus defines the observed state of FlowEdgeSet.
type FlowEdgeSetStatus struct {
	// edges are the directional flow relations between nodes
	// +optional
	Edges []FlowEdge `json:"edges,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Discovery",type=string,JSONPath=`.spec.discoveryRef`
// +kubebuilder:printcolumn:name="Edges",type=integer,JSONPath=`.status.edges`

// FlowEdgeSet stores the discovered flow edges for a NetworkFlowDiscovery resource.
type FlowEdgeSet struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// +required
	Spec FlowEdgeSetSpec `json:"spec"`

	// +optional
	Status FlowEdgeSetStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// FlowEdgeSetList contains a list of FlowEdgeSet.
type FlowEdgeSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FlowEdgeSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FlowEdgeSet{}, &FlowEdgeSetList{})
}
