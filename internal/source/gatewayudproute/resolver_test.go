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

package gatewayudproute_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	gur "github.com/golgoth31/sreportal/internal/source/gatewayudproute"
)

const (
	tAnnotHostname = "external-dns.alpha.kubernetes.io/hostname"
	tAnnotTarget   = "external-dns.alpha.kubernetes.io/target"
	tFQDNA         = "a.example.com"
	tTarget5555    = "5.5.5.5"
)

func TestUDPRouteResolver_HostnameFromAnnotation(t *testing.T) {
	r := gur.NewResolver()
	rt := &gwapiv1alpha2.UDPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{
				tAnnotTarget:   tTarget5555,
				tAnnotHostname: "a.example.com.",
			},
		},
	}
	eps, err := r.ResolveObject(context.Background(), rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != tFQDNA {
		t.Fatalf("unexpected: %+v", eps)
	}
}

func TestUDPRouteResolver_NoTarget(t *testing.T) {
	r := gur.NewResolver()
	rt := &gwapiv1alpha2.UDPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{tAnnotHostname: tFQDNA},
		},
	}
	eps, _ := r.ResolveObject(context.Background(), rt)
	if eps != nil {
		t.Fatalf("expected nil, got %+v", eps)
	}
}

func TestUDPRouteResolver_NoHostname(t *testing.T) {
	r := gur.NewResolver()
	rt := &gwapiv1alpha2.UDPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{tAnnotTarget: tTarget5555},
		},
	}
	eps, _ := r.ResolveObject(context.Background(), rt)
	if eps != nil {
		t.Fatalf("expected nil, got %+v", eps)
	}
}

func TestUDPRouteResolver_TrimsTrailingDot(t *testing.T) {
	r := gur.NewResolver()
	rt := &gwapiv1alpha2.UDPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{
				tAnnotTarget:   tTarget5555,
				tAnnotHostname: "a.example.com.",
			},
		},
	}
	eps, _ := r.ResolveObject(context.Background(), rt)
	if len(eps) != 1 || eps[0].DNSName != tFQDNA {
		t.Fatalf("unexpected: %+v", eps)
	}
}
