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

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

var _ = Describe("DNSRecord v1alpha1 Webhook", func() {
	var validator DNSRecordValidator

	BeforeEach(func() {
		validator = DNSRecordValidator{}
	})

	Context("ValidateUpdate", func() {
		It("rejects updates on records flagged as v1alpha2-manual", func() {
			old := &sreportalv1alpha1.DNSRecord{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					annotationV1Alpha2DNSRecordSpec: `{"origin":"manual","entries":[{"fqdn":"a.example.com"}]}`,
				},
			}}
			newObj := old.DeepCopy()
			newObj.Spec.SourceType = "service"

			warnings, err := validator.ValidateUpdate(context.Background(), old, newObj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("DNSRecord backed by v1alpha2 cannot be modified via v1alpha1"))
			Expect(warnings).To(BeEmpty())
		})

		It("allows updates on records without the v1alpha2-manual annotation", func() {
			old := &sreportalv1alpha1.DNSRecord{Spec: sreportalv1alpha1.DNSRecordSpec{SourceType: "ingress", PortalRef: "main"}}
			newObj := old.DeepCopy()
			newObj.Spec.SourceType = "service"

			warnings, err := validator.ValidateUpdate(context.Background(), old, newObj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	Context("ValidateCreate and ValidateDelete", func() {
		It("permits both unconditionally", func() {
			obj := &sreportalv1alpha1.DNSRecord{}
			_, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
			_, err = validator.ValidateDelete(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
