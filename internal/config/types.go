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

package config

import (
	"encoding/json"
	"fmt"
	"time"
)

// Duration is a wrapper around time.Duration that supports YAML/JSON unmarshaling from strings.
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler for Duration.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
	case string:
		duration, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(duration)
	}
	return nil
}

// MarshalJSON implements json.Marshaler for Duration.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// Duration returns the time.Duration value.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// OperatorConfig represents the complete operator configuration from ConfigMap.
type OperatorConfig struct {
	Sources        SourcesConfig        `json:"sources" yaml:"sources"`
	GroupMapping   GroupMappingConfig   `json:"groupMapping" yaml:"groupMapping"`
	Reconciliation ReconciliationConfig `json:"reconciliation" yaml:"reconciliation"`
	Release        ReleaseConfig        `json:"release,omitempty" yaml:"release,omitempty"`
	Auth           AuthConfig           `json:"auth,omitempty" yaml:"auth,omitempty"`
	Emoji          *EmojiConfig         `json:"emoji,omitempty" yaml:"emoji,omitempty"`
}

// AuthConfig configures authentication for write endpoints.
type AuthConfig struct {
	APIKey *APIKeyAuthConfig `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	JWT    *JWTAuthConfig    `json:"jwt,omitempty" yaml:"jwt,omitempty"`
}

// Enabled returns true if at least one authentication method is enabled.
func (c *AuthConfig) Enabled() bool {
	if c.APIKey != nil && c.APIKey.Enabled {
		return true
	}
	if c.JWT != nil && c.JWT.Enabled {
		return true
	}
	return false
}

// APIKeyAuthConfig configures header-based API key authentication.
// The actual key value is read from the HEADER_API_KEY environment variable.
type APIKeyAuthConfig struct {
	// Enabled controls whether API key authentication is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// HeaderName is the HTTP header to check (default: "X-API-Key").
	HeaderName string `json:"headerName,omitempty" yaml:"headerName,omitempty"`
}

// JWTAuthConfig configures JWT Bearer token authentication.
type JWTAuthConfig struct {
	// Enabled controls whether JWT authentication is active.
	Enabled bool              `json:"enabled" yaml:"enabled"`
	Issuers []JWTIssuerConfig `json:"issuers" yaml:"issuers"`
}

// JWTIssuerConfig configures a single JWT issuer.
type JWTIssuerConfig struct {
	Name           string            `json:"name" yaml:"name"`
	IssuerURL      string            `json:"issuerURL" yaml:"issuerURL"`
	Audience       string            `json:"audience,omitempty" yaml:"audience,omitempty"`
	JWKSURL        string            `json:"jwksURL" yaml:"jwksURL"`
	RequiredClaims map[string]string `json:"requiredClaims,omitempty" yaml:"requiredClaims,omitempty"`
}

// EmojiConfig configures custom emoji resolution from external sources.
type EmojiConfig struct {
	Slack *SlackEmojiConfig `json:"slack,omitempty" yaml:"slack,omitempty"`
}

// SlackEmojiConfig configures Slack custom emoji fetching.
// The actual Slack API token is read from the SLACK_API_TOKEN environment variable.
type SlackEmojiConfig struct {
	// Enabled controls whether Slack emoji fetching is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// RefreshInterval is the time between Slack emoji list refreshes (default: 24h).
	RefreshInterval Duration `json:"refreshInterval,omitempty" yaml:"refreshInterval,omitempty"`
}

// ReleaseConfig configures the Release CRD feature.
type ReleaseConfig struct {
	// TTL is how long Release CRs are kept before cleanup (default: 720h = 30 days).
	// The release controller checks expired CRs every 12 hours.
	TTL Duration `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	// Namespace is the namespace for Release CRs (defaults to operator namespace).
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// Types is the list of release allowed types.
	Types []ReleaseTypeConfig `json:"types,omitempty" yaml:"types,omitempty"`
}

// ReleaseTypeConfig configures the Release CRD feature.
type ReleaseTypeConfig struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Color string `json:"color,omitempty" yaml:"color,omitempty"`
}

