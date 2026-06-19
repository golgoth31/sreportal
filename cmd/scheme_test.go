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

package main

import (
	"testing"

	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// TestSchemeRegistersExternalSourceTypes guards against the source enrichment
// re-fetch failing client-side ("no kind is registered for the type ...") when
// a source backed by an external API (Istio, Gateway API) is enabled on a DNS
// CR. Each list type the native external-dns sources rely on must be recognized
// by the manager scheme built in init().
func TestSchemeRegistersExternalSourceTypes(t *testing.T) {
	lists := map[string]client.ObjectList{
		"istio-gateway":        &istionetworkingv1.GatewayList{},
		"istio-virtualservice": &istionetworkingv1.VirtualServiceList{},
		"gateway-httproute":    &gwapiv1.HTTPRouteList{},
		"gateway-grpcroute":    &gwapiv1.GRPCRouteList{},
		"gateway-tcproute":     &gwapiv1alpha2.TCPRouteList{},
		"gateway-tlsroute":     &gwapiv1alpha2.TLSRouteList{},
		"gateway-udproute":     &gwapiv1alpha2.UDPRouteList{},
	}
	for name, list := range lists {
		if _, _, err := scheme.ObjectKinds(list); err != nil {
			t.Errorf("scheme does not recognize %s list type %T: %v", name, list, err)
		}
	}
}
