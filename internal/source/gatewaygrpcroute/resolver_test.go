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

package gatewaygrpcroute_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	ggr "github.com/golgoth31/sreportal/internal/source/gatewaygrpcroute"
)

const (
	tTarget5555  = "5.5.5.5"
	tFQDNA       = "a.example.com"
	tAnnotTarget = "external-dns.alpha.kubernetes.io/target"
)

func TestGRPCRouteResolver_Hostnames(t *testing.T) {
	r := ggr.NewResolver()
	rt := &gwapiv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{tAnnotTarget: tTarget5555},
		},
		Spec: gwapiv1.GRPCRouteSpec{Hostnames: []gwapiv1.Hostname{tFQDNA, "b.example.com"}},
	}
	eps, err := r.ResolveObject(context.Background(), rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("want 2, got %d", len(eps))
	}
}

func TestGRPCRouteResolver_NoTarget(t *testing.T) {
	r := ggr.NewResolver()
	rt := &gwapiv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{},
		},
		Spec: gwapiv1.GRPCRouteSpec{Hostnames: []gwapiv1.Hostname{tFQDNA}},
	}
	eps, _ := r.ResolveObject(context.Background(), rt)
	if eps != nil {
		t.Fatalf("expected nil, got %+v", eps)
	}
}

func TestGRPCRouteResolver_NoHostnames(t *testing.T) {
	r := ggr.NewResolver()
	rt := &gwapiv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{tAnnotTarget: tTarget5555},
		},
		Spec: gwapiv1.GRPCRouteSpec{Hostnames: []gwapiv1.Hostname{}},
	}
	eps, _ := r.ResolveObject(context.Background(), rt)
	if eps != nil {
		t.Fatalf("expected nil, got %+v", eps)
	}
}

func TestGRPCRouteResolver_TrimsTrailingDot(t *testing.T) {
	r := ggr.NewResolver()
	rt := &gwapiv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt", Namespace: "ns",
			Annotations: map[string]string{tAnnotTarget: tTarget5555},
		},
		Spec: gwapiv1.GRPCRouteSpec{Hostnames: []gwapiv1.Hostname{"a.example.com."}},
	}
	eps, _ := r.ResolveObject(context.Background(), rt)
	if len(eps) != 1 || eps[0].DNSName != tFQDNA {
		t.Fatalf("unexpected: %+v", eps)
	}
}
