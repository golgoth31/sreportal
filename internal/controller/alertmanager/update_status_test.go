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

package alertmanager_test

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
	alertmanagerctrl "github.com/golgoth31/sreportal/internal/controller/alertmanager"
	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanager"
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

	newAM := func() *sreportalv1alpha1.Alertmanager {
		return &sreportalv1alpha1.Alertmanager{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-am",
				Namespace: "default",
			},
			Spec: sreportalv1alpha1.AlertmanagerSpec{
				PortalRef: "main",
				URL: sreportalv1alpha1.AlertmanagerURL{
					Local: "http://alertmanager:9093",
				},
			},
		}
	}

	Context("when alerts are present in Data", func() {
		It("should update the Alertmanager status with all alerts and set Ready condition", func() {
			am := newAM()
			c := buildClient(am)
			handler := alertmanagerctrl.NewUpdateStatusHandler(c)

			startsAt := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
			endsAt := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)

			alerts := []domainalertmanager.Alert{
				{
					Fingerprint: "aaa",
					Labels:      map[string]string{"alertname": "HighCPU", "severity": "critical"},
					Annotations: map[string]string{"summary": "CPU is high"},
					State:       domainalertmanager.StateActive,
					StartsAt:    startsAt,
					UpdatedAt:   startsAt,
				},
				{
					Fingerprint: "bbb",
					Labels:      map[string]string{"alertname": "DiskFull"},
					State:       domainalertmanager.StateActive,
					StartsAt:    startsAt,
					EndsAt:      &endsAt,
					UpdatedAt:   startsAt,
				},
			}

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Alertmanager, alertmanagerctrl.ChainData]{
				Resource: am,
				Data: alertmanagerctrl.ChainData{
					Alerts: alerts,
				},
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var updated sreportalv1alpha1.Alertmanager
			Expect(c.Get(context.Background(), types.NamespacedName{Name: "test-am", Namespace: "default"}, &updated)).To(Succeed())
			Expect(updated.Status.ActiveAlerts).To(HaveLen(2))
			Expect(updated.Status.ActiveAlerts[0].Fingerprint).To(Equal("aaa"))
			Expect(updated.Status.ActiveAlerts[0].Labels["alertname"]).To(Equal("HighCPU"))
			Expect(updated.Status.ActiveAlerts[0].Annotations["summary"]).To(Equal("CPU is high"))
			Expect(updated.Status.ActiveAlerts[1].EndsAt).NotTo(BeNil())
			Expect(updated.Status.LastReconcileTime).NotTo(BeNil())

			Expect(updated.Status.Conditions).NotTo(BeEmpty())
			readyCond := findCondition(updated.Status.Conditions, alertmanagerctrl.ConditionTypeReady)
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionTrue))
			Expect(readyCond.Reason).To(Equal("ReconcileSucceeded"))
			Expect(readyCond.Message).To(ContainSubstring("2 active alerts"))
		})
	})

	Context("when no alerts key exists in Data", func() {
		It("should update status with empty alerts and still set Ready condition", func() {
			am := newAM()
			c := buildClient(am)
			handler := alertmanagerctrl.NewUpdateStatusHandler(c)

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Alertmanager, alertmanagerctrl.ChainData]{
				Resource: am,
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var updated sreportalv1alpha1.Alertmanager
			Expect(c.Get(context.Background(), types.NamespacedName{Name: "test-am", Namespace: "default"}, &updated)).To(Succeed())
			Expect(updated.Status.ActiveAlerts).To(BeEmpty())
			Expect(updated.Status.LastReconcileTime).NotTo(BeNil())

			readyCond := findCondition(updated.Status.Conditions, alertmanagerctrl.ConditionTypeReady)
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionTrue))
		})
	})

	Context("Ready condition management", func() {
		It("should preserve LastTransitionTime when condition status does not change", func() {
			am := newAM()
			originalTime := metav1.NewTime(metav1.Now().Add(-60 * time.Second).Truncate(time.Second))
			am.Status.Conditions = []metav1.Condition{
				{
					Type:               alertmanagerctrl.ConditionTypeReady,
					Status:             metav1.ConditionTrue,
					Reason:             "ReconcileSucceeded",
					LastTransitionTime: originalTime,
				},
			}

			c := buildClient(am)
			handler := alertmanagerctrl.NewUpdateStatusHandler(c)

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Alertmanager, alertmanagerctrl.ChainData]{
				Resource: am,
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var updated sreportalv1alpha1.Alertmanager
			Expect(c.Get(context.Background(), types.NamespacedName{Name: "test-am", Namespace: "default"}, &updated)).To(Succeed())

			readyCond := findCondition(updated.Status.Conditions, alertmanagerctrl.ConditionTypeReady)
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.LastTransitionTime.Time).To(BeTemporally("~", originalTime.Time, time.Second))
		})
	})
})

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
