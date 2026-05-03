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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

const testDisallowedType = "hotfix"

var _ = Describe("Release Webhook", func() {
	var (
		k8sClient client.Client
		obj       *sreportalv1alpha1.Release
		oldObj    *sreportalv1alpha1.Release
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(sreportalv1alpha1.AddToScheme(scheme)).To(Succeed())
		portal := &sreportalv1alpha1.Portal{
			ObjectMeta: metav1.ObjectMeta{Name: tPortalMain, Namespace: tNsDefault},
			Spec:       sreportalv1alpha1.PortalSpec{Title: "Main Portal"},
		}
		k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()

		obj = &sreportalv1alpha1.Release{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "release-2026-03-22",
				Namespace: tNsDefault,
			},
			Spec: sreportalv1alpha1.ReleaseSpec{
				PortalRef: tPortalMain,
				Entries: []sreportalv1alpha1.ReleaseEntry{
					{
						Type:    tKindDeployment,
						Version: "v1.0.0",
						Origin:  "ci/cd",
						Date:    metav1.Now(),
					},
				},
			},
		}
		oldObj = obj.DeepCopy()
	})

	Context("portal reference validation", func() {
		It("Should reject creation when portalRef is missing", func() {
			obj.Spec.PortalRef = ""
			v := ReleaseCustomValidator{client: k8sClient, allowedTypes: nil}
			_, err := v.ValidateCreate(context.Background(), obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("portalRef"))
		})

		It("Should reject creation when portal does not exist", func() {
			obj.Spec.PortalRef = "missing-portal"
			v := ReleaseCustomValidator{client: k8sClient, allowedTypes: nil}
			_, err := v.ValidateCreate(context.Background(), obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("Should accept creation when portal exists", func() {
			v := ReleaseCustomValidator{client: k8sClient, allowedTypes: nil}
			warnings, err := v.ValidateCreate(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})
	})

	Context("When allowedTypes is empty (no restriction)", func() {
		var validator ReleaseCustomValidator

		BeforeEach(func() {
			validator = ReleaseCustomValidator{client: k8sClient, allowedTypes: nil}
		})

		It("Should allow creation with any type", func() {
			warnings, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should allow update with any type", func() {
			obj.Spec.Entries[0].Type = "anything"
			warnings, err := validator.ValidateUpdate(context.Background(), oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})
	})

	Context("When allowedTypes is configured", func() {
		var validator ReleaseCustomValidator

		BeforeEach(func() {
			validator = ReleaseCustomValidator{client: k8sClient, allowedTypes: []string{tKindDeployment, "rollback"}}
		})

		It("Should allow creation with allowed type", func() {
			obj.Spec.Entries[0].Type = tKindDeployment
			warnings, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should deny creation with disallowed type", func() {
			obj.Spec.Entries[0].Type = testDisallowedType
			_, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not allowed"))
			Expect(err.Error()).To(ContainSubstring(testDisallowedType))
		})

		It("Should deny when any entry has disallowed type", func() {
			obj.Spec.Entries = []sreportalv1alpha1.ReleaseEntry{
				{Type: tKindDeployment, Version: "v1.0.0", Origin: "ci", Date: metav1.Now()},
				{Type: testDisallowedType, Version: "v1.0.1", Origin: "manual", Date: metav1.Now()},
			}
			_, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.entries[1]"))
			Expect(err.Error()).To(ContainSubstring(testDisallowedType))
		})

		It("Should deny update with disallowed type", func() {
			obj.Spec.Entries[0].Type = testDisallowedType
			_, err := validator.ValidateUpdate(context.Background(), oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not allowed"))
		})

		It("Should allow deletion regardless of types", func() {
			obj.Spec.Entries[0].Type = testDisallowedType
			warnings, err := validator.ValidateDelete(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should allow creation with empty entries", func() {
			obj.Spec.Entries = nil
			warnings, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})
	})
})
