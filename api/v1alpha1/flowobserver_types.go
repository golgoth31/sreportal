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

// FlowObserverSpec defines the desired state of FlowObserver.
type FlowObserverSpec struct {
	// portalRef is the name of the Portal this observer is linked to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`

	// prometheus configures the Prometheus-based flow observation
	// +required
	Prometheus FlowObserverPrometheusConfig `json:"prometheus"`

	// metrics defines the list of mesh metric descriptors to probe.
	// Each descriptor describes how a specific CNI/mesh exposes flow metrics in Prometheus.
	// When empty, the built-in defaults (Hubble, Istio, Linkerd) are used.
	// The observer probes each descriptor in order and uses the first one that returns results.
	// +optional
	Metrics []FlowMetricDescriptor `json:"metrics,omitempty"`
}

// FlowObserverPrometheusConfig configures the Prometheus connection.
type FlowObserverPrometheusConfig struct {
	// address is the Prometheus server URL
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Address string `json:"address"`

	// queryWindow is the PromQL range window for flow queries (e.g. "5m")
	// +kubebuilder:default="5m"
	// +optional
	QueryWindow string `json:"queryWindow,omitempty"`
}

// FlowMetricDescriptor describes how a specific CNI or service mesh exposes
// flow metrics in Prometheus. The observer probes each descriptor in order
// and uses the first one whose probeQuery returns results.
type FlowMetricDescriptor struct {
	// name is a human-readable identifier for this mesh/CNI (e.g. "istio", "hubble", "linkerd")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// probeQuery is a PromQL query used to detect if this mesh is active.
	// Should return a non-empty vector when the mesh is present (e.g. "count(istio_requests_total)").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ProbeQuery string `json:"probeQuery"`

	// observedQueryTemplate is a PromQL query template returning one result per source/destination pair.
	// Use %s as placeholder for the query window (e.g. "5m").
	// Must use "max by" with the label names defined below.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ObservedQueryTemplate string `json:"observedQueryTemplate"`

	// sourceNamespaceLabel is the Prometheus label name for the source namespace
	// +kubebuilder:validation:Required
	SourceNamespaceLabel string `json:"sourceNamespaceLabel"`

	// sourceWorkloadLabel is the Prometheus label name for the source workload
	// +kubebuilder:validation:Required
	SourceWorkloadLabel string `json:"sourceWorkloadLabel"`

	// destinationNamespaceLabel is the Prometheus label name for the destination namespace
	// +kubebuilder:validation:Required
	DestinationNamespaceLabel string `json:"destinationNamespaceLabel"`

	// destinationWorkloadLabel is the Prometheus label name for the destination workload
	// +kubebuilder:validation:Required
	DestinationWorkloadLabel string `json:"destinationWorkloadLabel"`
}

// FlowObserverStatus defines the observed state of FlowObserver.
type FlowObserverStatus struct {
	// activeMesh is the name of the detected mesh provider (empty if none detected)
	// +optional
	ActiveMesh string `json:"activeMesh,omitempty"`

	// conditions represent the current state of the FlowObserver resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Active Mesh",type=string,JSONPath=`.status.activeMesh`

// FlowObserver is the Schema for the flowobservers API.
// It configures how the operator detects real traffic on network edges
// by querying Prometheus for mesh/CNI flow metrics.
type FlowObserver struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of FlowObserver
	// +required
	Spec FlowObserverSpec `json:"spec"`

	// status defines the observed state of FlowObserver
	// +optional
	Status FlowObserverStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// FlowObserverList contains a list of FlowObserver.
type FlowObserverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FlowObserver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FlowObserver{}, &FlowObserverList{})
}

// DefaultMetricDescriptors returns the built-in mesh metric descriptors
// used when spec.metrics is empty.
func DefaultMetricDescriptors() []FlowMetricDescriptor {
	return []FlowMetricDescriptor{
		{
			Name:                      "hubble",
			ProbeQuery:                `count(hubble_flows_processed_total)`,
			ObservedQueryTemplate:     `max by (source_workload, source_namespace, destination_workload, destination_namespace) (max_over_time(hubble_flows_processed_total{verdict="FORWARDED"}[%s]))`,
			SourceNamespaceLabel:      "source_namespace",
			SourceWorkloadLabel:       "source_workload",
			DestinationNamespaceLabel: "destination_namespace",
			DestinationWorkloadLabel:  "destination_workload",
		},
		{
			Name:                      "istio",
			ProbeQuery:                `count(istio_requests_total)`,
			ObservedQueryTemplate:     `max by (source_workload, source_workload_namespace, destination_workload, destination_workload_namespace) (max_over_time(istio_requests_total{reporter="destination"}[%s]))`,
			SourceNamespaceLabel:      "source_workload_namespace",
			SourceWorkloadLabel:       "source_workload",
			DestinationNamespaceLabel: "destination_workload_namespace",
			DestinationWorkloadLabel:  "destination_workload",
		},
		{
			Name:                      "linkerd",
			ProbeQuery:                `count(request_total{direction="outbound"})`,
			ObservedQueryTemplate:     `max by (namespace, deployment, dst_namespace, dst_deployment) (max_over_time(request_total{direction="outbound"}[%s]))`,
			SourceNamespaceLabel:      "namespace",
			SourceWorkloadLabel:       "deployment",
			DestinationNamespaceLabel: "dst_namespace",
			DestinationWorkloadLabel:  "dst_deployment",
		},
	}
}
