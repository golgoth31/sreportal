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

func TestIstioGatewayResolver_HostsFromServers(t *testing.T) {
	r := igw.NewResolver()
	gw := &istionetworkingv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: "edge", Namespace: "istio-system",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/target": "1.2.3.4"},
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
