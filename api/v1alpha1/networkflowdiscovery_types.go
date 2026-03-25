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

// NetworkFlowDiscoverySpec defines the desired state of NetworkFlowDiscovery.
type NetworkFlowDiscoverySpec struct {
	// portalRef is the name of the Portal this resource is linked to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`

	// namespaces is an optional list of namespaces to scan.
	// When empty, all namespaces are scanned.
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// isRemote indicates that the corresponding portal is remote and the operator
	// should fetch network flows from the remote portal Connect API instead of
	// scanning local Kubernetes NetworkPolicies.
	// +optional
	IsRemote bool `json:"isRemote,omitempty"`

	// remoteURL is the base URL of the remote SRE Portal to fetch network flows from.
	// Only used when isRemote is true.
	// +optional
	RemoteURL string `json:"remoteURL,omitempty"`
}

// NetworkFlowDiscoveryStatus defines the observed state of NetworkFlowDiscovery.
type NetworkFlowDiscoveryStatus struct {
	// nodeCount is the number of discovered nodes
	// +optional
	NodeCount int `json:"nodeCount,omitempty"`

	// edgeCount is the number of discovered edges
	// +optional
	EdgeCount int `json:"edgeCount,omitempty"`

	// conditions represent the current state of the resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// lastReconcileTime is the timestamp of the last reconciliation
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
}

// FlowNode represents a service, database, cron job, or external endpoint.
type FlowNode struct {
	// id is the unique node identifier (e.g. "service:core:my-account-api")
	ID string `json:"id"`

	// label is the human-readable name
	Label string `json:"label"`

	// namespace is the Kubernetes namespace
	Namespace string `json:"namespace"`

	// nodeType is one of: service, cron, database, messaging, external
	NodeType string `json:"nodeType"`

	// group is the logical group (namespace name by default)
	Group string `json:"group"`
}

// FlowEdge represents a directional flow between two nodes.
type FlowEdge struct {
	// from is the source node id
	From string `json:"from"`

	// to is the target node id
	To string `json:"to"`

	// edgeType describes the flow type (e.g. internal, cross-ns, cron, database, messaging, external)
	EdgeType string `json:"edgeType"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Nodes",type=integer,JSONPath=`.status.nodeCount`
// +kubebuilder:printcolumn:name="Edges",type=integer,JSONPath=`.status.edgeCount`
// +kubebuilder:printcolumn:name="Last Reconcile",type=date,JSONPath=`.status.lastReconcileTime`

// NetworkFlowDiscovery is the Schema for the networkflowdiscoveries API.
// It discovers network flows from Kubernetes NetworkPolicies and FQDNNetworkPolicies.
type NetworkFlowDiscovery struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of NetworkFlowDiscovery
	// +required
	Spec NetworkFlowDiscoverySpec `json:"spec"`

	// status defines the observed state of NetworkFlowDiscovery
	// +optional
	Status NetworkFlowDiscoveryStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// NetworkFlowDiscoveryList contains a list of NetworkFlowDiscovery.
type NetworkFlowDiscoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkFlowDiscovery `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkFlowDiscovery{}, &NetworkFlowDiscoveryList{})
}
