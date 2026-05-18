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
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

func TestSyncHash_ClearedWhenEndpointsBecomeEmpty(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1alpha2.AddToScheme(scheme)).To(Succeed())

	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "rec", Namespace: tNsDefault},
		Spec:       v1alpha2.DNSRecordSpec{PortalRef: tPortalMain},
		Status: v1alpha2.DNSRecordStatus{
			Endpoints:     nil,
			EndpointsHash: "stale-hash",
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(record).
		WithStatusSubresource(&v1alpha2.DNSRecord{}).
		Build()

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: record}
	g.Expect(chain.NewSyncEndpointsHashHandler(c).Handle(context.Background(), rc)).To(Succeed())

	var after v1alpha2.DNSRecord
	g.Expect(c.Get(context.Background(), client.ObjectKeyFromObject(record), &after)).To(Succeed())
	g.Expect(after.Status.EndpointsHash).To(BeEmpty())
}

func TestSyncHash_NoopWhenAlreadyEmpty(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1alpha2.AddToScheme(scheme)).To(Succeed())

	record := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "rec", Namespace: tNsDefault},
		Spec:       v1alpha2.DNSRecordSpec{PortalRef: tPortalMain},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(record).
		WithStatusSubresource(&v1alpha2.DNSRecord{}).
		Build()

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, chain.ChainData]{Resource: record}
	g.Expect(chain.NewSyncEndpointsHashHandler(c).Handle(context.Background(), rc)).To(Succeed())
}
