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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golgoth31/sreportal/internal/source/gatewaygrpcroute"
	"github.com/golgoth31/sreportal/internal/source/gatewayhttproute"
	"github.com/golgoth31/sreportal/internal/source/gatewaytcproute"
	"github.com/golgoth31/sreportal/internal/source/gatewaytlsroute"
	"github.com/golgoth31/sreportal/internal/source/gatewayudproute"
	"github.com/golgoth31/sreportal/internal/source/istiogateway"
	"github.com/golgoth31/sreportal/internal/source/istiovirtualservice"
)

// TestSchemeRegistersExternalSourceTypes guards against the SourceReconciler
// failing client-side ("no kind is registered for the type ...") when a source
// backed by an external API (Istio, Gateway API) is enabled on a DNS CR. Each
// resolver's list type must be recognized by the manager scheme built in init().
func TestSchemeRegistersExternalSourceTypes(t *testing.T) {
	lists := map[string]client.ObjectList{
		"istio-gateway":        istiogateway.NewResolver().ObjectList(),
		"istio-virtualservice": istiovirtualservice.NewResolver().ObjectList(),
		"gateway-httproute":    gatewayhttproute.NewResolver().ObjectList(),
		"gateway-grpcroute":    gatewaygrpcroute.NewResolver().ObjectList(),
		"gateway-tcproute":     gatewaytcproute.NewResolver().ObjectList(),
		"gateway-tlsroute":     gatewaytlsroute.NewResolver().ObjectList(),
		"gateway-udproute":     gatewayudproute.NewResolver().ObjectList(),
	}
	for name, list := range lists {
		if _, _, err := scheme.ObjectKinds(list); err != nil {
			t.Errorf("scheme does not recognize %s list type %T: %v", name, list, err)
		}
	}
}
