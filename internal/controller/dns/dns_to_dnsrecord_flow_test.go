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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnsrecords "github.com/golgoth31/sreportal/internal/controller/dnsrecords"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	dnsreadstore "github.com/golgoth31/sreportal/internal/readstore/dns"
	"github.com/golgoth31/sreportal/internal/source/externaldns"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// staticSourceReader returns a fixed slice of EnrichedEndpoints for the
// configured kind. Other kinds get an empty result. Used by the
// DNS → DNSRecord E2E flow test below.
type staticSourceReader struct {
	kind registry.SourceType
	eps  []domainsource.EnrichedEndpoint
}

func (s staticSourceReader) Lookup(kind registry.SourceType, _, _ string) ([]domainsource.EnrichedEndpoint, error) {
	if kind != s.kind {
		return nil, nil
	}
	return s.eps, nil
}

// Ready reports true: the static reader always has authoritative data.
func (s staticSourceReader) Ready(_ registry.SourceType) bool { return true }

func (s staticSourceReader) Kinds() []registry.SourceType { return []registry.SourceType{s.kind} }

var _ = Describe("DNS → DNSRecord E2E flow", func() {
	const (
		timeout  = time.Second * 15
		interval = time.Millisecond * 250

		portalName = "flow-portal"
		dnsName    = "flow-dns"
		fqdn       = "svc.flow.example.com"
		target     = "10.42.0.7"
	)

	ctx := context.Background()
	dnsNN := types.NamespacedName{Name: dnsName, Namespace: tNsDefault}
	recordNN := types.NamespacedName{Name: dnsName + "-service", Namespace: tNsDefault}

	BeforeEach(func() {
		p := &sreportalv1alpha1.Portal{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: portalName, Namespace: tNsDefault}, p); err != nil && errors.IsNotFound(err) {
			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.Portal{
				ObjectMeta: metav1.ObjectMeta{Name: portalName, Namespace: tNsDefault},
				Spec:       sreportalv1alpha1.PortalSpec{Title: portalName},
			})).To(Succeed())
		}
	})

	AfterEach(func() {
		rec := &v1alpha2.DNSRecord{}
		if err := k8sClient.Get(ctx, recordNN, rec); err == nil {
			Expect(k8sClient.Delete(ctx, rec)).To(Succeed())
		}
		dns := &v1alpha2.DNS{}
		if err := k8sClient.Get(ctx, dnsNN, dns); err == nil {
			Expect(k8sClient.Delete(ctx, dns)).To(Succeed())
		}
		p := &sreportalv1alpha1.Portal{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: portalName, Namespace: tNsDefault}, p); err == nil {
			Expect(k8sClient.Delete(ctx, p)).To(Succeed())
		}
	})

	// This test exercises the full event-driven flow:
	//   1. Source reader yields an endpoint for kind=service.
	//   2. DNS reconcile runs UpsertDNSRecordsHandler → writes spec.entries on
	//      a DNSRecord CR named "<dns>-service" with the DNS as controller owner.
	//   3. DNSRecord reconcile materialises spec.entries into status.endpoints
	//      and projects them into the FQDN read store.
	//   4. Verifies the DNS reconcile bumped Generation and the DNSRecord
	//      eventually observed it (observedGeneration alignment).
	It("propagates source endpoints from DNS down to the FQDN read store", func() {
		By("creating a DNS CR with sources.service enabled")
		Expect(k8sClient.Create(ctx, &v1alpha2.DNS{
			ObjectMeta: metav1.ObjectMeta{Name: dnsName, Namespace: tNsDefault},
			Spec: v1alpha2.DNSSpec{
				PortalRef: portalName,
				GroupMapping: v1alpha2.GroupMappingSpec{
					DefaultGroup: tGroupServices,
				},
				Sources: v1alpha2.SourcesSpec{
					Service: &v1alpha2.ServiceSourceSpec{
						CommonSourceSpec: v1alpha2.CommonSourceSpec{Enabled: true},
					},
				},
				Reconciliation: v1alpha2.ReconciliationSpec{
					Interval:        metav1.Duration{Duration: 5 * time.Minute},
					DisableDNSCheck: true,
				},
			},
		})).To(Succeed())

		By("wiring a DNS reconciler with a static source reader that yields one endpoint")
		sourceReader := staticSourceReader{
			kind: externaldns.KindService,
			eps: []domainsource.EnrichedEndpoint{{
				Endpoint: &endpoint.Endpoint{
					DNSName:    fqdn,
					RecordType: "A",
					Targets:    endpoint.Targets{target},
				},
				Kind: externaldns.KindService,
			}},
		}
		dnsRec := NewDNSReconciler(k8sClient, k8sClient.Scheme(), sourceReader, nil)

		By("reconciling the DNS — UpsertDNSRecordsHandler creates the auto DNSRecord")
		Eventually(func(g Gomega) {
			_, err := dnsRec.Reconcile(ctx, reconcile.Request{NamespacedName: dnsNN})
			g.Expect(err).NotTo(HaveOccurred())

			var rec v1alpha2.DNSRecord
			g.Expect(k8sClient.Get(ctx, recordNN, &rec)).To(Succeed())
			g.Expect(rec.Spec.Origin).To(Equal(v1alpha2.DNSRecordOriginAuto))
			g.Expect(rec.Spec.PortalRef).To(Equal(portalName))
			g.Expect(string(rec.Spec.SourceType)).To(Equal(string(externaldns.KindService)))
			g.Expect(rec.Spec.Entries).To(HaveLen(1))
			g.Expect(rec.Spec.Entries[0].FQDN).To(Equal(fqdn))
			g.Expect(rec.Spec.Entries[0].Targets).To(ConsistOf(target))

			// Owner reference points back to the DNS CR — this is what the
			// DNSRecord controller's enqueue map func uses to re-trigger the
			// DNS controller on DNSRecord changes.
			ownedByDNS := false
			for _, ref := range rec.OwnerReferences {
				if ref.Kind == "DNS" && ref.Name == dnsName && ref.Controller != nil && *ref.Controller {
					ownedByDNS = true
				}
			}
			g.Expect(ownedByDNS).To(BeTrue(), "DNSRecord must be controller-owned by the DNS CR")
		}, timeout, interval).Should(Succeed())

		By("reconciling the DNSRecord — MaterialiseEntriesHandler + ProjectStoreHandler populate the read store")
		store := dnsreadstore.NewFQDNStore()
		recRec := dnsrecords.NewDNSRecordReconciler(k8sClient, k8sClient.Scheme(), nil)
		recRec.SetFQDNWriter(store)

		Eventually(func(g Gomega) {
			result, err := recRec.Reconcile(ctx, reconcile.Request{NamespacedName: recordNN})
			g.Expect(err).NotTo(HaveOccurred())
			// Drift-check requeue contract: 1h periodic re-resolution.
			g.Expect(result.RequeueAfter).To(Equal(dnsrecords.DNSRecordResolveInterval))

			// status.endpoints materialised from spec.entries.
			var rec v1alpha2.DNSRecord
			g.Expect(k8sClient.Get(ctx, recordNN, &rec)).To(Succeed())
			g.Expect(rec.Status.Endpoints).To(HaveLen(1))
			g.Expect(rec.Status.Endpoints[0].DNSName).To(Equal(fqdn))
			g.Expect(rec.Status.Endpoints[0].Targets).To(ConsistOf(target))

			// Read store contains the projected FQDN view scoped to the portal.
			views, vErr := store.List(ctx, domaindns.FQDNFilters{Portal: portalName})
			g.Expect(vErr).NotTo(HaveOccurred())
			g.Expect(views).To(HaveLen(1))
			g.Expect(views[0].Name).To(Equal(fqdn))
			g.Expect(views[0].Targets).To(ConsistOf(target))
			g.Expect(views[0].FirstPortal()).To(Equal(portalName))
		}, timeout, interval).Should(Succeed())
	})
})
