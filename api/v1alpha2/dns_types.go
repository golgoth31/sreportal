package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SourceFilterDefaults defines fallback filter values applied to every source
// in this DNS CR. A source's own Namespace/LabelFilter, when non-empty,
// overrides the corresponding default.
type SourceFilterDefaults struct {
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +optional
	LabelFilter string `json:"labelFilter,omitempty"`
}

type SourcesSpec struct {
	Service                  *ServiceSourceSpec                  `json:"service,omitempty"`
	Ingress                  *IngressSourceSpec                  `json:"ingress,omitempty"`
	DNSEndpoint              *DNSEndpointSourceSpec              `json:"dnsEndpoint,omitempty"`
	IstioGateway             *IstioGatewaySourceSpec             `json:"istioGateway,omitempty"`
	IstioVirtualService      *IstioVirtualServiceSourceSpec      `json:"istioVirtualService,omitempty"`
	GatewayHTTPRoute         *GatewayRouteSourceSpec             `json:"gatewayHTTPRoute,omitempty"`
	GatewayGRPCRoute         *GatewayRouteSourceSpec             `json:"gatewayGRPCRoute,omitempty"`
	GatewayTLSRoute          *GatewayRouteSourceSpec             `json:"gatewayTLSRoute,omitempty"`
	GatewayTCPRoute          *GatewayRouteSourceSpec             `json:"gatewayTCPRoute,omitempty"`
	GatewayUDPRoute          *GatewayRouteSourceSpec             `json:"gatewayUDPRoute,omitempty"`
	CrossplaneScalewayRecord *CrossplaneScalewayRecordSourceSpec `json:"crossplaneScalewayRecord,omitempty"`
	// +optional
	Priority []SourceType `json:"priority,omitempty"`
}

type ServiceSourceSpec struct {
	CommonSourceSpec  `json:",inline"`
	PublishInternal   bool     `json:"publishInternal,omitempty"`
	PublishHostIP     bool     `json:"publishHostIP,omitempty"`
	ServiceTypeFilter []string `json:"serviceTypeFilter,omitempty"`
}

type IngressSourceSpec struct {
	CommonSourceSpec  `json:",inline"`
	IngressClassNames []string `json:"ingressClassNames,omitempty"`
}

type DNSEndpointSourceSpec struct {
	// +kubebuilder:default=false
	Enabled     bool   `json:"enabled"`
	Namespace   string `json:"namespace,omitempty"`
	LabelFilter string `json:"labelFilter,omitempty"`
}

type IstioGatewaySourceSpec struct {
	CommonSourceSpec `json:",inline"`
}

type IstioVirtualServiceSourceSpec struct {
	CommonSourceSpec `json:",inline"`
}

type GatewayRouteSourceSpec struct {
	CommonSourceSpec   `json:",inline"`
	GatewayName        string `json:"gatewayName,omitempty"`
	GatewayNamespace   string `json:"gatewayNamespace,omitempty"`
	GatewayLabelFilter string `json:"gatewayLabelFilter,omitempty"`
}

type CrossplaneScalewayRecordSourceSpec struct {
	// +kubebuilder:default=false
	Enabled       bool   `json:"enabled"`
	Namespace     string `json:"namespace,omitempty"`
	LabelFilter   string `json:"labelFilter,omitempty"`
	ClusterScoped bool   `json:"clusterScoped,omitempty"`
}

// GroupMappingSpec configures how FQDNs are organised into groups in the UI.
type GroupMappingSpec struct {
	// +kubebuilder:default="Services"
	// +kubebuilder:validation:MinLength=1
	DefaultGroup string `json:"defaultGroup"`
	// +optional
	LabelKey string `json:"labelKey,omitempty"`
	// +optional
	ByNamespace map[string]string `json:"byNamespace,omitempty"`
}

// ReconciliationSpec controls timing of the source poll loop.
type ReconciliationSpec struct {
	// +kubebuilder:default="5m"
	Interval metav1.Duration `json:"interval"`
	// +kubebuilder:default="30s"
	RetryOnError    metav1.Duration `json:"retryOnError"`
	DisableDNSCheck bool            `json:"disableDNSCheck,omitempty"`
}

// DNSSpec defines the desired state of DNS (v1alpha2).
// Multiple DNS CRs may reference the same Portal via spec.portalRef
// (1 portal → N DNS CRs, e.g. per-team split).
type DNSSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.portalRef is immutable"
	PortalRef string `json:"portalRef"`

	// +optional
	IsRemote bool `json:"isRemote,omitempty"`

	// +optional
	Defaults SourceFilterDefaults `json:"defaults,omitempty"`

	// +optional
	Sources SourcesSpec `json:"sources,omitempty"`

	// +kubebuilder:default={defaultGroup:"Services"}
	// +optional
	GroupMapping GroupMappingSpec `json:"groupMapping,omitempty"`

	// +kubebuilder:default={interval:"5m",retryOnError:"30s"}
	// +optional
	Reconciliation ReconciliationSpec `json:"reconciliation,omitempty"`
}

// DNSStatus defines the observed state of DNS (v1alpha2).
type DNSStatus struct {
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	LastReconcileTime  *metav1.Time       `json:"lastReconcileTime,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	ActiveSources      []string           `json:"activeSources,omitempty"`
	NextReconcileTime  *metav1.Time       `json:"nextReconcileTime,omitempty"`

	// skippedEntries lists the discovered entries dropped on the last reconcile
	// because they failed DNSRecord validation (FQDN pattern or record-type
	// enum). They are excluded from the produced DNSRecords instead of aborting
	// the whole reconcile. The list is a bounded sample; the full count is
	// carried by the EntriesValid condition and the dns_entries_invalid_total
	// metric.
	// +optional
	// +kubebuilder:validation:MaxItems=100
	SkippedEntries []SkippedFQDNStatus `json:"skippedEntries,omitempty"`
}

// SkippedFQDNStatus describes a single entry dropped during validation.
type SkippedFQDNStatus struct {
	// fqdn is the offending fully qualified domain name (truncated to the DNS
	// name length limit).
	// +kubebuilder:validation:MaxLength=253
	FQDN string `json:"fqdn"`

	// sourceType is the source kind that produced the entry.
	// +optional
	SourceType string `json:"sourceType,omitempty"`

	// recordType is the DNS record type of the dropped entry (truncated). It is
	// bounded because a dropped entry's record type is source-controlled and
	// unbounded (e.g. a Crossplane Scaleway Record type), and an unbounded value
	// could bloat the status object past the etcd size limit.
	// +optional
	// +kubebuilder:validation:MaxLength=16
	RecordType string `json:"recordType,omitempty"`

	// reason is a short machine-friendly cause (e.g. invalid_fqdn).
	Reason string `json:"reason"`
}

// FQDNGroupStatus represents a group of FQDNs in the status
type FQDNGroupStatus struct {
	// name is the group name
	Name string `json:"name"`

	// description is the group description
	// +optional
	Description string `json:"description,omitempty"`

	// source indicates where this group came from (manual, external-dns, or remote)
	Source FQDNGroupSource `json:"source"`

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
	// +optional
	SyncStatus SyncStatus `json:"syncStatus,omitempty"`

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
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DNS is the Schema for the dns API
type DNS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DNS
	// +required
	Spec DNSSpec `json:"spec"`

	// status defines the observed state of DNS
	// +optional
	Status DNSStatus `json:"status,omitzero"`
}

func (*DNS) Hub() {}

// +kubebuilder:object:root=true
type DNSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DNS `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNS{}, &DNSList{})
}
