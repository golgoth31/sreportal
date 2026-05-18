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

package v1alpha2_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	webhookv1alpha2 "github.com/golgoth31/sreportal/internal/webhook/v1alpha2"
)

const (
	tPortalMain    = "main"
	tPortalOther   = "other"
	tRecordIngress = "main-ingress"
	tRecordManual  = "main-manual-apis"
	tSourceIngress = "ingress"
	tFQDNAPIExamp  = "api.example.com"
)

// TestDNSWebhook_NameDiffFromPortalRef asserts that a DNS CR whose name differs
// from spec.portalRef is now ACCEPTED — N DNS CRs per Portal is allowed.
func TestDNSWebhook_NameDiffFromPortalRef(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: tPortalOther},
	}
	_, err := v.ValidateCreate(context.Background(), dns)
	g.Expect(err).NotTo(HaveOccurred())
}

// TestDNSWebhook_CreateNameEqualsPortalRef asserts that the classic case (name == portalRef) is still valid.
func TestDNSWebhook_CreateNameEqualsPortalRef(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: tPortalMain},
	}
	_, err := v.ValidateCreate(context.Background(), dns)
	g.Expect(err).NotTo(HaveOccurred())
}

// TestDNSWebhook_CreateMultiDNSPerPortal asserts that a DNS CR with an arbitrary name
// pointing to a portal is accepted, enabling N DNS CRs per Portal.
func TestDNSWebhook_CreateMultiDNSPerPortal(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a-dns"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "my-portal"},
	}
	_, err := v.ValidateCreate(context.Background(), dns)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestDNSWebhook_PortalRefImmutable(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	old := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: tPortalMain},
	}
	newDNS := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: tPortalOther},
	}
	_, err := v.ValidateUpdate(context.Background(), old, newDNS)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("portalRef is immutable"))
}

func TestDNSWebhook_ValidUpdate(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	old := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: tPortalMain},
	}
	newDNS := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: tPortalMain},
	}
	_, err := v.ValidateUpdate(context.Background(), old, newDNS)
	g.Expect(err).NotTo(HaveOccurred())
}

// TestDNSWebhook_UpdateNameDiffFromPortalRef asserts that updating a DNS CR
// whose name differs from spec.portalRef is now ACCEPTED (portalRef unchanged).
func TestDNSWebhook_UpdateNameDiffFromPortalRef(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	old := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a-dns"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "my-portal"},
	}
	newDNS := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a-dns"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "my-portal"},
	}
	_, err := v.ValidateUpdate(context.Background(), old, newDNS)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestDNSWebhook_DeleteNoOp(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: tPortalMain},
	}
	_, err := v.ValidateDelete(context.Background(), dns)
	g.Expect(err).NotTo(HaveOccurred())
}

// --- labelFilter validation ---

// TestDNSWebhook_DefaultsLabelFilterInvalid asserts that an invalid label selector
// in spec.defaults.labelFilter is rejected and error mentions the field path.
func TestDNSWebhook_DefaultsLabelFilterInvalid(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: tPortalMain,
			Defaults: sreportalv1alpha2.SourceFilterDefaults{
				LabelFilter: "not a valid selector!!",
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), dns)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.defaults.labelFilter"))
}

// TestDNSWebhook_DefaultsLabelFilterValid asserts that a well-formed label selector
// in spec.defaults.labelFilter is accepted.
func TestDNSWebhook_DefaultsLabelFilterValid(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: tPortalMain,
			Defaults: sreportalv1alpha2.SourceFilterDefaults{
				LabelFilter: "app=foo,tier!=db",
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), dns)
	g.Expect(err).NotTo(HaveOccurred())
}

// TestDNSWebhook_SourceServiceLabelFilterInvalid asserts that an invalid label selector
// in spec.sources.service.labelFilter is rejected and error mentions the field path.
func TestDNSWebhook_SourceServiceLabelFilterInvalid(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: tPortalMain,
			Sources: sreportalv1alpha2.SourcesSpec{
				Service: &sreportalv1alpha2.ServiceSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{
						Enabled:     true,
						LabelFilter: "app=foo,!=",
					},
				},
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), dns)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.sources.service.labelFilter"))
}

// --- priority validation ---

// TestDNSWebhook_PriorityRefersNotEnabledSource asserts that a priority entry
// referencing a source that is defined but NOT enabled is rejected.
func TestDNSWebhook_PriorityRefersNotEnabledSource(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: tPortalMain,
			Sources: sreportalv1alpha2.SourcesSpec{
				// service enabled; ingress pointer is nil — not present at all
				Service: &sreportalv1alpha2.ServiceSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
				},
				Priority: []sreportalv1alpha2.SourceType{
					sreportalv1alpha2.SourceTypeService,
					sreportalv1alpha2.SourceTypeIngress, // not enabled
				},
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), dns)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not an enabled source"))
}

// TestDNSWebhook_PriorityIngressNotEnabled asserts that priority = [ingress] is rejected
// when spec.sources.ingress.enabled = false.
func TestDNSWebhook_PriorityIngressNotEnabled(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: tPortalMain,
			Sources: sreportalv1alpha2.SourcesSpec{
				Ingress: &sreportalv1alpha2.IngressSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: false},
				},
				Priority: []sreportalv1alpha2.SourceType{
					sreportalv1alpha2.SourceTypeIngress,
				},
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), dns)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not an enabled source"))
}

// TestDNSWebhook_PriorityValidSubset asserts that a priority list that only contains
// source types that are actually enabled is accepted.
func TestDNSWebhook_PriorityValidSubset(t *testing.T) {
	g := NewWithT(t)
	v := webhookv1alpha2.NewDNSCustomValidator()
	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: tPortalMain,
			Sources: sreportalv1alpha2.SourcesSpec{
				Service: &sreportalv1alpha2.ServiceSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
				},
				Priority: []sreportalv1alpha2.SourceType{
					sreportalv1alpha2.SourceTypeService,
				},
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), dns)
	g.Expect(err).NotTo(HaveOccurred())
}
