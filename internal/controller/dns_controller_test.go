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

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
)

var _ = Describe("DNS Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When reconciling a resource with grouped entries", func() {
		const resourceName = "test-dns-groups"
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the DNS resource with grouped entries")
			dns := &sreportalv1alpha1.DNS{}
			err := k8sClient.Get(ctx, typeNamespacedName, dns)
			if err != nil && errors.IsNotFound(err) {
				resource := &sreportalv1alpha1.DNS{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: sreportalv1alpha1.DNSSpec{
						PortalRef: "main",
						Groups: []sreportalv1alpha1.DNSGroup{
							{
								Name:        "APIs",
								Description: "Backend API services",
								Entries: []sreportalv1alpha1.DNSEntry{
									{
										FQDN:        "api.example.com",
										Description: "Main API endpoint",
									},
									{
										FQDN:        "graphql.example.com",
										Description: "GraphQL API",
									},
								},
							},
							{
								Name:        "Applications",
								Description: "Web applications",
								Entries: []sreportalv1alpha1.DNSEntry{
									{
										FQDN:        "app.example.com",
										Description: "Web application",
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &sreportalv1alpha1.DNS{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup the DNS resource")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile and populate status with groups", func() {
			controllerReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), config.DefaultConfig())

			By("Reconciling and checking the DNS status contains the groups")
			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				g.Expect(err).NotTo(HaveOccurred())

				var dns sreportalv1alpha1.DNS
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &dns)).To(Succeed())
				g.Expect(dns.Status.Groups).To(HaveLen(2))

				// Verify group names are present
				groupNames := make([]string, len(dns.Status.Groups))
				for i, group := range dns.Status.Groups {
					groupNames[i] = group.Name
				}
				g.Expect(groupNames).To(ContainElements("APIs", "Applications"))

				// Verify FQDNs within groups
				for _, group := range dns.Status.Groups {
					if group.Name == "APIs" {
						g.Expect(group.FQDNs).To(HaveLen(2))
						g.Expect(group.Source).To(Equal("manual"))
					}
					if group.Name == "Applications" {
						g.Expect(group.FQDNs).To(HaveLen(1))
						g.Expect(group.Source).To(Equal("manual"))
					}
				}

				// Verify Ready condition
				g.Expect(dns.Status.Conditions).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When reconciling an empty DNS resource", func() {
		const resourceName = "test-dns-empty"
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating an empty DNS resource")
			dns := &sreportalv1alpha1.DNS{}
			err := k8sClient.Get(ctx, typeNamespacedName, dns)
			if err != nil && errors.IsNotFound(err) {
				resource := &sreportalv1alpha1.DNS{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: sreportalv1alpha1.DNSSpec{
						PortalRef: "main",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &sreportalv1alpha1.DNS{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup the DNS resource")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile with empty status", func() {
			controllerReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), config.DefaultConfig())

			By("Reconciling and checking the DNS status is empty but has conditions")
			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				g.Expect(err).NotTo(HaveOccurred())

				var dns sreportalv1alpha1.DNS
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &dns)).To(Succeed())
				g.Expect(dns.Status.Groups).To(BeEmpty())
				g.Expect(dns.Status.LastReconcileTime).NotTo(BeNil())
				g.Expect(dns.Status.Conditions).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When the DNS resource does not exist", func() {
		It("should not return an error", func() {
			ctx := context.Background()
			controllerReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), config.DefaultConfig())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
