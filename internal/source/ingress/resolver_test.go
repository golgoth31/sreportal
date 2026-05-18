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

func TestIngressResolver_Hostname(t *testing.T) {
	r := ingsrc.NewResolver()
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: "default",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/hostname": "web.example.com"},
		},
		Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{
			Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "10.0.0.1"}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), ing)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "web.example.com" || eps[0].Targets[0] != "10.0.0.1" {
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
