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

package dns

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
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	dnsreadstore "github.com/golgoth31/sreportal/internal/readstore/dns"
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
			Namespace: tNsDefault,
		}

		BeforeEach(func() {
			By("creating the DNS resource with grouped entries")
			dns := &sreportalv1alpha1.DNS{}
			err := k8sClient.Get(ctx, typeNamespacedName, dns)
			if err != nil && errors.IsNotFound(err) {
				resource := &sreportalv1alpha1.DNS{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: tNsDefault,
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
			controllerReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), true)

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
			Namespace: tNsDefault,
		}

		BeforeEach(func() {
			By("creating an empty DNS resource")
			dns := &sreportalv1alpha1.DNS{}
			err := k8sClient.Get(ctx, typeNamespacedName, dns)
			if err != nil && errors.IsNotFound(err) {
				resource := &sreportalv1alpha1.DNS{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: tNsDefault,
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
			controllerReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), true)

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
			controllerReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), true)

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent",
					Namespace: tNsDefault,
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When reconciling a remote DNS resource", func() {
		const resourceName = "remote-test-portal"
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: tNsDefault,
		}

		BeforeEach(func() {
			By("creating a remote DNS resource with pre-populated status")
			dns := &sreportalv1alpha1.DNS{}
			err := k8sClient.Get(ctx, typeNamespacedName, dns)
			if err != nil && errors.IsNotFound(err) {
				resource := &sreportalv1alpha1.DNS{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: tNsDefault,
					},
					Spec: sreportalv1alpha1.DNSSpec{
						PortalRef: "test-portal",
						IsRemote:  true,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())

				// Pre-populate status as the portal controller would.
				// Wait for the cache to see the newly created resource.
				Eventually(func() error {
					return k8sClient.Get(ctx, typeNamespacedName, resource)
				}, timeout, interval).Should(Succeed())
				resource.Status.Groups = []sreportalv1alpha1.FQDNGroupStatus{
					{
						Name:   "remote-group",
						Source: "remote",
						FQDNs: []sreportalv1alpha1.FQDNStatus{
							{
								FQDN:        "remote.example.com",
								Description: "Remote FQDN",
								LastSeen:    metav1.Now(),
							},
							{
								FQDN:        "remote-resolved.example.com",
								Description: "Remote FQDN with status",
								RecordType:  "A",
								Targets:     []string{"10.0.0.1"},
								SyncStatus:  "sync",
								LastSeen:    metav1.Now(),
							},
						},
					},
				}
				Expect(k8sClient.Status().Update(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &sreportalv1alpha1.DNS{}
			if err := k8sClient.Get(ctx, typeNamespacedName, resource); err == nil {
				By("Cleanup the DNS resource")
				_ = k8sClient.Delete(ctx, resource)
			}
		})

		It("should skip reconciliation and preserve status", func() {
			controllerReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), true)

			By("Waiting for status to be visible in cache")
			Eventually(func(g Gomega) {
				var dns sreportalv1alpha1.DNS
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &dns)).To(Succeed())
				g.Expect(dns.Status.Groups).To(HaveLen(1), "status should be populated")
			}, timeout, interval).Should(Succeed())

			By("Reconciling and verifying status is NOT overwritten")
			Eventually(func(g Gomega) {
				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result.RequeueAfter).To(BeZero(), "remote DNS should not be requeued by DNS controller")

				var dns sreportalv1alpha1.DNS
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &dns)).To(Succeed())
				g.Expect(dns.Status.Groups).To(HaveLen(1), "status should be preserved")
				g.Expect(dns.Status.Groups[0].FQDNs).To(HaveLen(2), "FQDNs should not be wiped")
				g.Expect(dns.Status.Groups[0].FQDNs[0].FQDN).To(Equal("remote.example.com"))
			}, timeout, interval).Should(Succeed())
		})

		It("should not project into the FQDN read store", func() {
			store := dnsreadstore.NewFQDNStore(nil)
			controllerReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), true)
			controllerReconciler.SetFQDNWriter(store)

			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				g.Expect(err).NotTo(HaveOccurred())

				views, err := store.List(ctx, domaindns.FQDNFilters{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(BeEmpty(), "DNS controller should not project remote FQDNs")
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When DNS CR name differs from PortalRef", func() {
		const dnsName = "test-dns-custom-name"
		ctx := context.Background()

		dnsNN := types.NamespacedName{Name: dnsName, Namespace: tNsDefault}

		AfterEach(func() {
			dns := &sreportalv1alpha1.DNS{}
			if err := k8sClient.Get(ctx, dnsNN, dns); err == nil {
				Expect(k8sClient.Delete(ctx, dns)).To(Succeed())
			}
		})

		It("should set FQDNView.PortalName from spec.portalRef, not resource name", func() {
			store := dnsreadstore.NewFQDNStore(nil)
			reconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), true)
			reconciler.SetFQDNWriter(store)

			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.DNS{
				ObjectMeta: metav1.ObjectMeta{Name: dnsName, Namespace: tNsDefault},
				Spec: sreportalv1alpha1.DNSSpec{
					PortalRef: "actual-portal",
					Groups: []sreportalv1alpha1.DNSGroup{
						{
							Name:    "Test",
							Entries: []sreportalv1alpha1.DNSEntry{{FQDN: "test.example.com"}},
						},
					},
				},
			})).To(Succeed())

			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: dnsNN})
				g.Expect(err).NotTo(HaveOccurred())

				// Should be findable by the actual portal name, not the DNS CR name
				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: "actual-portal"})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(HaveLen(1))
				g.Expect(views[0].PortalName).To(Equal("actual-portal"))

				// Should NOT appear under the DNS CR name
				views, err = store.List(ctx, domaindns.FQDNFilters{Portal: dnsName})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})
	})
})