// SourcesConfig enables and configures each source type.
type SourcesConfig struct {
	Service                  *ServiceConfig                  `json:"service,omitempty" yaml:"service,omitempty"`
	Ingress                  *IngressConfig                  `json:"ingress,omitempty" yaml:"ingress,omitempty"`
	DNSEndpoint              *DNSEndpointConfig              `json:"dnsEndpoint,omitempty" yaml:"dnsEndpoint,omitempty"`
	IstioGateway             *IstioGatewayConfig             `json:"istioGateway,omitempty" yaml:"istioGateway,omitempty"`
	IstioVirtualService      *IstioVirtualServiceConfig      `json:"istioVirtualService,omitempty" yaml:"istioVirtualService,omitempty"`
	GatewayHTTPRoute         *GatewayRouteConfig             `json:"gatewayHTTPRoute,omitempty" yaml:"gatewayHTTPRoute,omitempty"`
	GatewayGRPCRoute         *GatewayRouteConfig             `json:"gatewayGRPCRoute,omitempty" yaml:"gatewayGRPCRoute,omitempty"`
	GatewayTLSRoute          *GatewayRouteConfig             `json:"gatewayTLSRoute,omitempty" yaml:"gatewayTLSRoute,omitempty"`
	GatewayTCPRoute          *GatewayRouteConfig             `json:"gatewayTCPRoute,omitempty" yaml:"gatewayTCPRoute,omitempty"`
	GatewayUDPRoute          *GatewayRouteConfig             `json:"gatewayUDPRoute,omitempty" yaml:"gatewayUDPRoute,omitempty"`
	CrossplaneScalewayRecord *CrossplaneScalewayRecordConfig `json:"crossplaneScalewayRecord,omitempty" yaml:"crossplaneScalewayRecord,omitempty"`
	// Priority defines the preferred order of source types when the same FQDN+RecordType
	// is discovered by multiple sources. Sources listed earlier take precedence over later ones.
	// When a source is not listed, it receives the lowest priority. When empty, targets from
	// all sources are merged (backward-compatible default).
	// Valid values: "service", "ingress", "dnsendpoint", "istio-gateway", "istio-virtualservice",
	// "gateway-httproute", "gateway-grpcroute", "gateway-tlsroute", "gateway-tcproute", "gateway-udproute",
	// "crossplane-scaleway-record".
	Priority []string `json:"priority,omitempty" yaml:"priority,omitempty"`
}

// ServiceConfig maps to source.Config fields for Kubernetes Services.
type ServiceConfig struct {
	// Enabled controls whether Service source is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Namespace restricts watching to a specific namespace. Empty means all namespaces.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// AnnotationFilter filters Services by annotation (e.g., "external-dns.alpha.kubernetes.io/hostname").
	AnnotationFilter string `json:"annotationFilter,omitempty" yaml:"annotationFilter,omitempty"`
	// LabelFilter filters Services by label selector.
	LabelFilter string `json:"labelFilter,omitempty" yaml:"labelFilter,omitempty"`
	// ServiceTypeFilter restricts to specific Service types (e.g., LoadBalancer, ClusterIP).
	ServiceTypeFilter []string `json:"serviceTypeFilter,omitempty" yaml:"serviceTypeFilter,omitempty"`
	// PublishInternal publishes internal IPs instead of external ones.
	PublishInternal bool `json:"publishInternal,omitempty" yaml:"publishInternal,omitempty"`
	// PublishHostIP publishes host IP addresses.
	PublishHostIP bool `json:"publishHostIP,omitempty" yaml:"publishHostIP,omitempty"`
	// FQDNTemplate is a Go template for generating hostnames.
	FQDNTemplate string `json:"fqdnTemplate,omitempty" yaml:"fqdnTemplate,omitempty"`
	// CombineFQDNAndAnnotation combines template and annotation hostnames.
	CombineFQDNAndAnnotation bool `json:"combineFqdnAndAnnotation,omitempty" yaml:"combineFqdnAndAnnotation,omitempty"`
	// IgnoreHostnameAnnotation ignores hostname annotations.
	IgnoreHostnameAnnotation bool `json:"ignoreHostnameAnnotation,omitempty" yaml:"ignoreHostnameAnnotation,omitempty"`
	// ResolveLoadBalancerHostname resolves LB hostnames to IPs.
	ResolveLoadBalancerHostname bool `json:"resolveLoadBalancerHostname,omitempty" yaml:"resolveLoadBalancerHostname,omitempty"`
}

