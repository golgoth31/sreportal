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
	"github.com/golgoth31/sreportal/internal/source/externaldns"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

func TestIntraDNSDedup_FirstKindWins(t *testing.T) {
	h := &dnschain.IntraDNSDedupHandler{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Data: dnschain.ChainData{
			PriorityOrder: []registry.SourceType{externaldns.KindService, externaldns.KindIngress},
			EndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				externaldns.KindService: {endpoint.NewEndpoint("a.example.com", "A", "1.1.1.1")},
				externaldns.KindIngress: {
					endpoint.NewEndpoint("a.example.com", "A", "2.2.2.2"),
					endpoint.NewEndpoint("b.example.com", "A", "3.3.3.3"),
				},
			},
		},
	}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Len(t, rc.Data.KeptEndpointsByKind[externaldns.KindService], 1)
	require.Equal(t, "a.example.com", rc.Data.KeptEndpointsByKind[externaldns.KindService][0].DNSName)
	require.Len(t, rc.Data.KeptEndpointsByKind[externaldns.KindIngress], 1)
	require.Equal(t, "b.example.com", rc.Data.KeptEndpointsByKind[externaldns.KindIngress][0].DNSName)
}

// TestIntraDNSDedup_CrossRecordTypeOwnership verifies FQDN-level priority: a
// lower-priority kind contributing the same FQDN with a DIFFERENT record type
// (ingress→A vs ExternalName service→CNAME) is dropped entirely, not kept
// alongside the winner.
func TestIntraDNSDedup_CrossRecordTypeOwnership(t *testing.T) {
	h := &dnschain.IntraDNSDedupHandler{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Data: dnschain.ChainData{
			PriorityOrder: []registry.SourceType{externaldns.KindIngress, externaldns.KindService},
			EndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				externaldns.KindIngress: {endpoint.NewEndpoint("app.example.com", "A", "1.1.1.1")},
				externaldns.KindService: {endpoint.NewEndpoint("app.example.com", "CNAME", "lb.example.net")},
			},
		},
	}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Len(t, rc.Data.KeptEndpointsByKind[externaldns.KindIngress], 1)
	require.Equal(t, "A", rc.Data.KeptEndpointsByKind[externaldns.KindIngress][0].RecordType)
	require.Empty(t, rc.Data.KeptEndpointsByKind[externaldns.KindService],
		"lower-priority kind must not contribute a CNAME for an FQDN owned by ingress")
}

// TestIntraDNSDedup_KeepsMultiRecordTypeFromWinner verifies the winning kind
// keeps all its record types for one FQDN (dual-stack A + AAAA), while a
// lower-priority kind is still excluded for that name.
func TestIntraDNSDedup_KeepsMultiRecordTypeFromWinner(t *testing.T) {
	h := &dnschain.IntraDNSDedupHandler{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Data: dnschain.ChainData{
			PriorityOrder: []registry.SourceType{externaldns.KindIngress, externaldns.KindService},
			EndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				externaldns.KindIngress: {
					endpoint.NewEndpoint("app.example.com", "A", "1.1.1.1"),
					endpoint.NewEndpoint("app.example.com", "AAAA", "2001:db8::1"),
				},
				externaldns.KindService: {endpoint.NewEndpoint("app.example.com", "A", "9.9.9.9")},
			},
		},
	}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Len(t, rc.Data.KeptEndpointsByKind[externaldns.KindIngress], 2,
		"winning kind keeps both A and AAAA for the same FQDN")
	require.Empty(t, rc.Data.KeptEndpointsByKind[externaldns.KindService])
}
