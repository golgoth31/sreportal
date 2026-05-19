package v1alpha1_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

// TestDNSRecordConvertTo_CorruptedAnnotation verifies that ConvertTo surfaces
// an error when the annotationV1Alpha2DNSRecordSpec annotation holds invalid
// JSON instead of silently producing a zero-value preserved spec.
func TestDNSRecordConvertTo_CorruptedAnnotation(t *testing.T) {
	g := NewWithT(t)

	src := &v1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"sreportal.io/v1alpha2-dnsrecord-spec": tInvalidJSON,
			},
		},
		Spec: v1alpha1.DNSRecordSpec{PortalRef: tPortalMain, SourceType: "ingress"},
	}

	dst := &v1alpha2.DNSRecord{}
	err := src.ConvertTo(dst)
	g.Expect(err).To(HaveOccurred(), "ConvertTo must return an error for corrupted annotation JSON")
}

// TestDNSRecordRoundTrip_StatusEndpoints verifies that Status.Endpoints
// round-trip intact through v1alpha1 (spoke) storage.
//
// The conversion code in dnsrecord_types.go explicitly copies Endpoints in
// both ConvertFrom and ConvertTo, so the round-trip is expected to succeed.
func TestDNSRecordRoundTrip_StatusEndpoints(t *testing.T) {
	g := NewWithT(t)

	now := metav1.Now()
	src := &v1alpha2.DNSRecord{
		Spec: v1alpha2.DNSRecordSpec{
			Origin:     v1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: v1alpha2.SourceTypeService,
		},
		Status: v1alpha2.DNSRecordStatus{
			Endpoints: []v1alpha2.EndpointStatus{
				{
					DNSName:    "svc.example.com",
					RecordType: "A",
					Targets:    []string{"10.0.0.1", "10.0.0.2"},
					TTL:        300,
					Labels:     map[string]string{"heritage": "external-dns"},
					SyncStatus: v1alpha2.SyncStatus("sync"),
					LastSeen:   now,
				},
			},
			EndpointsHash: "deadbeef",
		},
	}

	var spoke v1alpha1.DNSRecord
	if err := spoke.ConvertFrom(src); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}

	var hub v1alpha2.DNSRecord
	if err := spoke.ConvertTo(&hub); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}

	if diff := cmp.Diff(src.Status.Endpoints, hub.Status.Endpoints); diff != "" {
		t.Errorf("Status.Endpoints did not round-trip (-want +got):\n%s", diff)
	}
	g.Expect(hub.Status.EndpointsHash).To(Equal(src.Status.EndpointsHash))
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
