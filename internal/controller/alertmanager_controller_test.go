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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanager"
)

type fakeFetcher struct {
	alerts []domainalertmanager.Alert
	err    error
}

func (f *fakeFetcher) GetActiveAlerts(_ context.Context, _ string) ([]domainalertmanager.Alert, error) {
	return f.alerts, f.err
}

var _ = Describe("Alertmanager Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		alertmanager := &sreportalv1alpha1.Alertmanager{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Alertmanager")
			err := k8sClient.Get(ctx, typeNamespacedName, alertmanager)
			if err != nil && errors.IsNotFound(err) {
				resource := &sreportalv1alpha1.Alertmanager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: sreportalv1alpha1.AlertmanagerSpec{
						PortalRef: "main",
						URL: sreportalv1alpha1.AlertmanagerURL{
							Local: "http://alertmanager:9093",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &sreportalv1alpha1.Alertmanager{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Alertmanager")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile and populate status with alerts", func() {
			By("Reconciling the created resource")
			fetcher := &fakeFetcher{
				alerts: []domainalertmanager.Alert{
					{
						Fingerprint: "aaa",
						Labels:      map[string]string{"alertname": "HighCPU", "severity": "critical"},
						State:       domainalertmanager.StateActive,
					},
				},
			}
			controllerReconciler := NewAlertmanagerReconciler(
				k8sClient,
				k8sClient.Scheme(),
				fetcher,
			)

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(alertmanagerRequeueAfter))

			var updated sreportalv1alpha1.Alertmanager
			Expect(k8sClient.Get(ctx, typeNamespacedName, &updated)).To(Succeed())
			Expect(updated.Status.ActiveAlerts).To(HaveLen(1))
			Expect(updated.Status.ActiveAlerts[0].Labels["alertname"]).To(Equal("HighCPU"))
			Expect(updated.Status.LastReconcileTime).NotTo(BeNil())
		})

		It("should handle fetch errors gracefully and requeue", func() {
			By("Reconciling with a failing fetcher")
			fetcher := &fakeFetcher{
				err: domainalertmanager.ErrFetchAlerts,
			}
			controllerReconciler := NewAlertmanagerReconciler(
				k8sClient,
				k8sClient.Scheme(),
				fetcher,
			)

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(alertmanagerRequeueAfter))
		})
	})
})
