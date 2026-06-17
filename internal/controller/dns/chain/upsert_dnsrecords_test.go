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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnschain "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/gatewayhttproute"
	"github.com/golgoth31/sreportal/internal/source/ingress"
	"github.com/golgoth31/sreportal/internal/source/registry"
	"github.com/golgoth31/sreportal/internal/source/service"
)

const (
	upsertTestNS1     = "ns1"
	upsertTestRecord  = "d-service"
	upsertTestTargetA = "1.1.1.1"
)

func TestUpsertDNSRecords_CreatesAndDeletes(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: upsertTestNS1, UID: "u1"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "p"},
	}
	existing := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name: "d-ingress", Namespace: upsertTestNS1,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: sreportalv1alpha2.GroupVersion.String(),
				Kind:       "DNS",
				Name:       dns.Name,
				UID:        dns.UID,
				Controller: ptr.To(true), //nolint:modernize // new(bool) yields false, not true
			}},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			SourceType: sreportalv1alpha2.SourceType(ingress.SourceTypeIngress),
			PortalRef:  "p",
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&sreportalv1alpha2.DNSRecord{}).
		WithObjects(dns, existing).
		Build()

	h := &dnschain.UpsertDNSRecordsHandler{Client: c}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: dns,
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				service.SourceTypeService: {endpoint.NewEndpoint("a.example.com", "A", upsertTestTargetA)},
			},
		},
	}
	require.NoError(t, h.Handle(context.Background(), rc))

	var created sreportalv1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: upsertTestNS1, Name: upsertTestRecord}, &created))
	require.Equal(t, sreportalv1alpha2.DNSRecordOriginAuto, created.Spec.Origin)
	require.Equal(t, string(service.SourceTypeService), string(created.Spec.SourceType))
	require.Equal(t, "p", created.Spec.PortalRef)
	require.Len(t, created.Spec.Entries, 1)
	require.Equal(t, "a.example.com", created.Spec.Entries[0].FQDN)
	require.Equal(t, "A", created.Spec.Entries[0].RecordType)
	require.Equal(t, []string{upsertTestTargetA}, created.Spec.Entries[0].Targets)

	var gone sreportalv1alpha2.DNSRecord
	err := c.Get(context.Background(), types.NamespacedName{Namespace: upsertTestNS1, Name: "d-ingress"}, &gone)
	require.True(t, apierrors.IsNotFound(err), "expected d-ingress to be deleted, got err=%v", err)
}

