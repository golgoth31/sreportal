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

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnschain "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/externaldns"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

func newDNSFor(portal string) *sreportalv1alpha2.DNS {
	dns := &sreportalv1alpha2.DNS{}
	dns.Spec.PortalRef = portal
	return dns
}

func TestValidateEntries_MixedDropsInvalidKeepsValid(t *testing.T) {
	const portal = "portal-mixed"
	h := &dnschain.ValidateEntriesHandler{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: newDNSFor(portal),
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				externaldns.KindService: {
					endpoint.NewEndpoint("good.example.com", "A", "1.1.1.1"),
					endpoint.NewEndpoint("override-on-app-set", "A", "2.2.2.2"),
				},
			},
		},
	}

	before := testutil.ToFloat64(metrics.DNSEntriesInvalid.WithLabelValues(portal, string(externaldns.KindService), "invalid_fqdn"))
	require.NoError(t, h.Handle(context.Background(), rc))

	kept := rc.Data.KeptEndpointsByKind[externaldns.KindService]
	require.Len(t, kept, 1)
	require.Equal(t, "good.example.com", kept[0].DNSName)

	require.Len(t, rc.Data.SkippedEntries, 1)
	require.Equal(t, "override-on-app-set", rc.Data.SkippedEntries[0].FQDN)
	require.Equal(t, "invalid_fqdn", rc.Data.SkippedEntries[0].Reason)
	require.Equal(t, externaldns.KindService, rc.Data.SkippedEntries[0].Kind)

	require.Equal(t, float64(1), testutil.ToFloat64(metrics.DNSEntriesValid.WithLabelValues(portal, string(externaldns.KindService))))
	after := testutil.ToFloat64(metrics.DNSEntriesInvalid.WithLabelValues(portal, string(externaldns.KindService), "invalid_fqdn"))
	require.Equal(t, float64(1), after-before)
}

func TestValidateEntries_AllInvalidYieldsEmptyKindAndPreserves(t *testing.T) {
	const portal = "portal-allbad"
	h := &dnschain.ValidateEntriesHandler{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: newDNSFor(portal),
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				externaldns.KindService: {
					endpoint.NewEndpoint("override-on-app-set", "A", "2.2.2.2"),
					endpoint.NewEndpoint("also_bad", "A", "3.3.3.3"),
				},
			},
		},
	}

	require.NoError(t, h.Handle(context.Background(), rc))
	require.Empty(t, rc.Data.KeptEndpointsByKind[externaldns.KindService])
	require.Len(t, rc.Data.SkippedEntries, 2)
	require.Equal(t, float64(0), testutil.ToFloat64(metrics.DNSEntriesValid.WithLabelValues(portal, string(externaldns.KindService))))
	// A kind that had drops must be preserved so upsert does not delete its
	// last-good DNSRecord on a transient all-invalid glitch.
	require.True(t, rc.Data.PreserveKinds[externaldns.KindService])
}

func TestValidateEntries_InvalidRecordTypeSkipped(t *testing.T) {
	const portal = "portal-rectype"
	h := &dnschain.ValidateEntriesHandler{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: newDNSFor(portal),
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				externaldns.KindService: {
					endpoint.NewEndpoint("mail.example.com", "MX", "10 mx.example.com"),
					endpoint.NewEndpoint("web.example.com", "A", "1.1.1.1"),
				},
			},
		},
	}

	before := testutil.ToFloat64(metrics.DNSEntriesInvalid.WithLabelValues(portal, string(externaldns.KindService), "invalid_record_type"))
	require.NoError(t, h.Handle(context.Background(), rc))

	kept := rc.Data.KeptEndpointsByKind[externaldns.KindService]
	require.Len(t, kept, 1)
	require.Equal(t, "web.example.com", kept[0].DNSName)
	require.Len(t, rc.Data.SkippedEntries, 1)
	require.Equal(t, "mail.example.com", rc.Data.SkippedEntries[0].FQDN)
	require.Equal(t, "invalid_record_type", rc.Data.SkippedEntries[0].Reason)
	after := testutil.ToFloat64(metrics.DNSEntriesInvalid.WithLabelValues(portal, string(externaldns.KindService), "invalid_record_type"))
	require.Equal(t, float64(1), after-before)
}

func TestValidateEntries_CrossKindRecordTypeTiebreak(t *testing.T) {
	h := &dnschain.ValidateEntriesHandler{}
	// Same invalid FQDN produced by two kinds, and within one kind the same FQDN
	// with two record types — exercises the (FQDN, RecordType, Kind) sort order.
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: newDNSFor("portal-tiebreak"),
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				externaldns.KindIngress: {endpoint.NewEndpoint("bad_name", "AAAA", "::1")},
				externaldns.KindService: {endpoint.NewEndpoint("bad_name", "A", "1.1.1.1")},
			},
		},
	}

	require.NoError(t, h.Handle(context.Background(), rc))
	require.Len(t, rc.Data.SkippedEntries, 2)
	// Sorted by RecordType (A < AAAA) since FQDN is equal.
	require.Equal(t, "A", rc.Data.SkippedEntries[0].RecordType)
	require.Equal(t, externaldns.KindService, rc.Data.SkippedEntries[0].Kind)
	require.Equal(t, "AAAA", rc.Data.SkippedEntries[1].RecordType)
	require.Equal(t, externaldns.KindIngress, rc.Data.SkippedEntries[1].Kind)
}

func TestValidateEntries_AllValidPassthrough(t *testing.T) {
	h := &dnschain.ValidateEntriesHandler{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: newDNSFor("portal-clean"),
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				externaldns.KindService: {
					endpoint.NewEndpoint("a.example.com", "A", "1.1.1.1"),
					endpoint.NewEndpoint("b.example.com", "A", "2.2.2.2"),
				},
			},
		},
	}

	require.NoError(t, h.Handle(context.Background(), rc))
	require.Len(t, rc.Data.KeptEndpointsByKind[externaldns.KindService], 2)
	require.Empty(t, rc.Data.SkippedEntries)
}

func TestValidateEntries_SkippedOrderDeterministic(t *testing.T) {
	h := &dnschain.ValidateEntriesHandler{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: newDNSFor("portal-order"),
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				externaldns.KindService: {
					endpoint.NewEndpoint("zzz_bad", "A", "1.1.1.1"),
					endpoint.NewEndpoint("aaa_bad", "A", "2.2.2.2"),
				},
			},
		},
	}

	require.NoError(t, h.Handle(context.Background(), rc))
	require.Len(t, rc.Data.SkippedEntries, 2)
	require.Equal(t, "aaa_bad", rc.Data.SkippedEntries[0].FQDN)
	require.Equal(t, "zzz_bad", rc.Data.SkippedEntries[1].FQDN)
}
