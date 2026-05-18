package v1alpha1_test

import (
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

const (
	tPortalMain   = "main"
	tGroupAPIs    = "APIs"
	tFQDNAPIExamp = "api.example.com"
)

func TestDNSConvertTo_PreservesGroups(t *testing.T) {
	g := NewWithT(t)

	src := &v1alpha1.DNS{
		Spec: v1alpha1.DNSSpec{
			PortalRef: tPortalMain,
			Groups: []v1alpha1.DNSGroup{
				{
					Name: tGroupAPIs,
					Entries: []v1alpha1.DNSEntry{
						{FQDN: tFQDNAPIExamp, Description: "Main API"},
					},
				},
			},
		},
	}

	dst := &v1alpha2.DNS{}
	err := src.ConvertTo(dst)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(dst.Spec.PortalRef).To(Equal(tPortalMain))

	// groups preserved in annotation
	raw := dst.Annotations["sreportal.io/v1alpha1-groups"]
	g.Expect(raw).NotTo(BeEmpty())
	var groups []v1alpha1.DNSGroup
	g.Expect(json.Unmarshal([]byte(raw), &groups)).To(Succeed())
	g.Expect(groups).To(HaveLen(1))
	g.Expect(groups[0].Name).To(Equal(tGroupAPIs))
}

func TestDNSConvertTo_EmptyGroups(t *testing.T) {
	g := NewWithT(t)

	src := &v1alpha1.DNS{
		Spec: v1alpha1.DNSSpec{PortalRef: tPortalMain},
	}
	dst := &v1alpha2.DNS{}
	g.Expect(src.ConvertTo(dst)).To(Succeed())
	// No annotation when groups is empty
	_, ok := dst.Annotations["sreportal.io/v1alpha1-groups"]
	g.Expect(ok).To(BeFalse())
}

func TestDNSConvertFrom_RestoresGroups(t *testing.T) {
	g := NewWithT(t)

	groups := []v1alpha1.DNSGroup{{Name: tGroupAPIs, Entries: []v1alpha1.DNSEntry{{FQDN: tFQDNAPIExamp}}}}
	raw, _ := json.Marshal(groups)

	src := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"sreportal.io/v1alpha1-groups": string(raw)},
		},
		Spec: v1alpha2.DNSSpec{PortalRef: tPortalMain},
	}

	dst := &v1alpha1.DNS{}
	err := dst.ConvertFrom(src)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(dst.Spec.PortalRef).To(Equal(tPortalMain))
	g.Expect(dst.Spec.Groups).To(HaveLen(1))
	g.Expect(dst.Spec.Groups[0].Name).To(Equal(tGroupAPIs))
}

func TestDNSConvertFrom_NoAnnotation(t *testing.T) {
	g := NewWithT(t)
	src := &v1alpha2.DNS{
		Spec: v1alpha2.DNSSpec{PortalRef: tPortalMain, IsRemote: true},
	}
	dst := &v1alpha1.DNS{}
	g.Expect(dst.ConvertFrom(src)).To(Succeed())
	g.Expect(dst.Spec.IsRemote).To(BeTrue())
	g.Expect(dst.Spec.Groups).To(BeNil())
}

func TestDNSConvertFrom_AnnotationCleanedUp(t *testing.T) {
	g := NewWithT(t)
	groups := []v1alpha1.DNSGroup{{Name: tGroupAPIs, Entries: []v1alpha1.DNSEntry{{FQDN: tFQDNAPIExamp}}}}
	raw, _ := json.Marshal(groups)
	src := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"sreportal.io/v1alpha1-groups": string(raw)},
		},
		Spec: v1alpha2.DNSSpec{PortalRef: tPortalMain},
	}
	dst := &v1alpha1.DNS{}
	g.Expect(dst.ConvertFrom(src)).To(Succeed())
	g.Expect(dst.Annotations).NotTo(HaveKey("sreportal.io/v1alpha1-groups"))
	g.Expect(dst.Spec.Groups).To(HaveLen(1))
}

func TestDNSRoundTrip_PreservesV1Alpha2OnlySpec(t *testing.T) {
	g := NewWithT(t)

	src := &v1alpha2.DNS{
		Spec: v1alpha2.DNSSpec{
			PortalRef: "main",
			Sources: v1alpha2.SourcesSpec{
				Service: &v1alpha2.ServiceSourceSpec{
					CommonSourceSpec: v1alpha2.CommonSourceSpec{Enabled: true, Namespace: "prod"},
				},
				Priority: []v1alpha2.SourceType{v1alpha2.SourceTypeService},
			},
			GroupMapping: v1alpha2.GroupMappingSpec{DefaultGroup: "Apps", LabelKey: "team"},
			Reconciliation: v1alpha2.ReconciliationSpec{
				Interval:        metav1.Duration{Duration: 90 * time.Second},
				DisableDNSCheck: true,
			},
		},
	}

	var spoke v1alpha1.DNS
	g.Expect(spoke.ConvertFrom(src)).To(Succeed())

	var hub v1alpha2.DNS
	g.Expect(spoke.ConvertTo(&hub)).To(Succeed())

	g.Expect(hub.Spec.Sources.Service).NotTo(BeNil())
	g.Expect(hub.Spec.Sources.Service.Enabled).To(BeTrue())
	g.Expect(hub.Spec.Sources.Service.Namespace).To(Equal("prod"))
	g.Expect(hub.Spec.Sources.Priority).To(ConsistOf(v1alpha2.SourceTypeService))
	g.Expect(hub.Spec.GroupMapping.DefaultGroup).To(Equal("Apps"))
	g.Expect(hub.Spec.Reconciliation.Interval.Duration).To(Equal(90 * time.Second))
	g.Expect(hub.Spec.Reconciliation.DisableDNSCheck).To(BeTrue())
	// Migration annotation is internal and must not leak back to v1alpha2 storage
	g.Expect(hub.Annotations).NotTo(HaveKey("sreportal.io/v1alpha2-spec"))
}

func TestDNSConvertTo_DoesNotMutateSource(t *testing.T) {
	g := NewWithT(t)
	src := &v1alpha1.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"existing-key": "existing-value"},
		},
		Spec: v1alpha1.DNSSpec{
			PortalRef: tPortalMain,
			Groups:    []v1alpha1.DNSGroup{{Name: tGroupAPIs}},
		},
	}
	originalAnnotations := map[string]string{"existing-key": "existing-value"}
	dst := &v1alpha2.DNS{}
	g.Expect(src.ConvertTo(dst)).To(Succeed())
	// Source annotations must not be mutated
	g.Expect(src.Annotations).To(Equal(originalAnnotations))
	// Destination must have both keys
	g.Expect(dst.Annotations).To(HaveKey("existing-key"))
	g.Expect(dst.Annotations).To(HaveKey("sreportal.io/v1alpha1-groups"))
}
