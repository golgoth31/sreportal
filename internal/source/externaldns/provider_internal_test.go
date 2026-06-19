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

package externaldns

import (
	"context"
	"errors"
	"testing"
	"time"

	kubefake "k8s.io/client-go/kubernetes/fake"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// TestEndpoints_BoundedWaitReturnsNotReady verifies that a build which has not
// finished within the bounded wait surfaces ErrSourceNotReady instead of
// blocking — the core of the per-kind isolation (a source that cannot sync must
// never hang the single-goroutine SourceReconciler).
func TestEndpoints_BoundedWaitReturnsNotReady(t *testing.T) {
	p := NewProvider(kubefake.NewSimpleClientset(), nil, nil)
	p.buildWait = time.Nanosecond // force the timeout branch before the build can finish

	cfgs := BuildEffectiveConfigs([]sreportalv1alpha2.DNS{{Spec: sreportalv1alpha2.DNSSpec{
		Sources: sreportalv1alpha2.SourcesSpec{
			Service: &sreportalv1alpha2.ServiceSourceSpec{
				CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
			},
		},
	}}})

	_, err := p.Endpoints(context.Background(), KindService, cfgs[KindService])
	if !errors.Is(err, ErrSourceNotReady) {
		t.Fatalf("expected ErrSourceNotReady within bounded wait, got %v", err)
	}
}

// TestToConfig_DNSEndpoint verifies the DNSEndpoint kind builds an effective
// config (the CRD type is hardwired by external-dns' NewCRDSource, so toConfig
// only needs to pass through namespace/labelFilter without error).
func TestToConfig_DNSEndpoint(t *testing.T) {
	cfgs := BuildEffectiveConfigs([]sreportalv1alpha2.DNS{{Spec: sreportalv1alpha2.DNSSpec{
		Sources: sreportalv1alpha2.SourcesSpec{
			DNSEndpoint: &sreportalv1alpha2.DNSEndpointSourceSpec{Enabled: true, Namespace: "team-a", LabelFilter: "team=a"},
		},
	}}})
	if cfgs[KindDNSEndpoint] == nil {
		t.Fatal("DNSEndpoint must yield an effective config when enabled")
	}
	cfg, err := cfgs[KindDNSEndpoint].toConfig(KindDNSEndpoint)
	if err != nil {
		t.Fatalf("toConfig: %v", err)
	}
	if cfg.Namespace != "team-a" {
		t.Fatalf("expected namespace passthrough, got %q", cfg.Namespace)
	}
	if cfg.LabelFilter.String() != "team=a" {
		t.Fatalf("expected labelFilter passthrough, got %q", cfg.LabelFilter.String())
	}
}

// TestToConfig_FQDNTemplate verifies a configured fqdnTemplate is captured
// (toConfig succeeds and the config hash differs from the no-template case, so
// the source is rebuilt when the template changes). template.Engine is a struct
// value, so it can't be compared to nil directly.
func TestToConfig_FQDNTemplate(t *testing.T) {
	withTmpl := BuildEffectiveConfigs([]sreportalv1alpha2.DNS{{Spec: sreportalv1alpha2.DNSSpec{
		Sources: sreportalv1alpha2.SourcesSpec{
			Ingress: &sreportalv1alpha2.IngressSourceSpec{CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{
				Enabled:      true,
				FQDNTemplate: "{{.Name}}.example.com",
			}},
		},
	}}})
	if _, err := withTmpl[KindIngress].toConfig(KindIngress); err != nil {
		t.Fatalf("toConfig with template: %v", err)
	}

	noTmpl := BuildEffectiveConfigs([]sreportalv1alpha2.DNS{{Spec: sreportalv1alpha2.DNSSpec{
		Sources: sreportalv1alpha2.SourcesSpec{
			Ingress: &sreportalv1alpha2.IngressSourceSpec{CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true}},
		},
	}}})

	if withTmpl[KindIngress].hash(KindIngress) == noTmpl[KindIngress].hash(KindIngress) {
		t.Fatal("config hash must differ when fqdnTemplate is set vs unset")
	}
}

// TestHandles_AllNativeKinds guards that every kind the Provider can build is
// reported as natively handled (so Cycle dispatches it to the Provider).
func TestHandles_AllNativeKinds(t *testing.T) {
	for _, k := range []registry.SourceType{
		KindService, KindIngress, KindIstioGateway, KindIstioVirtualService,
		KindGatewayHTTPRoute, KindGatewayGRPCRoute, KindGatewayTCPRoute,
		KindGatewayTLSRoute, KindGatewayUDPRoute, KindDNSEndpoint,
	} {
		if !Handles(k) {
			t.Errorf("Handles(%q) = false, want true", k)
		}
	}
}
