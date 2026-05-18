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

package dnsrecords

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

var _ = Describe("DNSRecord CRD CEL validation", func() {
	const (
		ns          = "default"
		ingressType = "ingress"
	)

	BeforeEach(func() {
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		})).To(Or(Succeed(), MatchError(ContainSubstring("already exists"))))
	})

	It("rejects origin=auto without sourceType", func() {
		rec := &v1alpha2.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-auto-missing-source", Namespace: ns},
			Spec: v1alpha2.DNSRecordSpec{
				Origin:    v1alpha2.DNSRecordOriginAuto,
				PortalRef: "p",
			},
		}
		err := k8sClient.Create(ctx, rec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("auto records require sourceType"))
	})

	It("rejects origin=manual with sourceType set", func() {
		rec := &v1alpha2.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-manual-with-source", Namespace: ns},
			Spec: v1alpha2.DNSRecordSpec{
				Origin:     v1alpha2.DNSRecordOriginManual,
				PortalRef:  "p",
				SourceType: ingressType,
				Entries:    []v1alpha2.DNSRecordEntry{{FQDN: "a.example.com", RecordType: "A"}},
			},
		}
		err := k8sClient.Create(ctx, rec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("manual records"))
	})

	It("rejects mutation of immutable origin", func() {
		rec := &v1alpha2.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-immutable-origin", Namespace: ns},
			Spec: v1alpha2.DNSRecordSpec{
				Origin:     v1alpha2.DNSRecordOriginAuto,
				PortalRef:  "p",
				SourceType: ingressType,
			},
		}
		Expect(k8sClient.Create(ctx, rec)).To(Succeed())

		var fetched v1alpha2.DNSRecord
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(rec), &fetched)
		}).Should(Succeed())
		fetched.Spec.Origin = v1alpha2.DNSRecordOriginManual
		err := k8sClient.Update(ctx, &fetched)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.origin is immutable"))
	})

	It("rejects mutation of immutable portalRef", func() {
		rec := &v1alpha2.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-immutable-portalref", Namespace: ns},
			Spec: v1alpha2.DNSRecordSpec{
				Origin:     v1alpha2.DNSRecordOriginAuto,
				PortalRef:  "p1",
				SourceType: ingressType,
			},
		}
		Expect(k8sClient.Create(ctx, rec)).To(Succeed())

		var fetched v1alpha2.DNSRecord
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(rec), &fetched)
		}).Should(Succeed())
		fetched.Spec.PortalRef = "p2"
		err := k8sClient.Update(ctx, &fetched)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.portalRef is immutable"))
	})

	It("accepts valid auto record", func() {
		rec := &v1alpha2.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-valid-auto", Namespace: ns},
			Spec: v1alpha2.DNSRecordSpec{
				Origin:     v1alpha2.DNSRecordOriginAuto,
				PortalRef:  "p",
				SourceType: ingressType,
			},
		}
		Expect(k8sClient.Create(ctx, rec)).To(Succeed())
	})

	It("accepts valid manual record", func() {
		rec := &v1alpha2.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "cel-valid-manual", Namespace: ns},
			Spec: v1alpha2.DNSRecordSpec{
				Origin:    v1alpha2.DNSRecordOriginManual,
				PortalRef: "p",
				Entries:   []v1alpha2.DNSRecordEntry{{FQDN: "a.example.com", RecordType: "A"}},
			},
		}
		Expect(k8sClient.Create(ctx, rec)).To(Succeed())
	})
})