// TestUpsertDNSRecordsHandler_MultipleKinds verifies that when ChainData
// carries endpoints for multiple source kinds, the handler creates one DNSRecord
// per kind (named {dnsName}-{kind}), each with the correct endpoints and
// an ownerReference pointing back to the DNS CR.
func TestUpsertDNSRecordsHandler_MultipleKinds(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: upsertTestNS1, UID: "uid-multi"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "portal-a"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&sreportalv1alpha2.DNSRecord{}).
		WithObjects(dns).
		Build()

	h := &dnschain.UpsertDNSRecordsHandler{Client: c}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: dns,
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				service.SourceTypeService: {
					endpoint.NewEndpoint("svc.example.com", "A", upsertTestTargetA),
				},
				ingress.SourceTypeIngress: {
					endpoint.NewEndpoint("ing.example.com", "A", "2.2.2.2"),
				},
				gatewayhttproute.SourceTypeGatewayHTTPRoute: {
					endpoint.NewEndpoint("gw.example.com", "A", "3.3.3.3"),
				},
			},
		},
	}

	require.NoError(t, h.Handle(context.Background(), rc))

	// Verify that exactly 3 DNSRecord CRs were created.
	var list sreportalv1alpha2.DNSRecordList
	require.NoError(t, c.List(context.Background(), &list))
	require.Len(t, list.Items, 3, "expected one DNSRecord per source kind")

	// Build a name→record map for deterministic assertions.
	byName := make(map[string]*sreportalv1alpha2.DNSRecord, 3)
	for i := range list.Items {
		byName[list.Items[i].Name] = &list.Items[i]
	}

	type kindExpect struct {
		kind    registry.SourceType
		dnsName string
		target  string
	}
	cases := []kindExpect{
		{service.SourceTypeService, "svc.example.com", "1.1.1.1"},
		{ingress.SourceTypeIngress, "ing.example.com", "2.2.2.2"},
		{gatewayhttproute.SourceTypeGatewayHTTPRoute, "gw.example.com", "3.3.3.3"},
	}

	for _, tc := range cases {
		name := fmt.Sprintf("d-%s", string(tc.kind))
		dr, ok := byName[name]
		require.True(t, ok, "expected DNSRecord %q to exist", name)

		// Correct source type annotation.
		require.Equal(t, sreportalv1alpha2.SourceType(tc.kind), dr.Spec.SourceType,
			"DNSRecord %q: wrong SourceType", name)

		// Spec.Entries contains the expected FQDN+target.
		require.Len(t, dr.Spec.Entries, 1, "DNSRecord %q: expected 1 spec entry", name)
		require.Equal(t, tc.dnsName, dr.Spec.Entries[0].FQDN,
			"DNSRecord %q: wrong FQDN", name)
		require.Equal(t, []string{tc.target}, dr.Spec.Entries[0].Targets,
			"DNSRecord %q: wrong Targets", name)

		// OwnerReference points back to the DNS CR.
		require.Len(t, dr.OwnerReferences, 1, "DNSRecord %q: expected 1 ownerReference", name)
		ref := dr.OwnerReferences[0]
		require.Equal(t, "DNS", ref.Kind, "DNSRecord %q: ownerRef.Kind", name)
		require.Equal(t, dns.UID, ref.UID, "DNSRecord %q: ownerRef.UID", name)
		require.Equal(t, dns.Name, ref.Name, "DNSRecord %q: ownerRef.Name", name)
		require.NotNil(t, ref.Controller, "DNSRecord %q: ownerRef.Controller must be set", name)
		require.True(t, *ref.Controller, "DNSRecord %q: ownerRef.Controller must be true", name)
	}
}

// TestUpsertDNSRecordsHandler_DualStack verifies that A and AAAA endpoints
// for the same FQDN survive the upsert path: with the composite listMapKey
// (fqdn, recordType) on spec.entries, the apiserver accepts both, and the
// handler preserves them as two distinct entries.
func TestUpsertDNSRecordsHandler_DualStack(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: upsertTestNS1, UID: "uid-dual"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "p"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&sreportalv1alpha2.DNSRecord{}).
		WithObjects(dns).
		Build()

	h := &dnschain.UpsertDNSRecordsHandler{Client: c}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: dns,
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				service.SourceTypeService: {
					endpoint.NewEndpoint("dual.example.com", "A", "1.2.3.4"),
					endpoint.NewEndpoint("dual.example.com", "AAAA", "::1"),
				},
			},
		},
	}
	require.NoError(t, h.Handle(context.Background(), rc))

	var created sreportalv1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: upsertTestNS1, Name: upsertTestRecord}, &created))
	require.Len(t, created.Spec.Entries, 2, "both A and AAAA must be preserved")
	byType := map[string]sreportalv1alpha2.DNSRecordEntry{}
	for _, e := range created.Spec.Entries {
		byType[e.RecordType] = e
	}
	require.Contains(t, byType, "A")
	require.Contains(t, byType, "AAAA")
	require.Equal(t, []string{"1.2.3.4"}, byType["A"].Targets)
	require.Equal(t, []string{"::1"}, byType["AAAA"].Targets)
}

