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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

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

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dns).
		Build()

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
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

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