// IngressConfig maps to source.Config fields for Kubernetes Ingresses.
type IngressConfig struct {
	// Enabled controls whether Ingress source is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Namespace restricts watching to a specific namespace. Empty means all namespaces.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// AnnotationFilter filters Ingresses by annotation.
	AnnotationFilter string `json:"annotationFilter,omitempty" yaml:"annotationFilter,omitempty"`
	// LabelFilter filters Ingresses by label selector.
	LabelFilter string `json:"labelFilter,omitempty" yaml:"labelFilter,omitempty"`
	// IngressClassNames restricts to specific Ingress classes.
	IngressClassNames []string `json:"ingressClassNames,omitempty" yaml:"ingressClassNames,omitempty"`
	// FQDNTemplate is a Go template for generating hostnames.
	FQDNTemplate string `json:"fqdnTemplate,omitempty" yaml:"fqdnTemplate,omitempty"`
	// CombineFQDNAndAnnotation combines template and annotation hostnames.
	CombineFQDNAndAnnotation bool `json:"combineFqdnAndAnnotation,omitempty" yaml:"combineFqdnAndAnnotation,omitempty"`
	// IgnoreHostnameAnnotation ignores hostname annotations.
	IgnoreHostnameAnnotation bool `json:"ignoreHostnameAnnotation,omitempty" yaml:"ignoreHostnameAnnotation,omitempty"`
	// IgnoreIngressTLSSpec ignores TLS spec when extracting hostnames.
	IgnoreIngressTLSSpec bool `json:"ignoreIngressTlsSpec,omitempty" yaml:"ignoreIngressTlsSpec,omitempty"`
	// IgnoreIngressRulesSpec ignores rules spec when extracting hostnames.
	IgnoreIngressRulesSpec bool `json:"ignoreIngressRulesSpec,omitempty" yaml:"ignoreIngressRulesSpec,omitempty"`
	// ResolveLoadBalancerHostname resolves LB hostnames to IPs.
	ResolveLoadBalancerHostname bool `json:"resolveLoadBalancerHostname,omitempty" yaml:"resolveLoadBalancerHostname,omitempty"`
}

