package v1alpha1_test

import (
	"testing"

	. "github.com/onsi/gomega"

	v1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

func TestDNSRecordConvertTo_SetsOriginAuto(t *testing.T) {
	g := NewWithT(t)
	src := &v1alpha1.DNSRecord{
		Spec: v1alpha1.DNSRecordSpec{
			PortalRef:  tPortalMain,
			SourceType: "ingress",
		},
	}
	dst := &v1alpha2.DNSRecord{}
	g.Expect(src.ConvertTo(dst)).To(Succeed())
	g.Expect(dst.Spec.Origin).To(Equal(v1alpha2.DNSRecordOriginAuto))
	g.Expect(dst.Spec.PortalRef).To(Equal(tPortalMain))
	g.Expect(dst.Spec.SourceType).To(Equal(v1alpha2.SourceType("ingress")))
}

func TestDNSRecordConvertTo_PreservesStatus(t *testing.T) {
	g := NewWithT(t)
	src := &v1alpha1.DNSRecord{
		Spec: v1alpha1.DNSRecordSpec{PortalRef: tPortalMain, SourceType: "service"},
		Status: v1alpha1.DNSRecordStatus{
			EndpointsHash: "abc123",
		},
	}
	dst := &v1alpha2.DNSRecord{}
	g.Expect(src.ConvertTo(dst)).To(Succeed())
	g.Expect(dst.Status.EndpointsHash).To(Equal("abc123"))
}

func TestDNSRecordConvertFrom_PreservesOriginAuto(t *testing.T) {
	g := NewWithT(t)
	src := &v1alpha2.DNSRecord{
		Spec: v1alpha2.DNSRecordSpec{
			Origin:     v1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: "service",
		},
	}
	dst := &v1alpha1.DNSRecord{}
	g.Expect(dst.ConvertFrom(src)).To(Succeed())
	g.Expect(dst.Spec.PortalRef).To(Equal(tPortalMain))
	g.Expect(dst.Spec.SourceType).To(Equal("service"))
}

func TestDNSRecordConvertFrom_ManualDropsEntries(t *testing.T) {
	g := NewWithT(t)
	src := &v1alpha2.DNSRecord{
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries:   []v1alpha2.DNSRecordEntry{{FQDN: tFQDNAPIExamp}},
		},
	}
	dst := &v1alpha1.DNSRecord{}
	g.Expect(dst.ConvertFrom(src)).To(Succeed())
	g.Expect(dst.Spec.PortalRef).To(Equal(tPortalMain))
	g.Expect(dst.Spec.SourceType).To(BeEmpty())
	// Entries are not representable in v1alpha1 — verify status not populated from them
	g.Expect(dst.Status.Endpoints).To(BeNil())
}

func TestDNSRecordRoundTrip_PreservesManualEntries(t *testing.T) {
	g := NewWithT(t)

	src := &v1alpha2.DNSRecord{
		Spec: v1alpha2.DNSRecordSpec{
			Origin:    v1alpha2.DNSRecordOriginManual,
			PortalRef: tPortalMain,
			Entries: []v1alpha2.DNSRecordEntry{
				{FQDN: "manual.example.com", Group: "Apps", RecordType: "A", Targets: []string{"10.0.0.1"}},
			},
		},
	}

	var spoke v1alpha1.DNSRecord
	g.Expect(spoke.ConvertFrom(src)).To(Succeed())
	g.Expect(spoke.Annotations).To(HaveKey("sreportal.io/v1alpha2-dnsrecord-spec"))

	var hub v1alpha2.DNSRecord
	g.Expect(spoke.ConvertTo(&hub)).To(Succeed())
	g.Expect(hub.Spec.Origin).To(Equal(v1alpha2.DNSRecordOriginManual))
	g.Expect(hub.Spec.Entries).To(HaveLen(1))
	g.Expect(hub.Spec.Entries[0].FQDN).To(Equal("manual.example.com"))
	g.Expect(hub.Annotations).NotTo(HaveKey("sreportal.io/v1alpha2-dnsrecord-spec"))
}

func TestDNSRecordRoundTrip_AutoUnchanged(t *testing.T) {
	g := NewWithT(t)
	src := &v1alpha2.DNSRecord{Spec: v1alpha2.DNSRecordSpec{
		Origin: v1alpha2.DNSRecordOriginAuto, PortalRef: tPortalMain, SourceType: v1alpha2.SourceTypeService,
	}}

	var spoke v1alpha1.DNSRecord
	g.Expect(spoke.ConvertFrom(src)).To(Succeed())
	var hub v1alpha2.DNSRecord
	g.Expect(spoke.ConvertTo(&hub)).To(Succeed())
	g.Expect(hub.Spec.Origin).To(Equal(v1alpha2.DNSRecordOriginAuto))
	g.Expect(hub.Spec.SourceType).To(Equal(v1alpha2.SourceTypeService))
	g.Expect(hub.Spec.Entries).To(BeEmpty())
}