// TestUpsertDNSRecordsHandler_DeterministicOrder verifies that the upsert
// path produces a stable spec.entries ordering regardless of the input slice
// order. Source endpoints come from a map iterated in random order; without
// a deterministic sort, spec would churn on every reconcile cycle and defeat
// the spec-canonical idempotence promised by the controller.
func TestUpsertDNSRecordsHandler_DeterministicOrder(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: upsertTestNS1, UID: "uid-det"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "p"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&sreportalv1alpha2.DNSRecord{}).
		WithObjects(dns).
		Build()

	h := &dnschain.UpsertDNSRecordsHandler{Client: c}
	build := func(eps []*endpoint.Endpoint) *reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData] {
		return &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
			Resource: dns,
			Data: dnschain.ChainData{
				KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
					service.SourceTypeService: eps,
				},
			},
		}
	}

	order1 := []*endpoint.Endpoint{
		endpoint.NewEndpoint("b.example.com", "A", "2.2.2.2"),
		endpoint.NewEndpoint("a.example.com", "A", upsertTestTargetA),
		endpoint.NewEndpoint("a.example.com", "AAAA", "::1"),
	}
	require.NoError(t, h.Handle(context.Background(), build(order1)))

	var first sreportalv1alpha2.DNSRecord
	key := types.NamespacedName{Namespace: upsertTestNS1, Name: upsertTestRecord}
	require.NoError(t, c.Get(context.Background(), key, &first))
	firstRV := first.ResourceVersion

	order2 := []*endpoint.Endpoint{
		endpoint.NewEndpoint("a.example.com", "AAAA", "::1"),
		endpoint.NewEndpoint("b.example.com", "A", "2.2.2.2"),
		endpoint.NewEndpoint("a.example.com", "A", upsertTestTargetA),
	}
	require.NoError(t, h.Handle(context.Background(), build(order2)))

	var second sreportalv1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), key, &second))
	require.Equal(t, firstRV, second.ResourceVersion,
		"identical content with different input order must not bump ResourceVersion")

	require.Len(t, second.Spec.Entries, 3)
	require.Equal(t, "a.example.com", second.Spec.Entries[0].FQDN)
	require.Equal(t, "A", second.Spec.Entries[0].RecordType)
	require.Equal(t, "a.example.com", second.Spec.Entries[1].FQDN)
	require.Equal(t, "AAAA", second.Spec.Entries[1].RecordType)
	require.Equal(t, "b.example.com", second.Spec.Entries[2].FQDN)
}

// TestUpsertDNSRecordsHandler_DedupSameKey verifies that duplicate
// (FQDN, RecordType) entries from a source are collapsed to one, with
// targets merged and sorted. external-dns can emit duplicate endpoints
// when multiple Services/Ingresses share an FQDN.
func TestUpsertDNSRecordsHandler_DedupSameKey(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: upsertTestNS1, UID: "uid-dedup"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "p"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&sreportalv1alpha2.DNSRecord{}).
		WithObjects(dns).
		Build()

	h := &dnschain.UpsertDNSRecordsHandler{Client: c}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: dns,
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				service.SourceTypeService: {
					endpoint.NewEndpoint("dup.example.com", "A", "2.2.2.2"),
					endpoint.NewEndpoint("dup.example.com", "A", upsertTestTargetA),
				},
			},
		},
	}
	require.NoError(t, h.Handle(context.Background(), rc))

	var created sreportalv1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: upsertTestNS1, Name: upsertTestRecord}, &created))
	require.Len(t, created.Spec.Entries, 1, "duplicate (fqdn, recordType) must be deduplicated")
	require.Equal(t, []string{"1.1.1.1", "2.2.2.2"}, created.Spec.Entries[0].Targets,
		"targets from duplicates must be merged and sorted")
}

// TestUpsertDNSRecordsHandler_NoOpWhenUnchanged verifies that a second
// reconcile with identical endpoints does not bump the DNSRecord
// generation or ResourceVersion (the foundation of the burst-cascade fix).
func TestUpsertDNSRecordsHandler_NoOpWhenUnchanged(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: upsertTestNS1, UID: "uid-noop"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "p"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&sreportalv1alpha2.DNSRecord{}).
		WithObjects(dns).
		Build()

	h := &dnschain.UpsertDNSRecordsHandler{Client: c}
	build := func() *reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData] {
		return &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
			Resource: dns,
			Data: dnschain.ChainData{
				KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
					service.SourceTypeService: {endpoint.NewEndpoint("a.example.com", "A", upsertTestTargetA)},
				},
			},
		}
	}

	require.NoError(t, h.Handle(context.Background(), build()))

	var first sreportalv1alpha2.DNSRecord
	key := types.NamespacedName{Namespace: upsertTestNS1, Name: upsertTestRecord}
	require.NoError(t, c.Get(context.Background(), key, &first))
	firstGen := first.Generation
	firstRV := first.ResourceVersion

	require.NoError(t, h.Handle(context.Background(), build()))

	var second sreportalv1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), key, &second))
	require.Equal(t, firstGen, second.Generation, "generation must not bump on no-op upsert")
	require.Equal(t, firstRV, second.ResourceVersion, "no write expected when content is identical")
}

