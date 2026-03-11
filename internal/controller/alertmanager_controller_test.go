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
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

		// directClient reads/writes from the API server directly. The manager's client
		// uses a cache that is not updated for Alertmanager (no controller watches it),
		// so Create/Get/Patch would be inconsistent. Use directClient for all operations.
		var directClient client.Client

		BeforeEach(func() {
			var err error
			directClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
			Expect(err).NotTo(HaveOccurred())

			By("creating the custom resource for the Kind Alertmanager")
			err = directClient.Get(ctx, typeNamespacedName, alertmanager)
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
				Expect(directClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &sreportalv1alpha1.Alertmanager{}
			Expect(directClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())

			By("Cleanup the specific resource instance Alertmanager")
			Expect(directClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile and populate status with alerts", func() {
			By("Reconciling the created resource")
			startsAt := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
			updatedAt := time.Date(2026, 3, 8, 10, 5, 0, 0, time.UTC)
			fetcher := &fakeFetcher{
				alerts: []domainalertmanager.Alert{
					{
						Fingerprint: "aaa",
						Labels:      map[string]string{"alertname": "HighCPU", "severity": "critical"},
						State:       domainalertmanager.StateActive,
						StartsAt:    startsAt,
						UpdatedAt:   updatedAt,
					},
				},
			}
			controllerReconciler := NewAlertmanagerReconciler(
				directClient,
				directClient.Scheme(),
				nil,
				fetcher,
			)

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(alertmanagerRequeueAfter))

			var updated sreportalv1alpha1.Alertmanager
			Expect(directClient.Get(ctx, typeNamespacedName, &updated)).To(Succeed())
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
				directClient,
				directClient.Scheme(),
				nil,
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
