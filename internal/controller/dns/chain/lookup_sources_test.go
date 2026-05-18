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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnschain "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	rsource "github.com/golgoth31/sreportal/internal/readstore/source"
	"github.com/golgoth31/sreportal/internal/reconciler"
	gwhttp "github.com/golgoth31/sreportal/internal/source/gatewayhttproute"
	"github.com/golgoth31/sreportal/internal/source/ingress"
	"github.com/golgoth31/sreportal/internal/source/registry"
	"github.com/golgoth31/sreportal/internal/source/service"
)

func TestLookupSourcesHandler_FiltersByNamespaceAndLabel(t *testing.T) {
	store := rsource.NewStore()
	store.ReplaceKind(service.SourceTypeService, []domainsource.EnrichedEndpoint{
		{Endpoint: endpoint.NewEndpoint("a.example.com", "A", "1.1.1.1"), Kind: service.SourceTypeService, Namespace: "ns1", SourceLabels: map[string]string{"team": "a"}},
		{Endpoint: endpoint.NewEndpoint("b.example.com", "A", "2.2.2.2"), Kind: service.SourceTypeService, Namespace: "ns1", SourceLabels: map[string]string{"team": "b"}},
		{Endpoint: endpoint.NewEndpoint("c.example.com", "A", "3.3.3.3"), Kind: service.SourceTypeService, Namespace: "ns2"},
	})

	h := &dnschain.LookupSourcesHandler{Source: store}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: &sreportalv1alpha2.DNS{
			ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "ns1"},
			Spec: sreportalv1alpha2.DNSSpec{
				Defaults: sreportalv1alpha2.SourceFilterDefaults{Namespace: "ns1"},
				Sources: sreportalv1alpha2.SourcesSpec{
					Service: &sreportalv1alpha2.ServiceSourceSpec{
						CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true, LabelFilter: "team=a"},
					},
				},
			},
		},
		Data: dnschain.ChainData{},
	}
	require.NoError(t, h.Handle(context.Background(), rc))
	got := rc.Data.EndpointsByKind[service.SourceTypeService]
	require.Len(t, got, 1)
	require.Equal(t, "a.example.com", got[0].DNSName)
}

func TestLookupSourcesHandler_PerKindOverridesDefaults(t *testing.T) {
	store := rsource.NewStore()
	store.ReplaceKind(ingress.SourceTypeIngress, []domainsource.EnrichedEndpoint{
		{Endpoint: endpoint.NewEndpoint("ing.example.com", "A", "9.9.9.9"), Kind: ingress.SourceTypeIngress, Namespace: "perKind"},
		{Endpoint: endpoint.NewEndpoint("ing-default.example.com", "A", "9.9.9.9"), Kind: ingress.SourceTypeIngress, Namespace: "defaultNS"},
	})

	h := &dnschain.LookupSourcesHandler{Source: store}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: &sreportalv1alpha2.DNS{
			ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "x"},
			Spec: sreportalv1alpha2.DNSSpec{
				Defaults: sreportalv1alpha2.SourceFilterDefaults{Namespace: "defaultNS"},
				Sources: sreportalv1alpha2.SourcesSpec{
					Ingress: &sreportalv1alpha2.IngressSourceSpec{
						CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true, Namespace: "perKind"},
					},
				},
			},
		},
		Data: dnschain.ChainData{},
	}
	require.NoError(t, h.Handle(context.Background(), rc))
	got := rc.Data.EndpointsByKind[ingress.SourceTypeIngress]
	require.Len(t, got, 1)
	require.Equal(t, "ing.example.com", got[0].DNSName)
}

func TestLookupSourcesHandler_PriorityOrder(t *testing.T) {
	store := rsource.NewStore()
	store.ReplaceKind(service.SourceTypeService, []domainsource.EnrichedEndpoint{
		{Endpoint: endpoint.NewEndpoint("a.example.com", "A", "1.1.1.1"), Kind: service.SourceTypeService, Namespace: "ns"},
	})
	store.ReplaceKind(ingress.SourceTypeIngress, []domainsource.EnrichedEndpoint{
		{Endpoint: endpoint.NewEndpoint("b.example.com", "A", "2.2.2.2"), Kind: ingress.SourceTypeIngress, Namespace: "ns"},
	})
	store.ReplaceKind(gwhttp.SourceTypeGatewayHTTPRoute, []domainsource.EnrichedEndpoint{
		{Endpoint: endpoint.NewEndpoint("c.example.com", "A", "3.3.3.3"), Kind: gwhttp.SourceTypeGatewayHTTPRoute, Namespace: "ns"},
	})

	h := &dnschain.LookupSourcesHandler{Source: store}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: &sreportalv1alpha2.DNS{
			ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "x"},
			Spec: sreportalv1alpha2.DNSSpec{
				Defaults: sreportalv1alpha2.SourceFilterDefaults{Namespace: "ns"},
				Sources: sreportalv1alpha2.SourcesSpec{
					Service: &sreportalv1alpha2.ServiceSourceSpec{CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true}},
					Ingress: &sreportalv1alpha2.IngressSourceSpec{CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true}},
					GatewayHTTPRoute: &sreportalv1alpha2.GatewayRouteSourceSpec{
						CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
					},
					Priority: []sreportalv1alpha2.SourceType{
						sreportalv1alpha2.SourceTypeGatewayHTTPRoute,
						sreportalv1alpha2.SourceTypeService,
					},
				},
			},
		},
		Data: dnschain.ChainData{},
	}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Equal(t,
		[]registry.SourceType{
			gwhttp.SourceTypeGatewayHTTPRoute,
			service.SourceTypeService,
			ingress.SourceTypeIngress,
		},
		rc.Data.PriorityOrder,
	)
}

func TestLookupSourcesHandler_InvalidLabelSelectorReturnsError(t *testing.T) {
	store := rsource.NewStore()
	store.ReplaceKind(service.SourceTypeService, []domainsource.EnrichedEndpoint{
		{Endpoint: endpoint.NewEndpoint("a.example.com", "A", "1.1.1.1"), Kind: service.SourceTypeService, Namespace: "ns"},
	})

	h := &dnschain.LookupSourcesHandler{Source: store}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: &sreportalv1alpha2.DNS{
			ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "x"},
			Spec: sreportalv1alpha2.DNSSpec{
				Defaults: sreportalv1alpha2.SourceFilterDefaults{Namespace: "ns"},
				Sources: sreportalv1alpha2.SourcesSpec{
					Service: &sreportalv1alpha2.ServiceSourceSpec{
						CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true, LabelFilter: "!!!"},
					},
				},
			},
		},
		Data: dnschain.ChainData{},
	}
	require.Error(t, h.Handle(context.Background(), rc))
}
