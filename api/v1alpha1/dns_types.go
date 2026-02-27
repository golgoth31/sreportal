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

// DNSSpec defines the desired state of DNS
type DNSSpec struct {
	// portalRef is the name of the Portal this DNS resource is linked to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`

	// groups is a list of DNS entry groups for organizing entries in the UI
	// +optional
	Groups []DNSGroup `json:"groups,omitempty"`
}

// DNSGroup represents a group of DNS entries
type DNSGroup struct {
	// name is the display name for this group
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// description is an optional description for the group
	// +optional
	Description string `json:"description,omitempty"`

	// entries is a list of DNS entries in this group
	// +optional
	Entries []DNSEntry `json:"entries,omitempty"`
}

// DNSEntry represents a manual DNS entry
type DNSEntry struct {
	// fqdn is the fully qualified domain name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	FQDN string `json:"fqdn"`

	// description is an optional description for the DNS entry
	// +optional
	Description string `json:"description,omitempty"`
}

// DNSStatus defines the observed state of DNS.
type DNSStatus struct {
	// groups is the list of FQDN groups with their status
	// +optional
	Groups []FQDNGroupStatus `json:"groups,omitempty"`

	// conditions represent the current state of the DNS resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// lastReconcileTime is the timestamp of the last reconciliation
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
}

// FQDNGroupStatus represents a group of FQDNs in the status
type FQDNGroupStatus struct {
	// name is the group name
	Name string `json:"name"`

	// description is the group description
	// +optional
	Description string `json:"description,omitempty"`

	// source indicates where this group came from (manual, external-dns, or remote)
	// +kubebuilder:validation:Enum=manual;external-dns;remote
	Source string `json:"source"`

	// fqdns is the list of FQDNs in this group
	// +optional
	FQDNs []FQDNStatus `json:"fqdns,omitempty"`
}

// OriginResourceRef identifies the Kubernetes resource that produced an FQDN.
// Only populated for FQDNs discovered via external-dns sources.
type OriginResourceRef struct {
	// kind is the Kubernetes resource kind (e.g. Service, Ingress, DNSEndpoint)
	Kind string `json:"kind"`

	// namespace is the Kubernetes namespace of the origin resource
	Namespace string `json:"namespace"`

	// name is the name of the origin Kubernetes resource
	Name string `json:"name"`
}

// FQDNStatus represents the status of an aggregated FQDN
type FQDNStatus struct {
	// fqdn is the fully qualified domain name
	FQDN string `json:"fqdn"`

	// description is an optional description for the FQDN
	// +optional
	Description string `json:"description,omitempty"`

	// recordType is the DNS record type (A, AAAA, CNAME, etc.)
	// +optional
	RecordType string `json:"recordType,omitempty"`

	// targets is the list of target addresses for this FQDN
	// +optional
	Targets []string `json:"targets,omitempty"`

	// syncStatus indicates whether the FQDN is correctly resolved in DNS.
	// sync: the FQDN resolves to the expected type and targets.
	// notavailable: the FQDN does not exist in DNS.
	// notsync: the FQDN exists but resolves to different targets or type.
	// +kubebuilder:validation:Enum=sync;notavailable;notsync;""
	// +optional
	SyncStatus string `json:"syncStatus,omitempty"`

	// lastSeen is the timestamp when this FQDN was last observed
	LastSeen metav1.Time `json:"lastSeen"`

	// originRef identifies the Kubernetes resource (Service, Ingress, DNSEndpoint)
	// that produced this FQDN via external-dns. Not set for manual entries.
	// +optional
	OriginRef *OriginResourceRef `json:"originRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=dns,scope=Namespaced

// DNS is the Schema for the dns API
type DNS struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DNS
	// +required
	Spec DNSSpec `json:"spec"`

	// status defines the observed state of DNS
	// +optional
	Status DNSStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DNSList contains a list of DNS
type DNSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DNS `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNS{}, &DNSList{})
}
