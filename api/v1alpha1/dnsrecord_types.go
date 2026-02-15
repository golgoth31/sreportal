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

// DNSRecordSpec defines the desired state of DNSRecord
type DNSRecordSpec struct {
	// sourceType indicates the external-dns source type that provides this record
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=service;ingress;dnsendpoint;istio-gateway;istio-virtualservice
	SourceType string `json:"sourceType"`

	// portalRef is the name of the Portal this record belongs to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`
}

// DNSRecordStatus defines the observed state of DNSRecord
type DNSRecordStatus struct {
	// endpoints contains the DNS endpoints discovered from this source
	// +optional
	Endpoints []EndpointStatus `json:"endpoints,omitempty"`

	// lastReconcileTime is the timestamp of the last reconciliation
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// conditions represent the current state of the DNSRecord resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// EndpointStatus represents a single DNS endpoint discovered from external-dns
type EndpointStatus struct {
	// dnsName is the fully qualified domain name
	// +kubebuilder:validation:Required
	DNSName string `json:"dnsName"`

	// recordType is the DNS record type (A, AAAA, CNAME, TXT, etc.)
	// +optional
	RecordType string `json:"recordType,omitempty"`

	// targets is the list of target addresses for this endpoint
	// +optional
	Targets []string `json:"targets,omitempty"`

	// ttl is the DNS record TTL in seconds
	// +optional
	TTL int64 `json:"ttl,omitempty"`

	// labels contains the endpoint labels from external-dns
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// lastSeen is the timestamp when this endpoint was last observed
	// +kubebuilder:validation:Required
	LastSeen metav1.Time `json:"lastSeen"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=dnsrecords,scope=Namespaced,shortName=dnsrec
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.sourceType`
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Endpoints",type=integer,JSONPath=`.status.endpoints`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DNSRecord is the Schema for the dnsrecords API.
// It represents DNS endpoints discovered from a specific external-dns source.
type DNSRecord struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DNSRecord
	// +required
	Spec DNSRecordSpec `json:"spec"`

	// status defines the observed state of DNSRecord
	// +optional
	Status DNSRecordStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DNSRecordList contains a list of DNSRecord
type DNSRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DNSRecord `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNSRecord{}, &DNSRecordList{})
}
