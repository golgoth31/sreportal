// internal/controller/dnsrecords/chain/materialise_entries_test.go
package chain_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

func TestMaterialiseEntriesHandler_ConvertEntriesToEndpoints(t *testing.T) {
	g := NewWithT(t)

	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "main-manual-apis", Namespace: tNsDefault},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "api.example.com", Group: "APIs", RecordType: "A", Targets: []string{tIP1234}},
				{FQDN: "graphql.example.com", Group: "APIs"},
				{FQDN: "health.example.com"},
			},
		},
	}

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: record,
		Data:     chain.ChainData{ResourceKey: "default/main-manual-apis"},
	}

	h := chain.NewMaterialiseEntriesHandler(nil)
	err := h.Handle(context.Background(), rc)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(record.Status.Endpoints).To(HaveLen(3))
	g.Expect(record.Status.Endpoints[0].DNSName).To(Equal("api.example.com"))
	g.Expect(record.Status.Endpoints[0].RecordType).To(Equal("A"))
	g.Expect(record.Status.Endpoints[0].Targets).To(ConsistOf(tIP1234))
	g.Expect(record.Status.Endpoints[1].DNSName).To(Equal("graphql.example.com"))
	g.Expect(record.Status.Endpoints[1].RecordType).To(Equal("A")) // default
	g.Expect(record.Status.Endpoints[0].LastSeen.IsZero()).To(BeFalse())
	g.Expect(record.Status.Endpoints[0].Labels["sreportal.io/group"]).To(Equal("APIs"))
	g.Expect(record.Status.Endpoints[2].Labels).To(BeNil())
	g.Expect(record.Status.EndpointsHash).NotTo(BeEmpty())
}

func TestMaterialiseEntriesHandler_MaterialisesForAuto(t *testing.T) {
	g := NewWithT(t)
	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "auto", Namespace: tNsDefault},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:     v1alpha2.DNSRecordOriginAuto,
			SourceType: "ingress",
			PortalRef:  tPortalMain,
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "auto.example.com", RecordType: "A", Targets: []string{tIP1234}},
			},
		},
	}
	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: record}
	h := chain.NewMaterialiseEntriesHandler(nil)
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints).To(HaveLen(1))
	g.Expect(record.Status.Endpoints[0].DNSName).To(Equal("auto.example.com"))
	g.Expect(record.Status.EndpointsHash).NotTo(BeEmpty())
}

func TestMaterialiseEntriesHandler_EmptyEntries(t *testing.T) {
	g := NewWithT(t)
	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "empty", Namespace: tNsDefault},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   nil,
		},
	}
	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: record}
	h := chain.NewMaterialiseEntriesHandler(nil)
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints).To(BeEmpty())
	g.Expect(record.Status.LastReconcileTime).NotTo(BeNil())
	g.Expect(record.Status.EndpointsHash).To(Equal(""))
}

func TestMaterialiseEntriesHandler_RecordTypeVariants(t *testing.T) {
	g := NewWithT(t)
	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "variants", Namespace: tNsDefault},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "default.example.com"},
				{FQDN: "v6.example.com", RecordType: "AAAA", Targets: []string{"::1"}},
				{FQDN: "alias.example.com", RecordType: "CNAME", Targets: []string{"target.example.com"}},
				{FQDN: "txt.example.com", RecordType: "TXT", Targets: []string{"hello"}},
			},
		},
	}
	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: record}
	h := chain.NewMaterialiseEntriesHandler(nil)
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints).To(HaveLen(4))
	g.Expect(record.Status.Endpoints[0].RecordType).To(Equal("A"))
	g.Expect(record.Status.Endpoints[1].RecordType).To(Equal("AAAA"))
	g.Expect(record.Status.Endpoints[2].RecordType).To(Equal("CNAME"))
	g.Expect(record.Status.Endpoints[3].RecordType).To(Equal("TXT"))
}

