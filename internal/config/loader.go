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

	return cfg, nil
}

// LogConfig returns a summary of the configuration for logging purposes.
func (c *OperatorConfig) LogSummary() map[string]any {
	summary := map[string]any{
		"reconciliation.interval":     c.Reconciliation.Interval.Duration().String(),
		"reconciliation.retryOnError": c.Reconciliation.RetryOnError.Duration().String(),
		"groupMapping.defaultGroup":   c.GroupMapping.DefaultGroup,
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

	return summary
}