// TestUpsertDNSRecordsHandler_PropagatesOriginRef verifies that the external-dns
// "resource" label on a kept endpoint is carried into the projected
// DNSRecordEntry.OriginRef so the origin survives the spec.entries hop.
func TestUpsertDNSRecordsHandler_PropagatesOriginRef(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: upsertTestNS1, UID: "u1"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "p"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&sreportalv1alpha2.DNSRecord{}).
		WithObjects(dns).
		Build()

	ep := endpoint.NewEndpoint("a.example.com", "A", upsertTestTargetA).
		WithLabel(endpoint.ResourceLabelKey, "service/ns1/budget-controls")

	h := &dnschain.UpsertDNSRecordsHandler{Client: c}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: dns,
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				service.SourceTypeService: {ep},
			},
		},
	}
	require.NoError(t, h.Handle(context.Background(), rc))

	var created sreportalv1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: upsertTestNS1, Name: upsertTestRecord}, &created))
	require.Len(t, created.Spec.Entries, 1)
	require.Equal(t, "service/ns1/budget-controls", created.Spec.Entries[0].OriginRef)
}

// TestUpsertDNSRecordsHandler_OriginRefFollowsPriority verifies the OriginRef
// carried into spec.entries is the one of the source that wins source priority:
// IntraDNSDedup keeps the higher-priority kind's endpoint (with its resource
// label), so the projected entry's OriginRef is that winner's resource.
func TestUpsertDNSRecordsHandler_OriginRefFollowsPriority(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: upsertTestNS1, UID: "u1"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "p"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&sreportalv1alpha2.DNSRecord{}).
		WithObjects(dns).
		Build()

	// Same FQDN produced by both ingress and service, each with its own origin.
	// Priority: ingress before service -> ingress wins.
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: dns,
		Data: dnschain.ChainData{
			PriorityOrder: []registry.SourceType{ingress.SourceTypeIngress, service.SourceTypeService},
			EndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				ingress.SourceTypeIngress: {
					endpoint.NewEndpoint("shared.example.com", "A", upsertTestTargetA).
						WithLabel(endpoint.ResourceLabelKey, "ingress/ns1/shared-ing"),
				},
				service.SourceTypeService: {
					endpoint.NewEndpoint("shared.example.com", "A", "2.2.2.2").
						WithLabel(endpoint.ResourceLabelKey, "service/ns1/shared-svc"),
				},
			},
		},
	}

	require.NoError(t, (&dnschain.IntraDNSDedupHandler{}).Handle(context.Background(), rc))
	require.NoError(t, (&dnschain.UpsertDNSRecordsHandler{Client: c}).Handle(context.Background(), rc))

	// Winner (ingress) carries the FQDN with the ingress origin.
	var ingRec sreportalv1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: upsertTestNS1, Name: "d-ingress"}, &ingRec))
	require.Len(t, ingRec.Spec.Entries, 1)
	require.Equal(t, "shared.example.com", ingRec.Spec.Entries[0].FQDN)
	require.Equal(t, "ingress/ns1/shared-ing", ingRec.Spec.Entries[0].OriginRef)

	// Loser (service) record must not contain the deduped FQDN.
	var svcRec sreportalv1alpha2.DNSRecord
	err := c.Get(context.Background(), types.NamespacedName{Namespace: upsertTestNS1, Name: upsertTestRecord}, &svcRec)
	if err == nil {
		for _, e := range svcRec.Spec.Entries {
			require.NotEqual(t, "shared.example.com", e.FQDN, "deduped FQDN must not appear in the lower-priority record")
		}
	} else {
		require.True(t, apierrors.IsNotFound(err))
	}
}
