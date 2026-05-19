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

package service_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	svcsrc "github.com/golgoth31/sreportal/internal/source/service"
)

const (
	tNSDefault     = "default"
	tAnnotHostname = "external-dns.alpha.kubernetes.io/hostname"
	tIP1234        = "1.2.3.4"
)

func TestServiceResolver_ResolveObject_Hostname(t *testing.T) {
	r := svcsrc.NewResolver()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "echo", Namespace: tNSDefault,
			Annotations: map[string]string{tAnnotHostname: "echo.example.com"},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: tIP1234}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "echo.example.com" || eps[0].Targets[0] != tIP1234 {
		t.Fatalf("unexpected endpoints: %+v", eps)
	}
}

func TestServiceResolver_ResolveObject_NoHostname(t *testing.T) {
	r := svcsrc.NewResolver()
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"}}
	eps, _ := r.ResolveObject(context.Background(), svc)
	if len(eps) != 0 {
		t.Fatalf("want no endpoints, got %d", len(eps))
	}
}

func TestServiceResolver_ResolveObject_NoLBIngress(t *testing.T) {
	r := svcsrc.NewResolver()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "x", Namespace: "y",
			Annotations: map[string]string{tAnnotHostname: "x.example.com"},
		},
	}
	eps, _ := r.ResolveObject(context.Background(), svc)
	if len(eps) != 0 {
		t.Fatalf("want no endpoints, got %d", len(eps))
	}
}

// TestServiceResolver_MultipleIPs verifies that when a Service has multiple
// LoadBalancer ingress IPs, all are collected into a single endpoint's Targets
// (scenario 5: multi-IP / multi-target).
func TestServiceResolver_MultipleIPs(t *testing.T) {
	r := svcsrc.NewResolver()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "multi", Namespace: tNSDefault,
			Annotations: map[string]string{tAnnotHostname: "multi.example.com"},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{
				{IP: tIP1234},
				{IP: "5.6.7.8"},
				{IP: "9.10.11.12"},
			},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 {
		t.Fatalf("want 1 endpoint, got %d", len(eps))
	}
	if len(eps[0].Targets) != 3 {
		t.Fatalf("want 3 targets, got %d: %v", len(eps[0].Targets), eps[0].Targets)
	}
}

// TestServiceResolver_TrailingDotHostname verifies that a hostname annotation
// with a trailing dot has the dot stripped before emitting (scenario 3).
func TestServiceResolver_TrailingDotHostname(t *testing.T) {
	r := svcsrc.NewResolver()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dot", Namespace: tNSDefault,
			Annotations: map[string]string{tAnnotHostname: "dot.example.com."},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: tIP1234}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "dot.example.com" {
		t.Fatalf("expected trailing dot stripped, got DNSName=%q", eps[0].DNSName)
	}
}

// TestServiceResolver_LBHostname_EmittedAsCNAME verifies that when a Service
// LoadBalancer has a hostname target (instead of an IP), the resolver emits a
// CNAME endpoint pointing at that hostname (scenario 2).
func TestServiceResolver_LBHostname_EmittedAsCNAME(t *testing.T) {
	r := svcsrc.NewResolver()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nlb", Namespace: tNSDefault,
			Annotations: map[string]string{tAnnotHostname: "api.example.com"},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{Hostname: "nlb.aws.example.com"}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 {
		t.Fatalf("want 1 CNAME endpoint, got %d", len(eps))
	}
	if eps[0].RecordType != "CNAME" {
		t.Errorf("want CNAME, got %s", eps[0].RecordType)
	}
	if eps[0].Targets[0] != "nlb.aws.example.com" {
		t.Errorf("want target nlb.aws.example.com, got %v", eps[0].Targets)
	}
}