// DNSEndpointConfig configures the external-dns CRD source.
type DNSEndpointConfig struct {
	// Enabled controls whether DNSEndpoint CRD source is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Namespace restricts watching to a specific namespace. Empty means all namespaces.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// IstioGatewayConfig configures the Istio Gateway source.
type IstioGatewayConfig struct {
	// Enabled controls whether Istio Gateway source is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Namespace restricts watching to a specific namespace. Empty means all namespaces.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// AnnotationFilter filters Gateways by annotation.
	AnnotationFilter string `json:"annotationFilter,omitempty" yaml:"annotationFilter,omitempty"`
	// FQDNTemplate is a Go template for generating hostnames.
	FQDNTemplate string `json:"fqdnTemplate,omitempty" yaml:"fqdnTemplate,omitempty"`
	// CombineFQDNAndAnnotation combines template and annotation hostnames.
	CombineFQDNAndAnnotation bool `json:"combineFqdnAndAnnotation,omitempty" yaml:"combineFqdnAndAnnotation,omitempty"`
	// IgnoreHostnameAnnotation ignores hostname annotations.
	IgnoreHostnameAnnotation bool `json:"ignoreHostnameAnnotation,omitempty" yaml:"ignoreHostnameAnnotation,omitempty"`
}

// IstioVirtualServiceConfig configures the Istio VirtualService source.
type IstioVirtualServiceConfig struct {
	// Enabled controls whether Istio VirtualService source is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Namespace restricts watching to a specific namespace. Empty means all namespaces.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// AnnotationFilter filters VirtualServices by annotation.
	AnnotationFilter string `json:"annotationFilter,omitempty" yaml:"annotationFilter,omitempty"`
	// FQDNTemplate is a Go template for generating hostnames.
	FQDNTemplate string `json:"fqdnTemplate,omitempty" yaml:"fqdnTemplate,omitempty"`
	// CombineFQDNAndAnnotation combines template and annotation hostnames.
	CombineFQDNAndAnnotation bool `json:"combineFqdnAndAnnotation,omitempty" yaml:"combineFqdnAndAnnotation,omitempty"`
	// IgnoreHostnameAnnotation ignores hostname annotations.
	IgnoreHostnameAnnotation bool `json:"ignoreHostnameAnnotation,omitempty" yaml:"ignoreHostnameAnnotation,omitempty"`
}

// GatewayRouteConfig configures a Gateway API route source (HTTPRoute, GRPCRoute, TLSRoute, TCPRoute, UDPRoute).
// All five route types share the same configuration fields.
type GatewayRouteConfig struct {
	// Enabled controls whether this Gateway route source is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Namespace restricts watching routes to a specific namespace. Empty means all namespaces.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// AnnotationFilter filters routes by annotation.
	AnnotationFilter string `json:"annotationFilter,omitempty" yaml:"annotationFilter,omitempty"`
	// LabelFilter filters routes by label selector.
	LabelFilter string `json:"labelFilter,omitempty" yaml:"labelFilter,omitempty"`
	// FQDNTemplate is a Go template for generating hostnames.
	FQDNTemplate string `json:"fqdnTemplate,omitempty" yaml:"fqdnTemplate,omitempty"`
	// CombineFQDNAndAnnotation combines template and annotation hostnames.
	CombineFQDNAndAnnotation bool `json:"combineFqdnAndAnnotation,omitempty" yaml:"combineFqdnAndAnnotation,omitempty"`
	// IgnoreHostnameAnnotation ignores hostname annotations.
	IgnoreHostnameAnnotation bool `json:"ignoreHostnameAnnotation,omitempty" yaml:"ignoreHostnameAnnotation,omitempty"`
	// GatewayName filters to a specific Gateway name. Empty means all Gateways.
	GatewayName string `json:"gatewayName,omitempty" yaml:"gatewayName,omitempty"`
	// GatewayNamespace is the namespace for Gateway resources. Empty means all namespaces.
	GatewayNamespace string `json:"gatewayNamespace,omitempty" yaml:"gatewayNamespace,omitempty"`
	// GatewayLabelFilter filters Gateway resources by label selector.
	GatewayLabelFilter string `json:"gatewayLabelFilter,omitempty" yaml:"gatewayLabelFilter,omitempty"`
}

// CrossplaneScalewayRecordConfig configures the Crossplane Scaleway DNS Record source.
type CrossplaneScalewayRecordConfig struct {
	// Enabled controls whether Crossplane Scaleway Record source is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Namespace restricts watching to a specific namespace. Empty means all namespaces.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// LabelFilter filters Records by label selector.
	LabelFilter string `json:"labelFilter,omitempty" yaml:"labelFilter,omitempty"`
	// ClusterScoped indicates the Record CRD is cluster-scoped (not namespaced).
	ClusterScoped bool `json:"clusterScoped,omitempty" yaml:"clusterScoped,omitempty"`
}

// GroupMappingConfig configures how FQDNs are organized into groups for the UI.
type GroupMappingConfig struct {
	// DefaultGroup is the group name for FQDNs that don't match any mapping rules.
	DefaultGroup string `json:"defaultGroup" yaml:"defaultGroup"`
	// LabelKey is the endpoint label key to use for grouping (e.g., "sreportal.io/group").
	LabelKey string `json:"labelKey,omitempty" yaml:"labelKey,omitempty"`
	// ByNamespace maps Kubernetes namespaces to group names.
	ByNamespace map[string]string `json:"byNamespace,omitempty" yaml:"byNamespace,omitempty"`
}

// ReconciliationConfig controls reconciliation timing.
type ReconciliationConfig struct {
	// Interval is the time between full reconciliations.
	Interval Duration `json:"interval" yaml:"interval"`
	// RetryOnError is the delay before retrying after an error.
	RetryOnError Duration `json:"retryOnError" yaml:"retryOnError"`
	// DisableDNSCheck disables live DNS resolution during reconciliation.
	// When true, FQDNs will not have a SyncStatus populated and the
	// ResolveDNSHandler step is skipped entirely. Useful when the operator
	// runs without outbound DNS access or to reduce reconciliation latency.
	DisableDNSCheck bool `json:"disableDNSCheck,omitempty" yaml:"disableDNSCheck,omitempty"`
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *OperatorConfig {
	return &OperatorConfig{
		Sources: SourcesConfig{},
		GroupMapping: GroupMappingConfig{
			DefaultGroup: "Services",
		},
		Reconciliation: ReconciliationConfig{
			Interval:     Duration(5 * time.Minute),
			RetryOnError: Duration(30 * time.Second),
		},
		Release: ReleaseConfig{
			TTL: Duration(30 * 24 * time.Hour),
		},
	}
}

// Validate checks that the configuration is internally consistent.
func (c *OperatorConfig) Validate() error {
	if c.Reconciliation.Interval.Duration() <= 0 {
		return fmt.Errorf("reconciliation.interval: %w", ErrInvalidInterval)
	}
	if c.GroupMapping.DefaultGroup == "" {
		return fmt.Errorf("groupMapping.defaultGroup: %w", ErrEmptyDefaultGroup)
	}
	if err := c.Auth.validate(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	return nil
}

func (c *AuthConfig) validate() error {
	if c.JWT != nil && c.JWT.Enabled {
		if len(c.JWT.Issuers) == 0 {
			return fmt.Errorf("jwt: at least one issuer is required")
		}
		for i, iss := range c.JWT.Issuers {
			if iss.IssuerURL == "" {
				return fmt.Errorf("jwt.issuers[%d]: issuerURL is required", i)
			}
			if iss.JWKSURL == "" {
				return fmt.Errorf("jwt.issuers[%d]: jwksURL is required", i)
			}
		}
	}
	return nil
}
