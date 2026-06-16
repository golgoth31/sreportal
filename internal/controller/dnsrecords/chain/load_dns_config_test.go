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

package chain_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// newFakeClientWithDNSIndex builds a fake client with the same
// spec.portalRef field indexer that main.go wires for v1alpha2.DNS.
func newFakeClientWithDNSIndex(scheme *runtime.Scheme, objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithIndex(&v1alpha2.DNS{}, portalfeatures.FieldIndexPortalRef, func(o client.Object) []string {
			dns := o.(*v1alpha2.DNS)
			if dns.Spec.PortalRef == "" {
				return nil
			}
			return []string{dns.Spec.PortalRef}
		}).
		Build()
}

func TestLoadDNSConfigHandler_PopulatesChainData(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = v1alpha2.AddToScheme(scheme)

	dns := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: tNsDefault},
		Spec: v1alpha2.DNSSpec{
			PortalRef: tPortalMain,
			GroupMapping: v1alpha2.GroupMappingSpec{
				DefaultGroup: "MyServices",
			},
			Reconciliation: v1alpha2.ReconciliationSpec{
				DisableDNSCheck: true,
			},
		},
	}

	c := newFakeClientWithDNSIndex(scheme, dns)

	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "main-ingress", Namespace: tNsDefault},
		Spec: v1alpha2.DNSRecordSpec{
			Origin:     v1alpha2.DNSRecordOriginAuto,
			PortalRef:  tPortalMain,
			SourceType: "ingress",
		},
	}

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: record,
		Data:     chain.ChainData{ResourceKey: "default/main-ingress"},
	}

	h := chain.NewLoadDNSConfigHandler(c)
	err := h.Handle(context.Background(), rc)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rc.Data.GroupMapping).NotTo(BeNil())
	g.Expect(rc.Data.GroupMapping.DefaultGroup).To(Equal("MyServices"))
	g.Expect(rc.Data.DisableDNSCheck).To(BeTrue())
}

func TestLoadDNSConfigHandler_MissingDNS_ShortCircuits(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1alpha2.AddToScheme(scheme)).To(Succeed())
	c := newFakeClientWithDNSIndex(scheme)

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: &v1alpha2.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "orphan", Namespace: tNsDefault},
			Spec:       v1alpha2.DNSRecordSpec{PortalRef: "missing"},
		},
	}

	err := chain.NewLoadDNSConfigHandler(c).Handle(context.Background(), rc)
	g.Expect(errors.Is(err, reconciler.ErrShortCircuit)).To(BeTrue())
	g.Expect(rc.Data.GroupMapping).To(BeNil())
}

// TestLoadDNSConfigHandler_DNSNameMismatchesPortalRef verifies that the
// handler resolves the DNS CR by Spec.PortalRef via the field index, not by
// Name. A DNS whose Name differs from PortalRef must still be discovered.
func TestLoadDNSConfigHandler_DNSNameMismatchesPortalRef(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1alpha2.AddToScheme(scheme)).To(Succeed())

	// DNS CR where Name != PortalRef value used by the record.
	dns := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "dns-platform", Namespace: tNsDefault},
		Spec: v1alpha2.DNSSpec{
			PortalRef: tPortalMain,
			GroupMapping: v1alpha2.GroupMappingSpec{
				DefaultGroup: "Platform",
			},
		},
	}

	c := newFakeClientWithDNSIndex(scheme, dns)

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: &v1alpha2.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "svc-ingress", Namespace: tNsDefault},
			Spec: v1alpha2.DNSRecordSpec{
				Origin:    v1alpha2.DNSRecordOriginAuto,
				PortalRef: tPortalMain,
			},
		},
	}

	err := chain.NewLoadDNSConfigHandler(c).Handle(context.Background(), rc)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rc.Data.GroupMapping).NotTo(BeNil())
	g.Expect(rc.Data.GroupMapping.DefaultGroup).To(Equal("Platform"))
}

// TestLoadDNSConfigHandler_MultipleDNS_PrefersOwner verifies that when several
// DNS CRs reference the same Portal (N:1 allowed), an auto record resolves the
// config of its owning DNS (via OwnerDNSName), not an arbitrary list entry.
func TestLoadDNSConfigHandler_MultipleDNS_PrefersOwner(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1alpha2.AddToScheme(scheme)).To(Succeed())

	dnsA := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "dns-a", Namespace: tNsDefault},
		Spec: v1alpha2.DNSSpec{
			PortalRef:    tPortalMain,
			GroupMapping: v1alpha2.GroupMappingSpec{DefaultGroup: "GroupA"},
		},
	}
	dnsB := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "dns-b", Namespace: tNsDefault},
		Spec: v1alpha2.DNSSpec{
			PortalRef:    tPortalMain,
			GroupMapping: v1alpha2.GroupMappingSpec{DefaultGroup: "GroupB"},
		},
	}
	c := newFakeClientWithDNSIndex(scheme, dnsA, dnsB)

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: &v1alpha2.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "svc-ingress", Namespace: tNsDefault},
			Spec: v1alpha2.DNSRecordSpec{
				Origin:    v1alpha2.DNSRecordOriginAuto,
				PortalRef: tPortalMain,
			},
		},
		Data: chain.ChainData{OwnerDNSName: "dns-b"},
	}

	err := chain.NewLoadDNSConfigHandler(c).Handle(context.Background(), rc)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rc.Data.GroupMapping).NotTo(BeNil())
	g.Expect(rc.Data.GroupMapping.DefaultGroup).To(Equal("GroupB"))
}

// TestLoadDNSConfigHandler_MultipleDNS_NoOwner_LowestName verifies the
// deterministic fallback: with no owner (e.g. a manual record), the DNS with
// the lowest name is selected regardless of list ordering, so an unchanged
// record never flaps between configs.
func TestLoadDNSConfigHandler_MultipleDNS_NoOwner_LowestName(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1alpha2.AddToScheme(scheme)).To(Succeed())

	dnsZ := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "dns-z", Namespace: tNsDefault},
		Spec: v1alpha2.DNSSpec{
			PortalRef:    tPortalMain,
			GroupMapping: v1alpha2.GroupMappingSpec{DefaultGroup: "GroupZ"},
		},
	}
	dnsA := &v1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "dns-a", Namespace: tNsDefault},
		Spec: v1alpha2.DNSSpec{
			PortalRef:    tPortalMain,
			GroupMapping: v1alpha2.GroupMappingSpec{DefaultGroup: "GroupA"},
		},
	}
	// Pass dns-z first to ensure selection isn't insertion-order dependent.
	c := newFakeClientWithDNSIndex(scheme, dnsZ, dnsA)

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{
		Resource: &v1alpha2.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{Name: "manual-rec", Namespace: tNsDefault},
			Spec: v1alpha2.DNSRecordSpec{
				Origin:    v1alpha2.DNSRecordOriginManual,
				PortalRef: tPortalMain,
			},
		},
	}

	err := chain.NewLoadDNSConfigHandler(c).Handle(context.Background(), rc)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rc.Data.GroupMapping).NotTo(BeNil())
	g.Expect(rc.Data.GroupMapping.DefaultGroup).To(Equal("GroupA"))
}
