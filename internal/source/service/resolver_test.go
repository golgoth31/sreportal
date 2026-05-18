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

func TestServiceResolver_ResolveObject_Hostname(t *testing.T) {
	r := svcsrc.NewResolver()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "echo", Namespace: "default",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/hostname": "echo.example.com"},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: "1.2.3.4"}},
		}},
	}
	eps, err := r.ResolveObject(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "echo.example.com" || eps[0].Targets[0] != "1.2.3.4" {
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
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/hostname": "x.example.com"},
		},
	}
	eps, _ := r.ResolveObject(context.Background(), svc)
	if len(eps) != 0 {
		t.Fatalf("want no endpoints, got %d", len(eps))
	}
}