// TestMaterialiseEntriesHandler_PersistsStatus verifies that the handler
// patches DNSRecord status when the endpoints hash changes — so downstream
// handlers (ResolveDNS, ProjectStore) cannot drop the materialised status
// by short-circuiting.
func TestMaterialiseEntriesHandler_PersistsStatus(t *testing.T) {
	const persistName = "persist"
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(v1alpha2.AddToScheme(scheme)).To(Succeed())

	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: persistName, Namespace: tNsDefault, Generation: 3},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "p.example.com", RecordType: "A", Targets: []string{tIP1234}},
			},
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1alpha2.DNSRecord{}).
		WithObjects(record).
		Build()

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: record}
	h := chain.NewMaterialiseEntriesHandler(c)
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())

	var got v1alpha2.DNSRecord
	g.Expect(c.Get(context.Background(), types.NamespacedName{Namespace: tNsDefault, Name: persistName}, &got)).To(Succeed())
	g.Expect(got.Status.Endpoints).To(HaveLen(1))
	g.Expect(got.Status.EndpointsHash).NotTo(BeEmpty())
	g.Expect(got.Status.ObservedGeneration).To(Equal(int64(3)))
	g.Expect(got.Status.LastReconcileTime).NotTo(BeNil())

	// Second call with the same content: must not patch (hash + obsGen unchanged).
	rv := got.ResourceVersion
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(c.Get(context.Background(), types.NamespacedName{Namespace: tNsDefault, Name: persistName}, &got)).To(Succeed())
	g.Expect(got.ResourceVersion).To(Equal(rv), "no-op materialise must not patch status")
}

func TestMaterialiseEntriesHandler_Idempotent(t *testing.T) {
	g := NewWithT(t)
	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "idem", Namespace: tNsDefault},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "a.example.com", RecordType: "A", Targets: []string{tIP1234}},
				{FQDN: "b.example.com", RecordType: "A", Targets: []string{"5.6.7.8"}},
			},
		},
	}
	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: record}
	h := chain.NewMaterialiseEntriesHandler(nil)
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints).To(HaveLen(2))

	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints).To(HaveLen(2))
	g.Expect(record.Status.Endpoints[0].DNSName).To(Equal("a.example.com"))
	g.Expect(record.Status.Endpoints[1].DNSName).To(Equal("b.example.com"))
}

// TestMaterialiseEntriesHandler_ReinjectsOriginRef verifies that an entry's
// OriginRef is re-injected into the status endpoint's external-dns "resource"
// label, so the adapter can derive FQDNView.OriginRef downstream. The hash
// excludes the resource label, so an OriginRef-only change must not churn it.
func TestMaterialiseEntriesHandler_ReinjectsOriginRef(t *testing.T) {
	g := NewWithT(t)

	withOrigin := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "auto-origin", Namespace: tNsDefault},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:     v1alpha2.DNSRecordOriginAuto,
			SourceType: "service",
			PortalRef:  tPortalMain,
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "a.example.com", Group: "APIs", RecordType: "A", Targets: []string{tIP1234}, OriginRef: "service/ns1/budget-controls"},
			},
		},
	}
	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: withOrigin}
	h := chain.NewMaterialiseEntriesHandler(nil)
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(withOrigin.Status.Endpoints).To(HaveLen(1))
	g.Expect(withOrigin.Status.Endpoints[0].Labels["resource"]).To(Equal("service/ns1/budget-controls"))
	g.Expect(withOrigin.Status.Endpoints[0].Labels["sreportal.io/group"]).To(Equal("APIs"))
	hashWithOrigin := withOrigin.Status.EndpointsHash

	// Same entry without OriginRef: the hash must be identical (resource label
	// is excluded from the hash), and no resource label is set.
	noOrigin := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "auto-noorigin", Namespace: tNsDefault},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:     v1alpha2.DNSRecordOriginAuto,
			SourceType: "service",
			PortalRef:  tPortalMain,
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "a.example.com", Group: "APIs", RecordType: "A", Targets: []string{tIP1234}},
			},
		},
	}
	rc2 := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: noOrigin}
	g.Expect(h.Handle(context.Background(), rc2)).To(Succeed())
	g.Expect(noOrigin.Status.Endpoints[0].Labels).NotTo(HaveKey("resource"))
	g.Expect(noOrigin.Status.EndpointsHash).To(Equal(hashWithOrigin), "OriginRef must not affect the endpoints hash")
}
