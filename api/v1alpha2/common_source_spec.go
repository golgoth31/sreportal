package v1alpha2

// SourceType identifies an external-dns source kind referenced by DNSRecord.spec.sourceType
// and by SourcesSpec.Priority.
// +kubebuilder:validation:Enum=service;ingress;dnsendpoint;istio-gateway;istio-virtualservice;gateway-httproute;gateway-grpcroute;gateway-tlsroute;gateway-tcproute;gateway-udproute;crossplane-scaleway-record
type SourceType string

const (
	SourceTypeService                  SourceType = "service"
	SourceTypeIngress                  SourceType = "ingress"
	SourceTypeDNSEndpoint              SourceType = "dnsendpoint"
	SourceTypeIstioGateway             SourceType = "istio-gateway"
	SourceTypeIstioVirtualService      SourceType = "istio-virtualservice"
	SourceTypeGatewayHTTPRoute         SourceType = "gateway-httproute"
	SourceTypeGatewayGRPCRoute         SourceType = "gateway-grpcroute"
	SourceTypeGatewayTLSRoute          SourceType = "gateway-tlsroute"
	SourceTypeGatewayTCPRoute          SourceType = "gateway-tcproute"
	SourceTypeGatewayUDPRoute          SourceType = "gateway-udproute"
	SourceTypeCrossplaneScalewayRecord SourceType = "crossplane-scaleway-record"
)

// SyncStatus is the DNS-side resolution status of an FQDN.
// +kubebuilder:validation:Enum=sync;notavailable;notsync;""
type SyncStatus string

const (
	SyncStatusUnknown      SyncStatus = ""
	SyncStatusSync         SyncStatus = "sync"
	SyncStatusNotAvailable SyncStatus = "notavailable"
	SyncStatusNotSync      SyncStatus = "notsync"
)

// FQDNGroupSource indicates where an FQDN group came from.
// +kubebuilder:validation:Enum=manual;external-dns;remote
type FQDNGroupSource string

const (
	FQDNGroupSourceManual      FQDNGroupSource = "manual"
	FQDNGroupSourceExternalDNS FQDNGroupSource = "external-dns"
	FQDNGroupSourceRemote      FQDNGroupSource = "remote"
)

// DNS condition types.
const (
	ConditionReady           = "Ready"
	ConditionSourcesReady    = "SourcesReady"
	ConditionTargetsConflict = "TargetsConflict"
)

// CommonSourceSpec carries the fields shared by every external-dns source spec.
// Embed it with json:",inline" so the CRD schema remains flat (no nesting).
type CommonSourceSpec struct {
	// +kubebuilder:default=false
	Enabled                  bool   `json:"enabled"`
	Namespace                string `json:"namespace,omitempty"`
	AnnotationFilter         string `json:"annotationFilter,omitempty"`
	LabelFilter              string `json:"labelFilter,omitempty"`
	FQDNTemplate             string `json:"fqdnTemplate,omitempty"`
	CombineFQDNAndAnnotation bool   `json:"combineFqdnAndAnnotation,omitempty"`
	IgnoreHostnameAnnotation bool   `json:"ignoreHostnameAnnotation,omitempty"`
}
