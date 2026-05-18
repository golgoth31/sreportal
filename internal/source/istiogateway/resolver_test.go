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

package istiogateway_test

import (
	"context"
	"testing"

	istionetworking "istio.io/api/networking/v1"
	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	igw "github.com/golgoth31/sreportal/internal/source/istiogateway"
)

const (
	tNSIstio     = "istio-system"
	tAnnotTarget = "external-dns.alpha.kubernetes.io/target"
	tTarget5555  = "5.5.5.5"
)

func TestIstioGatewayResolver_HostsFromServers(t *testing.T) {
	r := igw.NewResolver()
	gw := &istionetworkingv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: "edge", Namespace: tNSIstio,
			Annotations: map[string]string{tAnnotTarget: "1.2.3.4"},
		},
		Spec: istionetworking.Gateway{Servers: []*istionetworking.Server{
			{Hosts: []string{"namespace/foo.example.com", "*", "bar.example.com"}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), gw)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("want 2 endpoints, got %d", len(eps))
	}
}

// TestIstioGatewayResolver_MultiServerMultiHost verifies that hosts across
// multiple servers are all emitted (scenario 1: multi-host inputs).
func TestIstioGatewayResolver_MultiServerMultiHost(t *testing.T) {
	r := igw.NewResolver()
	gw := &istionetworkingv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gw", Namespace: tNSIstio,
			Annotations: map[string]string{tAnnotTarget: tTarget5555},
		},
		Spec: istionetworking.Gateway{Servers: []*istionetworking.Server{
			{Hosts: []string{"ns/a.example.com", "ns/b.example.com"}},
			{Hosts: []string{"ns/c.example.com"}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), gw)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 3 {
		t.Fatalf("want 3 endpoints, got %d", len(eps))
	}
}

// TestIstioGatewayResolver_WildcardSkipped verifies that bare "*" and
// "namespace/*" hosts are dropped (scenario 4: wildcard hostnames).
func TestIstioGatewayResolver_WildcardSkipped(t *testing.T) {
	r := igw.NewResolver()
	gw := &istionetworkingv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gw", Namespace: tNSIstio,
			Annotations: map[string]string{tAnnotTarget: tTarget5555},
		},
		Spec: istionetworking.Gateway{Servers: []*istionetworking.Server{
			{Hosts: []string{"ns/*", "*"}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), gw)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 0 {
		t.Fatalf("want 0 endpoints (wildcards skipped), got %d: %+v", len(eps), eps)
	}
}

// TestIstioGatewayResolver_TrailingDot verifies that trailing dots in host
// entries are stripped (scenario 3).
func TestIstioGatewayResolver_TrailingDot(t *testing.T) {
	r := igw.NewResolver()
	gw := &istionetworkingv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gw", Namespace: tNSIstio,
			Annotations: map[string]string{tAnnotTarget: tTarget5555},
		},
		Spec: istionetworking.Gateway{Servers: []*istionetworking.Server{
			{Hosts: []string{"ns/dot.example.com."}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), gw)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "dot.example.com" {
		t.Fatalf("expected trailing dot stripped, got %+v", eps)
	}
}
