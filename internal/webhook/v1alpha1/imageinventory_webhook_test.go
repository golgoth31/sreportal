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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

var _ = Describe("ImageInventory Webhook", func() {
	var (
		validator ImageInventoryCustomValidator
		obj       *sreportalv1alpha1.ImageInventory
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(sreportalv1alpha1.AddToScheme(scheme)).To(Succeed())
		portal := &sreportalv1alpha1.Portal{
			ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
			Spec:       sreportalv1alpha1.PortalSpec{Title: "Main Portal"},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()
		validator = ImageInventoryCustomValidator{client: fakeClient}

		obj = &sreportalv1alpha1.ImageInventory{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-inventory",
				Namespace: "default",
			},
			Spec: sreportalv1alpha1.ImageInventorySpec{
				PortalRef: "main",
			},
		}
	})

	Context("ValidateCreate", func() {
		It("should accept when portal exists", func() {
			warnings, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("should reject when portalRef is empty", func() {
			obj.Spec.PortalRef = ""
			_, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("portalRef"))
		})

		It("should reject when portal does not exist", func() {
			obj.Spec.PortalRef = "missing"
			_, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("ValidateUpdate", func() {
		It("should accept when new portalRef exists", func() {
			warnings, err := validator.ValidateUpdate(context.Background(), obj.DeepCopy(), obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("should reject when new portalRef does not exist", func() {
			newObj := obj.DeepCopy()
			newObj.Spec.PortalRef = "missing"
			_, err := validator.ValidateUpdate(context.Background(), obj, newObj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("ValidateDelete", func() {
		It("should always accept deletion", func() {
			warnings, err := validator.ValidateDelete(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})
	})
})
