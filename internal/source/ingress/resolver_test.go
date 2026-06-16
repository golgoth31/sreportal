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

package ingress_test

import (
	"context"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ingsrc "github.com/golgoth31/sreportal/internal/source/ingress"
)

const (
	tAnnotHostname = "external-dns.alpha.kubernetes.io/hostname"
	tNSDefault     = "default"
	tIP10001       = "10.0.0.1"
)

func TestIngressResolver_Hostname(t *testing.T) {
	r := ingsrc.NewResolver()
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: tNSDefault,
			Annotations: map[string]string{tAnnotHostname: "web.example.com"},
		},
		Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{
			Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: tIP10001}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), ing)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "web.example.com" || eps[0].Targets[0] != tIP10001 {
		t.Fatalf("unexpected: %+v", eps)
	}
}

func TestIngressResolver_NoHostname(t *testing.T) {
	r := ingsrc.NewResolver()
	ing := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"}}
	eps, _ := r.ResolveObject(context.Background(), ing)
	if len(eps) != 0 {
		t.Fatalf("want 0, got %d", len(eps))
	}
}

// TestIngressResolver_TrailingDotHostname verifies that a trailing dot in the
// hostname annotation is stripped (scenario 3).
func TestIngressResolver_TrailingDotHostname(t *testing.T) {
	r := ingsrc.NewResolver()
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dot", Namespace: tNSDefault,
			Annotations: map[string]string{tAnnotHostname: "dot.example.com."},
		},
		Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{
			Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: tIP10001}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), ing)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "dot.example.com" {
		t.Fatalf("expected trailing dot stripped, got %+v", eps)
	}
}

// TestIngressResolver_MultipleIPs verifies that multiple LoadBalancer IPs are
// all emitted as targets on a single endpoint (scenario 5).
func TestIngressResolver_MultipleIPs(t *testing.T) {
	r := ingsrc.NewResolver()
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "multi", Namespace: tNSDefault,
			Annotations: map[string]string{tAnnotHostname: "multi.example.com"},
		},
		Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{
			Ingress: []networkingv1.IngressLoadBalancerIngress{
				{IP: tIP10001},
				{IP: "10.0.0.2"},
			},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), ing)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 {
		t.Fatalf("want 1 endpoint, got %d", len(eps))
	}
	if len(eps[0].Targets) != 2 {
		t.Fatalf("want 2 targets, got %d: %v", len(eps[0].Targets), eps[0].Targets)
	}
}
