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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	dnsreadstore "github.com/golgoth31/sreportal/internal/readstore/dns"
)

// ensureDNS creates a v1alpha2 DNS CR named after the portal if missing.
// disableDNSCheck mirrors the value the chain should observe per reconciliation.
// Update retries on conflict to tolerate concurrent cache writes.
func ensureDNS(ctx context.Context, portal string, disableDNSCheck bool) {
	nn := types.NamespacedName{Name: portal, Namespace: tNsDefault}
	var existing v1alpha2.DNS
	err := k8sClient.Get(ctx, nn, &existing)
	if errors.IsNotFound(err) {
		Expect(k8sClient.Create(ctx, &v1alpha2.DNS{
			ObjectMeta: metav1.ObjectMeta{Name: portal, Namespace: tNsDefault},
			Spec: v1alpha2.DNSSpec{
				PortalRef: portal,
				GroupMapping: v1alpha2.GroupMappingSpec{
					DefaultGroup: "Services",
				},
				Reconciliation: v1alpha2.ReconciliationSpec{
					Interval:        metav1.Duration{Duration: 5 * time.Minute},
					RetryOnError:    metav1.Duration{Duration: 30 * time.Second},
					DisableDNSCheck: disableDNSCheck,
				},
			},
		})).To(Succeed())
		return
	}
	Expect(err).NotTo(HaveOccurred())
	if existing.Spec.Reconciliation.DisableDNSCheck == disableDNSCheck {
		return
	}
	Eventually(func(g Gomega) {
		var d v1alpha2.DNS
		g.Expect(k8sClient.Get(ctx, nn, &d)).To(Succeed())
		d.Spec.Reconciliation.DisableDNSCheck = disableDNSCheck
		g.Expect(k8sClient.Update(ctx, &d)).To(Succeed())
	}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
}

var _ = Describe("DNSRecord Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	// Portals + DNS CRs referenced by DNSRecord CRs in this suite.
	BeforeEach(func() {
		ctx := context.Background()
		for _, name := range []string{tPortalMain, tPortalMy} {
			nn := types.NamespacedName{Name: name, Namespace: tNsDefault}
			p := &sreportalv1alpha1.Portal{}
			if err := k8sClient.Get(ctx, nn, p); err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &sreportalv1alpha1.Portal{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: tNsDefault},
					Spec:       sreportalv1alpha1.PortalSpec{Title: name},
				})).To(Succeed())
			}
			// By default DNS check disabled; specific Contexts override via ensureDNS.
			ensureDNS(ctx, name, true)
		}
	})

	Context("When reconciling a DNSRecord with endpoints", func() {
		const recordName = "test-dnsrecord-projection"
		ctx := context.Background()

		recordNN := types.NamespacedName{Name: recordName, Namespace: tNsDefault}

		BeforeEach(func() {
			rec := &v1alpha2.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &v1alpha2.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: tNsDefault},
					Spec: v1alpha2.DNSRecordSpec{
						Origin:     v1alpha2.DNSRecordOriginAuto,
						SourceType: tSrcService,
						PortalRef:  tPortalMy,
						Entries: []v1alpha2.DNSRecordEntry{
							{FQDN: "api.example.com", RecordType: "A", Targets: []string{tIP1234}},
							{FQDN: "web.example.com", RecordType: "CNAME", Targets: []string{"lb.example.com"}},
						},
					},
				})).To(Succeed())
			}
		})

		AfterEach(func() {
			rec := &v1alpha2.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); err == nil {
				Expect(k8sClient.Delete(ctx, rec)).To(Succeed())
			}
		})

		It("should project endpoints into the FQDN read store with correct PortalRef", func() {
			store := dnsreadstore.NewFQDNStore()
			reconciler := NewDNSRecordReconciler(k8sClient, k8sClient.Scheme())
			reconciler.SetFQDNWriter(store)

			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: tPortalMy})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(HaveLen(2))

				for _, v := range views {
					g.Expect(v.FirstPortal()).To(Equal(tPortalMy))
					g.Expect(v.Namespace).To(Equal(tNsDefault))
					g.Expect(v.Source).To(Equal(domaindns.SourceExternalDNS))
				}
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When a DNSRecord is deleted", func() {
		const recordName = "test-dnsrecord-delete"
		ctx := context.Background()

		recordNN := types.NamespacedName{Name: recordName, Namespace: tNsDefault}

		It("should remove entries from the read store", func() {
			store := dnsreadstore.NewFQDNStore()
			reconciler := NewDNSRecordReconciler(k8sClient, k8sClient.Scheme())
			reconciler.SetFQDNWriter(store)

			By("creating and populating the DNSRecord")
			Expect(k8sClient.Create(ctx, &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: tNsDefault},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:     v1alpha2.DNSRecordOriginAuto,
					SourceType: tSrcService,
					PortalRef:  tPortalMain,
					Entries: []v1alpha2.DNSRecordEntry{
						{FQDN: "delete-me.example.com", RecordType: "A", Targets: []string{tIP1234}},
					},
				},
			})).To(Succeed())

			By("reconciling to populate the store")
			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: tPortalMain})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(HaveLen(1))
			}, timeout, interval).Should(Succeed())

			By("deleting the DNSRecord")
			rec := &v1alpha2.DNSRecord{}
			Expect(k8sClient.Get(ctx, recordNN, rec)).To(Succeed())
			Expect(k8sClient.Delete(ctx, rec)).To(Succeed())

			By("reconciling after deletion — store should be empty")
			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: tPortalMain})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When a DNSRecord has a stale or missing EndpointsHash", func() {
		const recordName = "test-dnsrecord-hash-resync"
		ctx := context.Background()

		recordNN := types.NamespacedName{Name: recordName, Namespace: tNsDefault}

		AfterEach(func() {
			rec := &v1alpha2.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); err == nil {
				Expect(k8sClient.Delete(ctx, rec)).To(Succeed())
			}
		})

		It("should correctly project spec entries into the store even when EndpointsHash is missing", func() {
			store := dnsreadstore.NewFQDNStore()
			reconciler := NewDNSRecordReconciler(k8sClient, k8sClient.Scheme())
			reconciler.SetFQDNWriter(store)

			By("creating a DNSRecord with spec entries")
			Expect(k8sClient.Create(ctx, &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: tNsDefault},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:     v1alpha2.DNSRecordOriginAuto,
					SourceType: tSrcService,
					PortalRef:  tPortalMain,
					Entries: []v1alpha2.DNSRecordEntry{
						{FQDN: "hash.example.com", RecordType: "A", Targets: []string{tIP1234}},
					},
				},
			})).To(Succeed())

			Eventually(func(g Gomega) {
				var r v1alpha2.DNSRecord
				g.Expect(k8sClient.Get(ctx, recordNN, &r)).To(Succeed())
				r.Status.EndpointsHash = "" // clear hash to simulate missing
				g.Expect(k8sClient.Status().Update(ctx, &r)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("reconciling — spec entries are materialised and projected to the store")
			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, vErr := store.List(ctx, domaindns.FQDNFilters{Portal: tPortalMain})
				g.Expect(vErr).NotTo(HaveOccurred())
				g.Expect(views).To(HaveLen(1))
				g.Expect(views[0].Name).To(Equal("hash.example.com"))
				g.Expect(views[0].RecordType).To(Equal("A"))
				g.Expect(views[0].Targets).To(ConsistOf(tIP1234))
			}, timeout, interval).Should(Succeed())
		})

		It("should correctly project spec entries into the store even when EndpointsHash is stale", func() {
			store := dnsreadstore.NewFQDNStore()
			reconciler := NewDNSRecordReconciler(k8sClient, k8sClient.Scheme())
			reconciler.SetFQDNWriter(store)

			By("creating a DNSRecord with spec entries")
			Expect(k8sClient.Create(ctx, &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: tNsDefault},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:     v1alpha2.DNSRecordOriginAuto,
					SourceType: tSrcService,
					PortalRef:  tPortalMain,
					Entries: []v1alpha2.DNSRecordEntry{
						{FQDN: "edited.example.com", RecordType: "A", Targets: []string{"9.9.9.9"}},
					},
				},
			})).To(Succeed())

			Eventually(func(g Gomega) {
				var r v1alpha2.DNSRecord
				g.Expect(k8sClient.Get(ctx, recordNN, &r)).To(Succeed())
				r.Status.EndpointsHash = "stale-wrong-hash"
				g.Expect(k8sClient.Status().Update(ctx, &r)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("reconciling — spec entries are materialised and projected to the store regardless of stale hash")
			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, vErr := store.List(ctx, domaindns.FQDNFilters{Portal: tPortalMain})
				g.Expect(vErr).NotTo(HaveOccurred())
				g.Expect(views).To(HaveLen(1))
				g.Expect(views[0].Name).To(Equal("edited.example.com"))
				g.Expect(views[0].Targets).To(ConsistOf("9.9.9.9"))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When the referenced Portal does not exist", func() {
		const (
			orphanRecord = "rec-orphan"
			orphanDNS    = "dns-orphan"
			orphanPortal = "missing-portal"
		)
		ctx := context.Background()

		recordNN := types.NamespacedName{Name: orphanRecord, Namespace: tNsDefault}
		dnsNN := types.NamespacedName{Name: orphanDNS, Namespace: tNsDefault}

		AfterEach(func() {
			rec := &v1alpha2.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); err == nil {
				Expect(k8sClient.Delete(ctx, rec)).To(Succeed())
			}
			dns := &v1alpha2.DNS{}
			if err := k8sClient.Get(ctx, dnsNN, dns); err == nil {
				Expect(k8sClient.Delete(ctx, dns)).To(Succeed())
			}
		})

		It("fast-outs and cleans the read store when the referenced Portal does not exist", func() {
			store := dnsreadstore.NewFQDNStore()
			reconciler := NewDNSRecordReconciler(k8sClient, k8sClient.Scheme())
			reconciler.SetFQDNWriter(store)

			By("creating an owner DNS that references a portal which doesn't exist")
			dns := &v1alpha2.DNS{
				ObjectMeta: metav1.ObjectMeta{Name: orphanDNS, Namespace: tNsDefault},
				Spec: v1alpha2.DNSSpec{
					PortalRef: orphanPortal,
					GroupMapping: v1alpha2.GroupMappingSpec{
						DefaultGroup: "Services",
					},
					Reconciliation: v1alpha2.ReconciliationSpec{
						Interval:        metav1.Duration{Duration: 5 * time.Minute},
						RetryOnError:    metav1.Duration{Duration: 30 * time.Second},
						DisableDNSCheck: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, dns)).To(Succeed())
			// Re-fetch to capture UID and ResourceVersion before referencing it.
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, dnsNN, dns)).To(Succeed())
				g.Expect(dns.UID).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())

			isController := true
			blockOwner := true
			rec := &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      orphanRecord,
					Namespace: tNsDefault,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         v1alpha2.GroupVersion.String(),
						Kind:               "DNS",
						Name:               orphanDNS,
						UID:                dns.UID,
						Controller:         &isController,
						BlockOwnerDeletion: &blockOwner,
					}},
				},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:    v1alpha2.DNSRecordOriginManual,
					PortalRef: orphanPortal,
					Entries: []v1alpha2.DNSRecordEntry{
						{FQDN: "foo.example.com", RecordType: "A", Targets: []string{tIP1234}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rec)).To(Succeed())

			By("pre-populating the read store with an entry attributed to this DNSRecord")
			resourceKey := tNsDefault + "/" + orphanRecord
			Expect(store.Replace(ctx, resourceKey, orphanPortal, []domaindns.FQDNView{
				{
					Name: "foo.example.com", RecordType: "A", Namespace: tNsDefault,
					Portals: []string{orphanPortal}, Source: domaindns.SourceExternalDNS,
				},
			})).To(Succeed())

			By("reconciling — controller should detect missing Portal and clean the store")
			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())

				_, gErr := store.Get(ctx, "foo.example.com", "A")
				g.Expect(gErr).To(MatchError(domaindns.ErrFQDNNotFound))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When a DNSRecord has empty endpoints", func() {
		const recordName = "test-dnsrecord-empty"
		ctx := context.Background()

		recordNN := types.NamespacedName{Name: recordName, Namespace: tNsDefault}

		AfterEach(func() {
			rec := &v1alpha2.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); err == nil {
				Expect(k8sClient.Delete(ctx, rec)).To(Succeed())
			}
		})

		It("should produce no entries in the read store", func() {
			store := dnsreadstore.NewFQDNStore()
			reconciler := NewDNSRecordReconciler(k8sClient, k8sClient.Scheme())
			reconciler.SetFQDNWriter(store)

			Expect(k8sClient.Create(ctx, &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: tNsDefault},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:     v1alpha2.DNSRecordOriginAuto,
					SourceType: tSrcService,
					PortalRef:  tPortalMain,
				},
			})).To(Succeed())

			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())

				views, err := store.List(ctx, domaindns.FQDNFilters{Portal: tPortalMain})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(views).To(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("GenerationChangedPredicate and RequeueAfter behaviour", func() {
		const recordName = "test-dnsrecord-predicate"
		ctx := context.Background()

		recordNN := types.NamespacedName{Name: recordName, Namespace: tNsDefault}

		AfterEach(func() {
			rec := &v1alpha2.DNSRecord{}
			if err := k8sClient.Get(ctx, recordNN, rec); err == nil {
				Expect(k8sClient.Delete(ctx, rec)).To(Succeed())
			}
		})

		It("does not reconcile when only status is patched (same generation)", func() {
			// Build two snapshots: same Generation, different ResourceVersion / Status.
			// GenerationChangedPredicate must return false for a status-only patch.
			gen := int64(3)
			oldObj := &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:            recordName,
					Namespace:       tNsDefault,
					Generation:      gen,
					ResourceVersion: "100",
				},
			}
			newObj := &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:            recordName,
					Namespace:       tNsDefault,
					Generation:      gen, // same generation → status-only change
					ResourceVersion: "101",
				},
				Status: v1alpha2.DNSRecordStatus{
					EndpointsHash: "some-new-hash",
				},
			}

			p := predicate.GenerationChangedPredicate{}
			Expect(p.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})).To(BeFalse(),
				"same generation should not enqueue (status-only patch)")

			// Conversely: generation bump must enqueue.
			newObjSpecChange := newObj.DeepCopy()
			newObjSpecChange.Generation = gen + 1
			Expect(p.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObjSpecChange})).To(BeTrue(),
				"generation bump should enqueue (spec change)")
		})

		It("requeues after DNSRecordResolveInterval on successful reconcile", func() {
			store := dnsreadstore.NewFQDNStore()
			rec := NewDNSRecordReconciler(k8sClient, k8sClient.Scheme())
			rec.SetFQDNWriter(store)

			Expect(k8sClient.Create(ctx, &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: recordName, Namespace: tNsDefault},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:     v1alpha2.DNSRecordOriginAuto,
					SourceType: tSrcService,
					PortalRef:  tPortalMain,
				},
			})).To(Succeed())

			Eventually(func(g Gomega) {
				result, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result.RequeueAfter).To(Equal(DNSRecordResolveInterval),
					"reconcile must schedule a periodic drift-check requeue")
			}, timeout, interval).Should(Succeed())
		})
	})
})
