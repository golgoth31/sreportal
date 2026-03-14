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
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

const (
	// DefaultConfigPath is the default path for the configuration file.
	DefaultConfigPath = "/etc/sreportal/config.yaml"
)

// LoadFromFile reads the operator configuration from a file path.
// Returns the default configuration if the file doesn't exist.
func LoadFromFile(path string) (*OperatorConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// LogConfig returns a summary of the configuration for logging purposes.
func (c *OperatorConfig) LogSummary() map[string]any {
	summary := map[string]any{
		"reconciliation.interval":        c.Reconciliation.Interval.Duration().String(),
		"reconciliation.retryOnError":    c.Reconciliation.RetryOnError.Duration().String(),
		"reconciliation.disableDNSCheck": c.Reconciliation.DisableDNSCheck,
		"groupMapping.defaultGroup":      c.GroupMapping.DefaultGroup,
		"sources.priority":               c.Sources.Priority,
	}

	if c.Sources.Service != nil {
		summary["sources.service.enabled"] = c.Sources.Service.Enabled
		summary["sources.service.namespace"] = c.Sources.Service.Namespace
		summary["sources.service.annotationFilter"] = c.Sources.Service.AnnotationFilter
		summary["sources.service.serviceTypeFilter"] = c.Sources.Service.ServiceTypeFilter
	} else {
		summary["sources.service"] = nil
	}

	if c.Sources.Ingress != nil {
		summary["sources.ingress.enabled"] = c.Sources.Ingress.Enabled
		summary["sources.ingress.namespace"] = c.Sources.Ingress.Namespace
		summary["sources.ingress.ingressClassNames"] = c.Sources.Ingress.IngressClassNames
	} else {
		summary["sources.ingress"] = nil
	}

	if c.Sources.DNSEndpoint != nil {
		summary["sources.dnsEndpoint.enabled"] = c.Sources.DNSEndpoint.Enabled
		summary["sources.dnsEndpoint.namespace"] = c.Sources.DNSEndpoint.Namespace
	} else {
		summary["sources.dnsEndpoint"] = nil
	}

	if c.Sources.IstioGateway != nil {
		summary["sources.istioGateway.enabled"] = c.Sources.IstioGateway.Enabled
		summary["sources.istioGateway.namespace"] = c.Sources.IstioGateway.Namespace
		summary["sources.istioGateway.annotationFilter"] = c.Sources.IstioGateway.AnnotationFilter
	} else {
		summary["sources.istioGateway"] = nil
	}

	if c.Sources.IstioVirtualService != nil {
		summary["sources.istioVirtualService.enabled"] = c.Sources.IstioVirtualService.Enabled
		summary["sources.istioVirtualService.namespace"] = c.Sources.IstioVirtualService.Namespace
		summary["sources.istioVirtualService.annotationFilter"] = c.Sources.IstioVirtualService.AnnotationFilter
	} else {
		summary["sources.istioVirtualService"] = nil
	}

	if c.Sources.GatewayHTTPRoute != nil {
		summary["sources.gatewayHTTPRoute.enabled"] = c.Sources.GatewayHTTPRoute.Enabled
		summary["sources.gatewayHTTPRoute.namespace"] = c.Sources.GatewayHTTPRoute.Namespace
		summary["sources.gatewayHTTPRoute.gatewayName"] = c.Sources.GatewayHTTPRoute.GatewayName
		summary["sources.gatewayHTTPRoute.gatewayNamespace"] = c.Sources.GatewayHTTPRoute.GatewayNamespace
	} else {
		summary["sources.gatewayHTTPRoute"] = nil
	}

	if c.Sources.GatewayGRPCRoute != nil {
		summary["sources.gatewayGRPCRoute.enabled"] = c.Sources.GatewayGRPCRoute.Enabled
		summary["sources.gatewayGRPCRoute.namespace"] = c.Sources.GatewayGRPCRoute.Namespace
		summary["sources.gatewayGRPCRoute.gatewayName"] = c.Sources.GatewayGRPCRoute.GatewayName
		summary["sources.gatewayGRPCRoute.gatewayNamespace"] = c.Sources.GatewayGRPCRoute.GatewayNamespace
	} else {
		summary["sources.gatewayGRPCRoute"] = nil
	}

	if c.Sources.GatewayTLSRoute != nil {
		summary["sources.gatewayTLSRoute.enabled"] = c.Sources.GatewayTLSRoute.Enabled
		summary["sources.gatewayTLSRoute.namespace"] = c.Sources.GatewayTLSRoute.Namespace
		summary["sources.gatewayTLSRoute.gatewayName"] = c.Sources.GatewayTLSRoute.GatewayName
		summary["sources.gatewayTLSRoute.gatewayNamespace"] = c.Sources.GatewayTLSRoute.GatewayNamespace
	} else {
		summary["sources.gatewayTLSRoute"] = nil
	}

	if c.Sources.GatewayTCPRoute != nil {
		summary["sources.gatewayTCPRoute.enabled"] = c.Sources.GatewayTCPRoute.Enabled
		summary["sources.gatewayTCPRoute.namespace"] = c.Sources.GatewayTCPRoute.Namespace
		summary["sources.gatewayTCPRoute.gatewayName"] = c.Sources.GatewayTCPRoute.GatewayName
		summary["sources.gatewayTCPRoute.gatewayNamespace"] = c.Sources.GatewayTCPRoute.GatewayNamespace
	} else {
		summary["sources.gatewayTCPRoute"] = nil
	}

	if c.Sources.GatewayUDPRoute != nil {
		summary["sources.gatewayUDPRoute.enabled"] = c.Sources.GatewayUDPRoute.Enabled
		summary["sources.gatewayUDPRoute.namespace"] = c.Sources.GatewayUDPRoute.Namespace
		summary["sources.gatewayUDPRoute.gatewayName"] = c.Sources.GatewayUDPRoute.GatewayName
		summary["sources.gatewayUDPRoute.gatewayNamespace"] = c.Sources.GatewayUDPRoute.GatewayNamespace
	} else {
		summary["sources.gatewayUDPRoute"] = nil
	}

	return summary
}
