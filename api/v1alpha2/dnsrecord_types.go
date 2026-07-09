package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DNSRecordOrigin discriminates auto-discovered vs manually created DNSRecord.
// +kubebuilder:validation:Enum=auto;manual
type DNSRecordOrigin string

const (
	DNSRecordOriginAuto   DNSRecordOrigin = "auto"
	DNSRecordOriginManual DNSRecordOrigin = "manual"
)

// DNSRecordSpec defines the desired state of DNSRecord (v1alpha2).
// +kubebuilder:validation:XValidation:rule="self.origin == 'auto' ? has(self.sourceType) : !has(self.sourceType) && has(self.entries) && size(self.entries) > 0",message="auto records require sourceType; manual records require entries and no sourceType"
type DNSRecordSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=auto;manual
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.origin is immutable"
	Origin DNSRecordOrigin `json:"origin"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.portalRef is immutable"
	PortalRef string `json:"portalRef"`

	// Required when origin=auto. Must be empty when origin=manual.
	// +optional
	SourceType SourceType `json:"sourceType,omitempty"`

	// Endpoints projected for this DNSRecord.
	//
	// For origin=manual: required, set by the user (at least one entry).
	// For origin=auto: written exclusively by the operator's DNS controller
	// from the in-memory source store. The validating webhook reserves
	// updates of auto records to the controller ServiceAccount, so manual
	// edits by humans are rejected at admission. Any field stored here by
	// other means will be overwritten at the next DNS reconcile.
	// +optional
	// +listType=map
	// +listMapKey=fqdn
	// +listMapKey=recordType
	Entries []DNSRecordEntry `json:"entries,omitempty"`
}

// DNSRecordEntry is a single manual DNS entry.
type DNSRecordEntry struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// Pattern MUST stay byte-identical to domaindns.FQDNPattern
	// (internal/domain/dns/fqdn.go): the DNS controller pre-filters auto entries
	// with that expression so a single invalid FQDN doesn't get the whole
	// DNSRecord rejected at admission.
	// +kubebuilder:validation:Pattern=`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`
	FQDN string `json:"fqdn"`

	// +optional
	Group string `json:"group,omitempty"`

	// groups are the UI groups this entry belongs to (the sreportal.io/groups
	// annotation, comma-separated). Supports multiple groups, unlike the single
	// group field. Set by the DNS controller for origin=auto entries from the
	// source resource annotation; may be set directly on manual entries.
	// +optional
	Groups []string `json:"groups,omitempty"`

	// +optional
	Description string `json:"description,omitempty"`

	// +kubebuilder:validation:Enum=A;AAAA;CNAME;TXT
	// +kubebuilder:default="A"
	// +optional
	RecordType string `json:"recordType,omitempty"`

	// +optional
	Targets []string `json:"targets,omitempty"`

	// originRef identifies the source Kubernetes resource that produced this
	// entry, in "kind/namespace/name" form (the external-dns "resource" label).
	// Set by the DNS controller for origin=auto entries; empty for manual.
	// +optional
	OriginRef string `json:"originRef,omitempty"`
}

// DNSRecordStatus defines the observed state of DNSRecord (v1alpha2).
type DNSRecordStatus struct {
	Endpoints         []EndpointStatus `json:"endpoints,omitempty"`
	EndpointsHash     string           `json:"endpointsHash,omitempty"`
	LastReconcileTime *metav1.Time     `json:"lastReconcileTime,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
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

	// syncStatus indicates whether the endpoint is correctly resolved in DNS.
	// sync: the FQDN resolves to the expected type and targets.
	// notavailable: the FQDN does not exist in DNS.
	// notsync: the FQDN exists but resolves to different targets or type.
	// +optional
	SyncStatus SyncStatus `json:"syncStatus,omitempty"`

	// lastSeen is the timestamp when this endpoint was last observed
	// +kubebuilder:validation:Required
	LastSeen metav1.Time `json:"lastSeen"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=dnsrecords,scope=Namespaced,shortName=dnsrec
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Origin",type=string,JSONPath=`.spec.origin`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Endpoints",type=integer,JSONPath=`.status.endpoints`

// DNSRecord is the Schema for the dnsrecords API
type DNSRecord struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DNSRecord
	// +required
	Spec DNSRecordSpec `json:"spec"`

	// status defines the observed state of DNSRecord
	// +optional
	Status DNSRecordStatus `json:"status,omitzero"`
}

func (*DNSRecord) Hub() {}

// +kubebuilder:object:root=true
type DNSRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DNSRecord `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNSRecord{}, &DNSRecordList{})
}
