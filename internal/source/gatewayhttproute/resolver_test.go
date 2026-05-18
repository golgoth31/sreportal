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

package gatewayhttproute_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	ghr "github.com/golgoth31/sreportal/internal/source/gatewayhttproute"
)

const (
	tTarget5555  = "5.5.5.5"
	tAnnotTarget = "external-dns.alpha.kubernetes.io/target"
)

func TestHTTPRouteResolver_Hostnames(t *testing.T) {
	r := ghr.NewResolver()
	rt := &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{tAnnotTarget: tTarget5555},
		},
		Spec: gwapiv1.HTTPRouteSpec{Hostnames: []gwapiv1.Hostname{"a.example.com", "b.example.com"}},
	}
	eps, err := r.ResolveObject(context.Background(), rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("want 2, got %d", len(eps))
	}
}

// TestHTTPRouteResolver_TrailingDot verifies that trailing dots in Spec.Hostnames
// are stripped (scenario 3).
func TestHTTPRouteResolver_TrailingDot(t *testing.T) {
	r := ghr.NewResolver()
	rt := &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{tAnnotTarget: tTarget5555},
		},
		Spec: gwapiv1.HTTPRouteSpec{Hostnames: []gwapiv1.Hostname{"api.example.com."}},
	}
	eps, err := r.ResolveObject(context.Background(), rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "api.example.com" {
		t.Fatalf("expected trailing dot stripped, got %+v", eps)
	}
}

// TestHTTPRouteResolver_MultiHost verifies all hostnames emit separate
// endpoints (scenario 1: multi-host inputs).
func TestHTTPRouteResolver_MultiHost(t *testing.T) {
	r := ghr.NewResolver()
	rt := &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{tAnnotTarget: tTarget5555},
		},
		Spec: gwapiv1.HTTPRouteSpec{Hostnames: []gwapiv1.Hostname{
			"a.example.com", "b.example.com", "c.example.com",
		}},
	}
	eps, err := r.ResolveObject(context.Background(), rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 3 {
		t.Fatalf("want 3 endpoints, got %d", len(eps))
	}
}

// TestHTTPRouteResolver_WildcardSubdomain verifies that "*.example.com"
// hostnames are passed through as-is (scenario 4): the HTTPRoute resolver
// does not filter sub-wildcard entries, only empty strings.
func TestHTTPRouteResolver_WildcardSubdomain(t *testing.T) {
	r := ghr.NewResolver()
	rt := &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{tAnnotTarget: tTarget5555},
		},
		Spec: gwapiv1.HTTPRouteSpec{Hostnames: []gwapiv1.Hostname{"*.example.com"}},
	}
	eps, err := r.ResolveObject(context.Background(), rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "*.example.com" {
		t.Fatalf("expected wildcard subdomain to be emitted as-is, got %+v", eps)
	}
}
