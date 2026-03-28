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
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	dnsreadstore "github.com/golgoth31/sreportal/internal/readstore/dns"
	releasereadstore "github.com/golgoth31/sreportal/internal/readstore/release"
	"github.com/golgoth31/sreportal/internal/remoteclient"
)

var _ = Describe("Portal Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		portal := &sreportalv1alpha1.Portal{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Portal")
			err := k8sClient.Get(ctx, typeNamespacedName, portal)
			if err != nil && errors.IsNotFound(err) {
				resource := &sreportalv1alpha1.Portal{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: sreportalv1alpha1.PortalSpec{
						Title: "Test Portal",
						Main:  true,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &sreportalv1alpha1.Portal{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup the specific resource instance Portal")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PortalReconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				remoteClientCache: remoteclient.NewCache(),
			}

			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				g.Expect(err).NotTo(HaveOccurred())
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
		})
	})

	Context("When DNS feature is disabled on a portal", func() {
		const (
			portalName = "portal-dns-toggle"
			dnsName    = "dns-for-toggle"
			recordName = "portal-dns-toggle-service"
		)
		ctx := context.Background()

		portalNN := types.NamespacedName{Name: portalName, Namespace: "default"}
		dnsNN := types.NamespacedName{Name: dnsName, Namespace: "default"}
		recordNN := types.NamespacedName{Name: recordName, Namespace: "default"}

		AfterEach(func() {
			rec := &sreportalv1alpha1.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); err == nil {
				_ = k8sClient.Delete(ctx, rec)
			}
			dns := &sreportalv1alpha1.DNS{}
			if err := k8sClient.Get(ctx, dnsNN, dns); err == nil {
				_ = k8sClient.Delete(ctx, dns)
			}
			portal := &sreportalv1alpha1.Portal{}
			if err := k8sClient.Get(ctx, portalNN, portal); err == nil {
				_ = k8sClient.Delete(ctx, portal)
			}
		})

		It("should clear FQDN read store entries and delete DNSRecords", func() {
			store := dnsreadstore.NewFQDNStore(nil)
			controllerReconciler := NewPortalReconciler(k8sClient, k8sClient.Scheme(), remoteclient.NewCache())
			controllerReconciler.SetFQDNWriter(store)

			By("creating a portal with DNS enabled")
			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.Portal{
				ObjectMeta: metav1.ObjectMeta{Name: portalName, Namespace: "default"},
				Spec: sreportalv1alpha1.PortalSpec{
					Title: "Toggle Portal",
				},
			})).To(Succeed())

			By("creating a DNS CR for this portal")
			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.DNS{
				ObjectMeta: metav1.ObjectMeta{Name: dnsName, Namespace: "default"},
				Spec: sreportalv1alpha1.DNSSpec{
					PortalRef: portalName,
					Groups: []sreportalv1alpha1.DNSGroup{
						{Name: "Test", Entries: []sreportalv1alpha1.DNSEntry{{FQDN: "toggle.example.com"}}},
					},
				},
			})).To(Succeed())

			By("creating a DNSRecord CR for this portal")
			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: "default"},
				Spec: sreportalv1alpha1.DNSRecordSpec{
					SourceType: "service",
					PortalRef:  portalName,
				},
			})).To(Succeed())

			By("pre-populating the FQDN read store as controllers would")
			dnsKey := "default/" + dnsName
			recordKey := "default/" + recordName
			Expect(store.Replace(ctx, dnsKey, []domaindns.FQDNView{
				{Name: "toggle.example.com", PortalName: portalName, Source: domaindns.SourceManual},
			})).To(Succeed())
			Expect(store.Replace(ctx, recordKey, []domaindns.FQDNView{
				{Name: "svc.example.com", PortalName: portalName, Source: domaindns.SourceExternalDNS},
			})).To(Succeed())

			views, _ := store.List(ctx, domaindns.FQDNFilters{Portal: portalName})
			Expect(views).To(HaveLen(2), "pre-condition: store should have 2 entries")

			By("disabling DNS on the portal")
			Eventually(func(g Gomega) {
				var portal sreportalv1alpha1.Portal
				g.Expect(k8sClient.Get(ctx, portalNN, &portal)).To(Succeed())
				dnsDisabled := false
				portal.Spec.Features = &sreportalv1alpha1.PortalFeatures{DNS: &dnsDisabled}
				g.Expect(k8sClient.Update(ctx, &portal)).To(Succeed())
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

			By("reconciling the portal — read store should be cleared, DNSRecord deleted")
			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: portalNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: portalName})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(BeEmpty(), "all FQDN views for this portal should be cleared")
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

			By("verifying DNS CR still exists but DNSRecord is deleted")
			Eventually(func(g Gomega) {
				var dns sreportalv1alpha1.DNS
				g.Expect(k8sClient.Get(ctx, dnsNN, &dns)).To(Succeed(), "DNS CR should be preserved")

				var rec sreportalv1alpha1.DNSRecord
				err := k8sClient.Get(ctx, recordNN, &rec)
				g.Expect(errors.IsNotFound(err)).To(BeTrue(), "DNSRecord should be deleted")
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
		})

		It("should recover data when DNS is re-enabled", func() {
			store := dnsreadstore.NewFQDNStore(nil)
			portalReconciler := NewPortalReconciler(k8sClient, k8sClient.Scheme(), remoteclient.NewCache())
			portalReconciler.SetFQDNWriter(store)
			dnsReconciler := NewDNSReconciler(k8sClient, k8sClient.Scheme(), true)
			dnsReconciler.SetFQDNWriter(store)

			By("creating a portal with DNS disabled")
			dnsDisabled := false
			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.Portal{
				ObjectMeta: metav1.ObjectMeta{Name: portalName, Namespace: "default"},
				Spec: sreportalv1alpha1.PortalSpec{
					Title:    "Toggle Portal",
					Features: &sreportalv1alpha1.PortalFeatures{DNS: &dnsDisabled},
				},
			})).To(Succeed())

			By("creating a DNS CR")
			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.DNS{
				ObjectMeta: metav1.ObjectMeta{Name: dnsName, Namespace: "default"},
				Spec: sreportalv1alpha1.DNSSpec{
					PortalRef: portalName,
					Groups: []sreportalv1alpha1.DNSGroup{
						{Name: "Test", Entries: []sreportalv1alpha1.DNSEntry{{FQDN: "recover.example.com"}}},
					},
				},
			})).To(Succeed())

			By("reconciling portal with DNS disabled — store should be empty")
			Eventually(func(g Gomega) {
				_, err := portalReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: portalNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: portalName})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(BeEmpty())
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

			By("re-enabling DNS on the portal")
			Eventually(func(g Gomega) {
				var portal sreportalv1alpha1.Portal
				g.Expect(k8sClient.Get(ctx, portalNN, &portal)).To(Succeed())
				dnsEnabled := true
				portal.Spec.Features.DNS = &dnsEnabled
				g.Expect(k8sClient.Update(ctx, &portal)).To(Succeed())
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

			By("reconciling the DNS controller — data should be recovered")
			Eventually(func(g Gomega) {
				_, err := dnsReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: dnsNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: portalName})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(HaveLen(1), "FQDN should be recovered after re-enabling DNS")
				g.Expect(views[0].Name).To(Equal("recover.example.com"))
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
		})
	})

	Context("When releases feature is disabled on a portal", func() {
		const (
			portalRelName = "portal-release-toggle"
			releaseCRName = "release-2026-03-21"
		)
		ctx := context.Background()
		portalRelNN := types.NamespacedName{Name: portalRelName, Namespace: "default"}
		releaseNN := types.NamespacedName{Name: releaseCRName, Namespace: "default"}

		AfterEach(func() {
			rel := &sreportalv1alpha1.Release{}
			if err := k8sClient.Get(ctx, releaseNN, rel); err == nil {
				_ = k8sClient.Delete(ctx, rel)
			}
			p := &sreportalv1alpha1.Portal{}
			if err := k8sClient.Get(ctx, portalRelNN, p); err == nil {
				_ = k8sClient.Delete(ctx, p)
			}
		})

		It("should flush read store entries and not delete Release CRs", func() {
			store := releasereadstore.NewReleaseStore()
			portalReconciler := NewPortalReconciler(k8sClient, k8sClient.Scheme(), remoteclient.NewCache())
			portalReconciler.SetReleaseWriter(store)

			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.Portal{
				ObjectMeta: metav1.ObjectMeta{Name: portalRelName, Namespace: "default"},
				Spec:       sreportalv1alpha1.PortalSpec{Title: "Release Toggle Portal"},
			})).To(Succeed())

			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{Name: releaseCRName, Namespace: "default"},
				Spec: sreportalv1alpha1.ReleaseSpec{
					PortalRef: portalRelName,
					Entries: []sreportalv1alpha1.ReleaseEntry{
						{Type: "deployment", Origin: "ci", Date: metav1.Now()},
					},
				},
			})).To(Succeed())

			resourceKey := "default/" + releaseCRName
			Expect(store.Replace(ctx, resourceKey, []domainrelease.EntryView{
				{PortalRef: portalRelName, Day: "2026-03-21", Type: "deployment", Origin: "ci", Date: time.Now().UTC()},
			})).To(Succeed())

			entries, err := store.ListEntries(ctx, "2026-03-21", portalRelName)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).NotTo(BeEmpty())

			releasesOff := false
			Eventually(func(g Gomega) {
				var portal sreportalv1alpha1.Portal
				g.Expect(k8sClient.Get(ctx, portalRelNN, &portal)).To(Succeed())
				portal.Spec.Features = &sreportalv1alpha1.PortalFeatures{Releases: &releasesOff}
				g.Expect(k8sClient.Update(ctx, &portal)).To(Succeed())
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

			Eventually(func(g Gomega) {
				_, err := portalReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: portalRelNN})
				g.Expect(err).NotTo(HaveOccurred())
				got, err := store.ListEntries(ctx, "2026-03-21", portalRelName)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(got).To(BeEmpty())
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

			var rel sreportalv1alpha1.Release
			Expect(k8sClient.Get(ctx, releaseNN, &rel)).To(Succeed(), "Release CR must remain when feature is disabled")
		})
	})
})
