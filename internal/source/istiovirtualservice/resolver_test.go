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

package istiovirtualservice_test

import (
	"context"
	"testing"

	istionetworking "istio.io/api/networking/v1"
	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ivs "github.com/golgoth31/sreportal/internal/source/istiovirtualservice"
)

func TestIstioVirtualServiceResolver_Hosts(t *testing.T) {
	r := ivs.NewResolver()
	vs := &istionetworkingv1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vs", Namespace: "x",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/target": "9.9.9.9"},
		},
		Spec: istionetworking.VirtualService{Hosts: []string{"a.example.com", "*"}},
	}
	eps, err := r.ResolveObject(context.Background(), vs)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "a.example.com" {
		t.Fatalf("unexpected: %+v", eps)
	}
}
