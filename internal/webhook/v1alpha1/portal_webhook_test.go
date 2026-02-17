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

const testRemoteURL = "https://remote.example.com"

var _ = Describe("Portal Webhook", func() {
	var (
		obj       *sreportalv1alpha1.Portal
		oldObj    *sreportalv1alpha1.Portal
		validator PortalCustomValidator
		defaulter PortalCustomDefaulter
	)

	BeforeEach(func() {
		obj = &sreportalv1alpha1.Portal{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-portal",
				Namespace: "default",
			},
			Spec: sreportalv1alpha1.PortalSpec{
				Title: "Test Portal",
			},
		}
		oldObj = &sreportalv1alpha1.Portal{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-portal",
				Namespace: "default",
			},
			Spec: sreportalv1alpha1.PortalSpec{
				Title: "Test Portal",
			},
		}
		validator = PortalCustomValidator{}
		defaulter = PortalCustomDefaulter{}
	})

	Context("When creating Portal under Defaulting Webhook", func() {
		It("Should set subPath to name when not specified", func() {
			By("creating a portal without subPath")
			obj.Spec.SubPath = ""

			By("calling the Default method to apply defaults")
			err := defaulter.Default(context.Background(), obj)

			By("checking that subPath is set to the portal name")
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.SubPath).To(Equal("test-portal"))
		})

		It("Should preserve subPath when already specified", func() {
			By("creating a portal with a custom subPath")
			obj.Spec.SubPath = "custom-path"

			By("calling the Default method")
			err := defaulter.Default(context.Background(), obj)

			By("checking that subPath is preserved")
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.SubPath).To(Equal("custom-path"))
		})
	})

	Context("When creating Portal under Validating Webhook", func() {
		It("Should allow creation of a local portal (no remote)", func() {
			By("creating a portal without remote")
			obj.Spec.Remote = nil
			obj.Spec.Main = false

			By("validating the creation")
			warnings, err := validator.ValidateCreate(context.Background(), obj)

			By("checking that validation passes")
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should allow creation of a main portal without remote", func() {
			By("creating a main portal without remote")
			obj.Spec.Remote = nil
			obj.Spec.Main = true

			By("validating the creation")
			warnings, err := validator.ValidateCreate(context.Background(), obj)

			By("checking that validation passes")
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should allow creation of a remote portal (with remote, not main)", func() {
			By("creating a remote portal with remote config")
			obj.Spec.Remote = &sreportalv1alpha1.RemotePortalSpec{
				URL:    testRemoteURL,
				Portal: "main",
			}
			obj.Spec.Main = false

			By("validating the creation")
			warnings, err := validator.ValidateCreate(context.Background(), obj)

			By("checking that validation passes")
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should deny creation of a main portal with remote", func() {
			By("creating a main portal with remote (invalid)")
			obj.Spec.Remote = &sreportalv1alpha1.RemotePortalSpec{
				URL: testRemoteURL,
			}
			obj.Spec.Main = true

			By("validating the creation")
			_, err := validator.ValidateCreate(context.Background(), obj)

			By("checking that validation fails with correct error message")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.remote cannot be set when spec.main is true"))
			Expect(err.Error()).To(ContainSubstring("main portal must be local"))
		})
	})

	Context("When updating Portal under Validating Webhook", func() {
		It("Should allow update when remote is added to non-main portal", func() {
			By("setting up old portal without remote")
			oldObj.Spec.Remote = nil
			oldObj.Spec.Main = false

			By("updating portal to add remote")
			obj.Spec.Remote = &sreportalv1alpha1.RemotePortalSpec{
				URL: testRemoteURL,
			}
			obj.Spec.Main = false

			By("validating the update")
			warnings, err := validator.ValidateUpdate(context.Background(), oldObj, obj)

			By("checking that validation passes")
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should deny update when remote is added to main portal", func() {
			By("setting up old main portal without remote")
			oldObj.Spec.Remote = nil
			oldObj.Spec.Main = true

			By("updating main portal to add remote")
			obj.Spec.Remote = &sreportalv1alpha1.RemotePortalSpec{
				URL: testRemoteURL,
			}
			obj.Spec.Main = true

			By("validating the update")
			_, err := validator.ValidateUpdate(context.Background(), oldObj, obj)

			By("checking that validation fails")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.remote cannot be set when spec.main is true"))
		})

		It("Should deny update when portal with remote is changed to main", func() {
			By("setting up old remote portal with remote config")
			oldObj.Spec.Remote = &sreportalv1alpha1.RemotePortalSpec{
				URL: testRemoteURL,
			}
			oldObj.Spec.Main = false

			By("updating portal to be main (while keeping remote)")
			obj.Spec.Remote = &sreportalv1alpha1.RemotePortalSpec{
				URL: testRemoteURL,
			}
			obj.Spec.Main = true

			By("validating the update")
			_, err := validator.ValidateUpdate(context.Background(), oldObj, obj)

			By("checking that validation fails")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.remote cannot be set when spec.main is true"))
		})

		It("Should allow update when remote is removed from portal becoming main", func() {
			By("setting up old remote portal with remote config")
			oldObj.Spec.Remote = &sreportalv1alpha1.RemotePortalSpec{
				URL: testRemoteURL,
			}
			oldObj.Spec.Main = false

			By("updating portal to be main and removing remote")
			obj.Spec.Remote = nil
			obj.Spec.Main = true

			By("validating the update")
			warnings, err := validator.ValidateUpdate(context.Background(), oldObj, obj)

			By("checking that validation passes")
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})
	})

	Context("When deleting Portal under Validating Webhook", func() {
		It("Should always allow deletion", func() {
			By("deleting a portal")
			warnings, err := validator.ValidateDelete(context.Background(), obj)

			By("checking that deletion is allowed")
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should allow deletion of main portal with remote (even if invalid state)", func() {
			By("deleting a main portal with remote")
			obj.Spec.Remote = &sreportalv1alpha1.RemotePortalSpec{
				URL: testRemoteURL,
			}
			obj.Spec.Main = true

			warnings, err := validator.ValidateDelete(context.Background(), obj)

			By("checking that deletion is allowed")
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})
	})
})
