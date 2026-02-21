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

package dns_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	dnspkg "github.com/golgoth31/sreportal/internal/controller/dns"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

var _ = Describe("UpdateStatusHandler", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(sreportalv1alpha1.AddToScheme(scheme)).To(Succeed())
	})

	buildClient := func(obj client.Object) client.Client {
		return fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(obj).
			WithStatusSubresource(obj).
			Build()
	}

	newDNS := func() *sreportalv1alpha1.DNS {
		return &sreportalv1alpha1.DNS{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-dns",
				Namespace: "default",
			},
		}
	}

	Context("when aggregated groups are present in Data", func() {
		It("should update the DNS status with all groups and set Ready condition", func() {
			dns := newDNS()
			c := buildClient(dns)
			handler := dnspkg.NewUpdateStatusHandler(c)

			groups := []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "APIs",
					Source: dnspkg.SourceExternalDNS,
					FQDNs:  []sreportalv1alpha1.FQDNStatus{{FQDN: "api.example.com"}},
				},
				{
					Name:   "Internal",
					Source: dnspkg.SourceManual,
					FQDNs:  []sreportalv1alpha1.FQDNStatus{{FQDN: "db.internal.com"}},
				},
			}

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DNS]{
				Resource: dns,
				Data: map[string]any{
					dnspkg.DataKeyAggregatedGroups: groups,
				},
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			// Verify the status was persisted via the fake client
			var updated sreportalv1alpha1.DNS
			Expect(c.Get(context.Background(), types.NamespacedName{Name: "test-dns", Namespace: "default"}, &updated)).To(Succeed())
			Expect(updated.Status.Groups).To(HaveLen(2))
			Expect(updated.Status.Groups[0].Name).To(Equal("APIs"))
			Expect(updated.Status.Groups[1].Name).To(Equal("Internal"))
			Expect(updated.Status.LastReconcileTime).NotTo(BeNil())

			// Ready condition must be present and true
			Expect(updated.Status.Conditions).NotTo(BeEmpty())
			readyCond := findCondition(updated.Status.Conditions, dnspkg.ConditionTypeReady)
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionTrue))
			Expect(readyCond.Reason).To(Equal("ReconcileSucceeded"))
		})
	})

	Context("when no aggregated groups key exists in Data", func() {
		It("should update status with empty groups and still set Ready condition", func() {
			dns := newDNS()
			c := buildClient(dns)
			handler := dnspkg.NewUpdateStatusHandler(c)

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DNS]{
				Resource: dns,
				Data:     make(map[string]any),
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var updated sreportalv1alpha1.DNS
			Expect(c.Get(context.Background(), types.NamespacedName{Name: "test-dns", Namespace: "default"}, &updated)).To(Succeed())
			Expect(updated.Status.Groups).To(BeEmpty())
			Expect(updated.Status.LastReconcileTime).NotTo(BeNil())

			readyCond := findCondition(updated.Status.Conditions, dnspkg.ConditionTypeReady)
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionTrue))
		})
	})

	Context("Ready condition management", func() {
		It("should preserve LastTransitionTime when condition status does not change", func() {
			dns := newDNS()
			// Truncate to second precision: metav1.Time serialises to RFC3339 (no
			// sub-second precision), so the fake client round-trips lose nanoseconds.
			originalTime := metav1.NewTime(metav1.Now().Add(-60 * time.Second).Truncate(time.Second))
			dns.Status.Conditions = []metav1.Condition{
				{
					Type:               dnspkg.ConditionTypeReady,
					Status:             metav1.ConditionTrue,
					Reason:             "ReconcileSucceeded",
					LastTransitionTime: originalTime,
				},
			}

			c := buildClient(dns)
			handler := dnspkg.NewUpdateStatusHandler(c)

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DNS]{
				Resource: dns,
				Data:     make(map[string]any),
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var updated sreportalv1alpha1.DNS
			Expect(c.Get(context.Background(), types.NamespacedName{Name: "test-dns", Namespace: "default"}, &updated)).To(Succeed())

			readyCond := findCondition(updated.Status.Conditions, dnspkg.ConditionTypeReady)
			Expect(readyCond).NotTo(BeNil())
			// LastTransitionTime must be preserved (not bumped) when status stays True.
			// Allow ±1s tolerance to absorb RFC3339 serialisation truncation.
			Expect(readyCond.LastTransitionTime.Time).To(BeTemporally("~", originalTime.Time, time.Second))
		})
	})
})

// findCondition returns the condition with the given type, or nil if not found.
func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
