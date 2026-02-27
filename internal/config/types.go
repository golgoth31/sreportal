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
}

// SourcesConfig enables and configures each source type.
type SourcesConfig struct {
	Service             *ServiceConfig             `json:"service,omitempty" yaml:"service,omitempty"`
	Ingress             *IngressConfig             `json:"ingress,omitempty" yaml:"ingress,omitempty"`
	DNSEndpoint         *DNSEndpointConfig         `json:"dnsEndpoint,omitempty" yaml:"dnsEndpoint,omitempty"`
	IstioGateway        *IstioGatewayConfig        `json:"istioGateway,omitempty" yaml:"istioGateway,omitempty"`
	IstioVirtualService *IstioVirtualServiceConfig `json:"istioVirtualService,omitempty" yaml:"istioVirtualService,omitempty"`
	// Priority defines the preferred order of source types when the same FQDN+RecordType
	// is discovered by multiple sources. Sources listed earlier take precedence over later ones.
	// When a source is not listed, it receives the lowest priority. When empty, targets from
	// all sources are merged (backward-compatible default).
	// Valid values: "service", "ingress", "dnsendpoint", "istio-gateway", "istio-virtualservice".
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
	}
}
