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

package dns_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnschain "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/ingress"
	"github.com/golgoth31/sreportal/internal/source/registry"
	"github.com/golgoth31/sreportal/internal/source/service"
)

func TestIntraDNSDedup_FirstKindWins(t *testing.T) {
	h := &dnschain.IntraDNSDedupHandler{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Data: dnschain.ChainData{
			PriorityOrder: []registry.SourceType{service.SourceTypeService, ingress.SourceTypeIngress},
			EndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				service.SourceTypeService: {endpoint.NewEndpoint("a.example.com", "A", "1.1.1.1")},
				ingress.SourceTypeIngress: {
					endpoint.NewEndpoint("a.example.com", "A", "2.2.2.2"),
					endpoint.NewEndpoint("b.example.com", "A", "3.3.3.3"),
				},
			},
		},
	}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Len(t, rc.Data.KeptEndpointsByKind[service.SourceTypeService], 1)
	require.Equal(t, "a.example.com", rc.Data.KeptEndpointsByKind[service.SourceTypeService][0].DNSName)
	require.Len(t, rc.Data.KeptEndpointsByKind[ingress.SourceTypeIngress], 1)
	require.Equal(t, "b.example.com", rc.Data.KeptEndpointsByKind[ingress.SourceTypeIngress][0].DNSName)
}
