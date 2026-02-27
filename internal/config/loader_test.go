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
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFromFile(t *testing.T) {
	// Create a temp config file
	content := `
sources:
  service:
    enabled: true
    annotationFilter: "external-dns.alpha.kubernetes.io/hostname"
    serviceTypeFilter:
      - LoadBalancer

  ingress:
    enabled: true
    ingressClassNames:
      - nginx

groupMapping:
  defaultGroup: "Discovered Services"
  labelKey: "sreportal.io/group"
  byNamespace:
    production: "Production"
    test-app: "Test Applications"

reconciliation:
  interval: 30s
  retryOnError: 10s
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Verify service config
	if cfg.Sources.Service == nil {
		t.Fatal("Sources.Service is nil, expected non-nil")
	}
	if !cfg.Sources.Service.Enabled {
		t.Error("Sources.Service.Enabled is false, expected true")
	}
	if cfg.Sources.Service.AnnotationFilter != "external-dns.alpha.kubernetes.io/hostname" {
		t.Errorf("Sources.Service.AnnotationFilter = %q, expected %q",
			cfg.Sources.Service.AnnotationFilter, "external-dns.alpha.kubernetes.io/hostname")
	}
	if len(cfg.Sources.Service.ServiceTypeFilter) != 1 || cfg.Sources.Service.ServiceTypeFilter[0] != "LoadBalancer" {
		t.Errorf("Sources.Service.ServiceTypeFilter = %v, expected [LoadBalancer]",
			cfg.Sources.Service.ServiceTypeFilter)
	}

	// Verify ingress config
	if cfg.Sources.Ingress == nil {
		t.Fatal("Sources.Ingress is nil, expected non-nil")
	}
	if !cfg.Sources.Ingress.Enabled {
		t.Error("Sources.Ingress.Enabled is false, expected true")
	}
	if len(cfg.Sources.Ingress.IngressClassNames) != 1 || cfg.Sources.Ingress.IngressClassNames[0] != "nginx" {
		t.Errorf("Sources.Ingress.IngressClassNames = %v, expected [nginx]",
			cfg.Sources.Ingress.IngressClassNames)
	}

	// Verify group mapping
	if cfg.GroupMapping.DefaultGroup != "Discovered Services" {
		t.Errorf("GroupMapping.DefaultGroup = %q, expected %q",
			cfg.GroupMapping.DefaultGroup, "Discovered Services")
	}

	// Verify reconciliation
	if cfg.Reconciliation.Interval.Duration() != 30*time.Second {
		t.Errorf("Reconciliation.Interval = %v, expected 30s",
			cfg.Reconciliation.Interval.Duration())
	}
	if cfg.Reconciliation.RetryOnError.Duration() != 10*time.Second {
		t.Errorf("Reconciliation.RetryOnError = %v, expected 10s",
			cfg.Reconciliation.RetryOnError.Duration())
	}

	t.Logf("Config parsed successfully: %+v", cfg.LogSummary())
}

func TestLoadFromFile_NotExist(t *testing.T) {
	cfg, err := LoadFromFile("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("LoadFromFile failed for non-existent file: %v", err)
	}

	// Should return default config
	if cfg.Sources.Service != nil {
		t.Error("Expected Sources.Service to be nil for default config")
	}
	if cfg.GroupMapping.DefaultGroup != "Services" {
		t.Errorf("GroupMapping.DefaultGroup = %q, expected %q for default",
			cfg.GroupMapping.DefaultGroup, "Services")
	}
}

func TestLoadFromFile_WithSourcePriority(t *testing.T) {
	content := `
sources:
  service:
    enabled: true
  ingress:
    enabled: true
  priority:
    - service
    - ingress
    - dnsendpoint

groupMapping:
  defaultGroup: "Services"

reconciliation:
  interval: 5m
  retryOnError: 30s
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	wantPriority := []string{"service", "ingress", "dnsendpoint"}
	if len(cfg.Sources.Priority) != len(wantPriority) {
		t.Fatalf("Sources.Priority = %v, expected %v", cfg.Sources.Priority, wantPriority)
	}
	for i, v := range wantPriority {
		if cfg.Sources.Priority[i] != v {
			t.Errorf("Sources.Priority[%d] = %q, expected %q", i, cfg.Sources.Priority[i], v)
		}
	}
}

func TestLoadFromFile_ActualTestConfig(t *testing.T) {
	// Test with the actual test config file
	cfg, err := LoadFromFile("../../config/samples/test_config.yaml")
	if err != nil {
		t.Fatalf("LoadFromFile failed for test_config.yaml: %v", err)
	}

	t.Logf("Loaded test_config.yaml:")
	for k, v := range cfg.LogSummary() {
		t.Logf("  %s: %v", k, v)
	}

	if cfg.Sources.Service == nil {
		t.Fatal("Sources.Service is nil after loading test_config.yaml")
	}
	if !cfg.Sources.Service.Enabled {
		t.Error("Sources.Service.Enabled is false in test_config.yaml")
	}
}
