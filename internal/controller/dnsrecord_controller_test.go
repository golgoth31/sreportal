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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	dnsreadstore "github.com/golgoth31/sreportal/internal/readstore/dns"
)

// stubResolver is a fake dns.Resolver for testing.
type stubResolver struct {
	hosts map[string][]string
}

func (r *stubResolver) LookupHost(_ context.Context, fqdn string) ([]string, error) {
	addrs, ok := r.hosts[fqdn]
	if !ok {
		return nil, fmt.Errorf("no such host: %s", fqdn)
	}
	return addrs, nil
}

func (r *stubResolver) LookupCNAME(_ context.Context, fqdn string) (string, error) {
	return "", fmt.Errorf("no CNAME for: %s", fqdn)
}

var _ = Describe("DNSRecord Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When reconciling a DNSRecord with endpoints", func() {
		const recordName = "test-dnsrecord-projection"
		ctx := context.Background()

		recordNN := types.NamespacedName{Name: recordName, Namespace: "default"}

		BeforeEach(func() {
			rec := &sreportalv1alpha1.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &sreportalv1alpha1.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: "default"},
					Spec: sreportalv1alpha1.DNSRecordSpec{
						SourceType: "service",
						PortalRef:  "my-portal",
					},
				})).To(Succeed())
			}

			Eventually(func(g Gomega) {
				var r sreportalv1alpha1.DNSRecord
				g.Expect(k8sClient.Get(ctx, recordNN, &r)).To(Succeed())
				r.Status.Endpoints = []sreportalv1alpha1.EndpointStatus{
					{DNSName: "api.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}, LastSeen: metav1.Now()},
					{DNSName: "web.example.com", RecordType: "CNAME", Targets: []string{"lb.example.com"}, LastSeen: metav1.Now()},
				}
				g.Expect(k8sClient.Status().Update(ctx, &r)).To(Succeed())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			rec := &sreportalv1alpha1.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); err == nil {
				Expect(k8sClient.Delete(ctx, rec)).To(Succeed())
			}
		})

		It("should project endpoints into the FQDN read store with correct PortalRef", func() {
			store := dnsreadstore.NewFQDNStore(nil)
			reconciler := NewDNSRecordReconciler(k8sClient, k8sClient.Scheme(), nil, nil, true)
			reconciler.SetFQDNWriter(store)

			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: "my-portal"})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(HaveLen(2))

				for _, v := range views {
					g.Expect(v.PortalName).To(Equal("my-portal"))
					g.Expect(v.Namespace).To(Equal("default"))
					g.Expect(v.Source).To(Equal(domaindns.SourceExternalDNS))
				}
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When a DNSRecord is deleted", func() {
		const recordName = "test-dnsrecord-delete"
		ctx := context.Background()

		recordNN := types.NamespacedName{Name: recordName, Namespace: "default"}

		It("should remove entries from the read store", func() {
			store := dnsreadstore.NewFQDNStore(nil)
			reconciler := NewDNSRecordReconciler(k8sClient, k8sClient.Scheme(), nil, nil, true)
			reconciler.SetFQDNWriter(store)

			By("creating and populating the DNSRecord")
			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: "default"},
				Spec: sreportalv1alpha1.DNSRecordSpec{
					SourceType: "service",
					PortalRef:  "main",
				},
			})).To(Succeed())

			Eventually(func(g Gomega) {
				var r sreportalv1alpha1.DNSRecord
				g.Expect(k8sClient.Get(ctx, recordNN, &r)).To(Succeed())
				r.Status.Endpoints = []sreportalv1alpha1.EndpointStatus{
					{DNSName: "delete-me.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}, LastSeen: metav1.Now()},
				}
				g.Expect(k8sClient.Status().Update(ctx, &r)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("reconciling to populate the store")
			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: "main"})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(HaveLen(1))
			}, timeout, interval).Should(Succeed())

			By("deleting the DNSRecord")
			rec := &sreportalv1alpha1.DNSRecord{}
			Expect(k8sClient.Get(ctx, recordNN, rec)).To(Succeed())
			Expect(k8sClient.Delete(ctx, rec)).To(Succeed())

			By("reconciling after deletion — store should be empty")
			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: "main"})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When reconciling with DNS resolution enabled", func() {
		const recordName = "test-dnsrecord-resolve"
		ctx := context.Background()

		recordNN := types.NamespacedName{Name: recordName, Namespace: "default"}

		BeforeEach(func() {
			rec := &sreportalv1alpha1.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &sreportalv1alpha1.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: "default"},
					Spec: sreportalv1alpha1.DNSRecordSpec{
						SourceType: "service",
						PortalRef:  "my-portal",
					},
				})).To(Succeed())
			}

			Eventually(func(g Gomega) {
				var r sreportalv1alpha1.DNSRecord
				g.Expect(k8sClient.Get(ctx, recordNN, &r)).To(Succeed())
				r.Status.Endpoints = []sreportalv1alpha1.EndpointStatus{
					{DNSName: "resolved.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}, LastSeen: metav1.Now()},
					{DNSName: "missing.example.com", RecordType: "A", Targets: []string{"5.6.7.8"}, LastSeen: metav1.Now()},
				}
				g.Expect(k8sClient.Status().Update(ctx, &r)).To(Succeed())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			rec := &sreportalv1alpha1.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); err == nil {
				Expect(k8sClient.Delete(ctx, rec)).To(Succeed())
			}
		})

		It("should persist SyncStatus on CR and propagate to read store", func() {
			store := dnsreadstore.NewFQDNStore(nil)
			resolver := &stubResolver{
				hosts: map[string][]string{
					"resolved.example.com": {"1.2.3.4"},
					// missing.example.com has no entry → notavailable
				},
			}
			reconciler := NewDNSRecordReconciler(k8sClient, k8sClient.Scheme(), nil, resolver, false)
			reconciler.SetFQDNWriter(store)

			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())

				// Verify SyncStatus persisted on CR
				var updated sreportalv1alpha1.DNSRecord
				g.Expect(k8sClient.Get(ctx, recordNN, &updated)).To(Succeed())
				epStatusByName := make(map[string]string)
				for _, ep := range updated.Status.Endpoints {
					epStatusByName[ep.DNSName] = ep.SyncStatus
				}
				g.Expect(epStatusByName["resolved.example.com"]).To(Equal("sync"))
				g.Expect(epStatusByName["missing.example.com"]).To(Equal("notavailable"))

				// Verify SyncStatus propagated to read store
				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: "my-portal"})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(HaveLen(2))

				viewStatusByName := make(map[string]string)
				for _, v := range views {
					viewStatusByName[v.Name] = v.SyncStatus
				}
				g.Expect(viewStatusByName["resolved.example.com"]).To(Equal("sync"))
				g.Expect(viewStatusByName["missing.example.com"]).To(Equal("notavailable"))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When a DNSRecord has empty endpoints", func() {
		const recordName = "test-dnsrecord-empty"
		ctx := context.Background()

		recordNN := types.NamespacedName{Name: recordName, Namespace: "default"}

		AfterEach(func() {
			rec := &sreportalv1alpha1.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); err == nil {
				Expect(k8sClient.Delete(ctx, rec)).To(Succeed())
			}
		})

		It("should produce no entries in the read store", func() {
			store := dnsreadstore.NewFQDNStore(nil)
			reconciler := NewDNSRecordReconciler(k8sClient, k8sClient.Scheme(), nil, nil, true)
			reconciler.SetFQDNWriter(store)

			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: "default"},
				Spec: sreportalv1alpha1.DNSRecordSpec{
					SourceType: "service",
					PortalRef:  "main",
				},
			})).To(Succeed())

			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: "main"})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})
	})
})
