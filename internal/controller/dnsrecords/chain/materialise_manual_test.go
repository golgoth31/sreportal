// internal/controller/dnsrecords/chain/materialise_manual_test.go
package chain_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

func TestMaterialiseManualEntriesHandler_ConvertEntriesToEndpoints(t *testing.T) {
	g := NewWithT(t)

	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "main-manual-apis", Namespace: tNsDefault},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "api.example.com", Group: "APIs", RecordType: "A", Targets: []string{"1.2.3.4"}},
				{FQDN: "graphql.example.com", Group: "APIs"},
				{FQDN: "health.example.com"},
			},
		},
	}

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: record,
		Data:     chain.ChainData{ResourceKey: "default/main-manual-apis"},
	}

	h := chain.NewMaterialiseManualEntriesHandler()
	err := h.Handle(context.Background(), rc)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(record.Status.Endpoints).To(HaveLen(3))
	g.Expect(record.Status.Endpoints[0].DNSName).To(Equal("api.example.com"))
	g.Expect(record.Status.Endpoints[0].RecordType).To(Equal("A"))
	g.Expect(record.Status.Endpoints[0].Targets).To(ConsistOf("1.2.3.4"))
	g.Expect(record.Status.Endpoints[1].DNSName).To(Equal("graphql.example.com"))
	g.Expect(record.Status.Endpoints[1].RecordType).To(Equal("A")) // default
	g.Expect(record.Status.Endpoints[0].LastSeen.IsZero()).To(BeFalse())
	g.Expect(record.Status.Endpoints[0].Labels["sreportal.io/group"]).To(Equal("APIs"))
	g.Expect(record.Status.Endpoints[2].Labels).To(BeNil())
}

func TestMaterialiseManualEntriesHandler_NoopForAuto(t *testing.T) {
	g := NewWithT(t)
	record := &v1alpha2.DNSRecord{
		Spec: v1alpha2.DNSRecordSpec{
			Origin:     v1alpha2.DNSRecordOriginAuto,
			SourceType: "ingress",
		},
	}
	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: record}
	h := chain.NewMaterialiseManualEntriesHandler()
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints).To(BeNil())
}

func TestMaterialiseManualEntriesHandler_EmptyEntries(t *testing.T) {
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
	h := chain.NewMaterialiseManualEntriesHandler()
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints).To(BeEmpty())
	g.Expect(record.Status.LastReconcileTime).NotTo(BeNil())
}

func TestMaterialiseManualEntriesHandler_RecordTypeVariants(t *testing.T) {
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
	h := chain.NewMaterialiseManualEntriesHandler()
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints).To(HaveLen(4))
	g.Expect(record.Status.Endpoints[0].RecordType).To(Equal("A"))
	g.Expect(record.Status.Endpoints[1].RecordType).To(Equal("AAAA"))
	g.Expect(record.Status.Endpoints[2].RecordType).To(Equal("CNAME"))
	g.Expect(record.Status.Endpoints[3].RecordType).To(Equal("TXT"))
}

func TestMaterialiseManualEntriesHandler_Idempotent(t *testing.T) {
	g := NewWithT(t)
	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "idem", Namespace: tNsDefault},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "a.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}},
				{FQDN: "b.example.com", RecordType: "A", Targets: []string{"5.6.7.8"}},
			},
		},
	}
	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: record}
	h := chain.NewMaterialiseManualEntriesHandler()
	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints).To(HaveLen(2))

	g.Expect(h.Handle(context.Background(), rc)).To(Succeed())
	g.Expect(record.Status.Endpoints).To(HaveLen(2))
	g.Expect(record.Status.Endpoints[0].DNSName).To(Equal("a.example.com"))
	g.Expect(record.Status.Endpoints[1].DNSName).To(Equal("b.example.com"))
}
