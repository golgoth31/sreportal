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

// FlowNodeSetSpec defines the desired state of FlowNodeSet.
type FlowNodeSetSpec struct {
	// discoveryRef is the name of the parent NetworkFlowDiscovery resource
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	DiscoveryRef string `json:"discoveryRef"`
}

// FlowNodeSetStatus defines the observed state of FlowNodeSet.
type FlowNodeSetStatus struct {
	// nodes are all discovered services, databases, crons, and external endpoints
	// +optional
	Nodes []FlowNode `json:"nodes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Discovery",type=string,JSONPath=`.spec.discoveryRef`
// +kubebuilder:printcolumn:name="Nodes",type=integer,JSONPath=`.status.nodes`

// FlowNodeSet stores the discovered flow nodes for a NetworkFlowDiscovery resource.
type FlowNodeSet struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// +required
	Spec FlowNodeSetSpec `json:"spec"`

	// +optional
	Status FlowNodeSetStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// FlowNodeSetList contains a list of FlowNodeSet.
type FlowNodeSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FlowNodeSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FlowNodeSet{}, &FlowNodeSetList{})
}
