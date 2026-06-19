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

package externaldns_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/source/externaldns"
)

func dnsEnabling(set func(*sreportalv1alpha2.SourcesSpec)) []sreportalv1alpha2.DNS {
	d := sreportalv1alpha2.DNS{}
	set(&d.Spec.Sources)
	return []sreportalv1alpha2.DNS{d}
}

// TestProvider_IngressFromRules proves native external-dns extraction: an Ingress
// with spec.rules[].host and NO external-dns hostname annotation yields an
// endpoint — exactly the case the hand-rolled resolver dropped (194→5).
func TestProvider_IngressFromRules(t *testing.T) {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{Host: "app.example.com"}},
		},
		Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{
			Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "1.2.3.4"}},
		}},
	}
	kube := kubefake.NewSimpleClientset(ing)
	p := externaldns.NewProvider(kube, nil, nil)
	cfgs := externaldns.BuildEffectiveConfigs(dnsEnabling(func(s *sreportalv1alpha2.SourcesSpec) {
		s.Ingress = &sreportalv1alpha2.IngressSourceSpec{CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true}}
	}))

	eps, err := p.Endpoints(context.Background(), externaldns.KindIngress, cfgs[externaldns.KindIngress])
	if err != nil {
		t.Fatalf("Endpoints: %v", err)
	}
	found := false
	for _, e := range eps {
		if e.DNSName == "app.example.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected app.example.com from ingress rules, got %d endpoints: %+v", len(eps), eps)
	}
}

// TestProvider_ServiceAnnotated covers a LoadBalancer service with the external-dns
// hostname annotation.
func TestProvider_ServiceAnnotated(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "echo", Namespace: "default",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/hostname": "echo.example.com"},
		},
		Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: "5.6.7.8"}},
		}},
	}
	kube := kubefake.NewSimpleClientset(svc)
	p := externaldns.NewProvider(kube, nil, nil)
	cfgs := externaldns.BuildEffectiveConfigs(dnsEnabling(func(s *sreportalv1alpha2.SourcesSpec) {
		s.Service = &sreportalv1alpha2.ServiceSourceSpec{
			CommonSourceSpec:  sreportalv1alpha2.CommonSourceSpec{Enabled: true},
			ServiceTypeFilter: []string{"LoadBalancer"},
		}
	}))

	eps, err := p.Endpoints(context.Background(), externaldns.KindService, cfgs[externaldns.KindService])
	if err != nil {
		t.Fatalf("Endpoints: %v", err)
	}
	found := false
	for _, e := range eps {
		if e.DNSName == "echo.example.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected echo.example.com from service, got %d endpoints: %+v", len(eps), eps)
	}
}

// TestProvider_IstioGatewayWithoutIstioClient verifies that requesting the
// istio-gateway kind without an istio client surfaces an error (so the cycle
// preserves state) rather than panicking or silently returning nothing.
func TestProvider_IstioGatewayWithoutIstioClient(t *testing.T) {
	p := externaldns.NewProvider(kubefake.NewSimpleClientset(), nil, nil) // istio == nil
	cfgs := externaldns.BuildEffectiveConfigs(dnsEnabling(func(s *sreportalv1alpha2.SourcesSpec) {
		s.IstioGateway = &sreportalv1alpha2.IstioGatewaySourceSpec{
			CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
		}
	}))

	_, err := p.Endpoints(context.Background(), externaldns.KindIstioGateway, cfgs[externaldns.KindIstioGateway])
	if err == nil {
		t.Fatal("expected an error when istio client is not configured, got nil")
	}
}
