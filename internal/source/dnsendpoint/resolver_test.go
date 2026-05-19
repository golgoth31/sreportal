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

package dnsendpoint_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"

	desrc "github.com/golgoth31/sreportal/internal/source/dnsendpoint"
)

func TestDNSEndpointResolver_Passthrough(t *testing.T) {
	r := desrc.NewResolver()
	de := &v1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"},
		Spec: v1alpha1.DNSEndpointSpec{Endpoints: []*endpoint.Endpoint{
			endpoint.NewEndpoint("a.example.com", endpoint.RecordTypeA, "1.2.3.4"),
		}},
	}
	eps, err := r.ResolveObject(context.Background(), de)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "a.example.com" {
		t.Fatalf("unexpected: %+v", eps)
	}
}

// TestDNSEndpointResolver_MultiEndpoint verifies that all endpoints in
// Spec.Endpoints are passed through (scenario 1: multi-host inputs via
// multiple Endpoint entries).
func TestDNSEndpointResolver_MultiEndpoint(t *testing.T) {
	r := desrc.NewResolver()
	de := &v1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"},
		Spec: v1alpha1.DNSEndpointSpec{Endpoints: []*endpoint.Endpoint{
			endpoint.NewEndpoint("a.example.com", endpoint.RecordTypeA, "1.1.1.1"),
			endpoint.NewEndpoint("b.example.com", endpoint.RecordTypeA, "2.2.2.2"),
			endpoint.NewEndpoint("c.example.com", endpoint.RecordTypeCNAME, "lb.example.com"),
		}},
	}
	eps, err := r.ResolveObject(context.Background(), de)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 3 {
		t.Fatalf("want 3 endpoints, got %d: %+v", len(eps), eps)
	}
}

// TestDNSEndpointResolver_EmptyEndpoints verifies that a DNSEndpoint with no
// endpoints in its spec returns nil cleanly.
func TestDNSEndpointResolver_EmptyEndpoints(t *testing.T) {
	r := desrc.NewResolver()
	de := &v1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"},
		Spec:       v1alpha1.DNSEndpointSpec{Endpoints: nil},
	}
	eps, err := r.ResolveObject(context.Background(), de)
	if err != nil {
		t.Fatal(err)
	}
	if eps != nil {
		t.Fatalf("want nil, got %+v", eps)
	}
}

// TestDNSEndpointResolver_NilEntryFiltered verifies that nil endpoint entries
// in Spec.Endpoints are silently skipped.
func TestDNSEndpointResolver_NilEntryFiltered(t *testing.T) {
	r := desrc.NewResolver()
	de := &v1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"},
		Spec: v1alpha1.DNSEndpointSpec{Endpoints: []*endpoint.Endpoint{
			nil,
			endpoint.NewEndpoint("a.example.com", endpoint.RecordTypeA, "1.2.3.4"),
		}},
	}
	eps, err := r.ResolveObject(context.Background(), de)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "a.example.com" {
		t.Fatalf("unexpected: %+v", eps)
	}
}
