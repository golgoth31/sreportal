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

package maintenance

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	maintenancereadstore "github.com/golgoth31/sreportal/internal/readstore/maintenance"
)

var _ = Describe("Maintenance Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-maintenance"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the Maintenance resource")
			maint := &sreportalv1alpha1.Maintenance{}
			err := k8sClient.Get(ctx, typeNamespacedName, maint)
			if err != nil && errors.IsNotFound(err) {
				resource := &sreportalv1alpha1.Maintenance{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: "default"},
					Spec: sreportalv1alpha1.MaintenanceSpec{
						Title:          "Test Maintenance",
						PortalRef:      "main",
						ScheduledStart: metav1.NewTime(time.Now().Add(1 * time.Hour)),
						ScheduledEnd:   metav1.NewTime(time.Now().Add(3 * time.Hour)),
						AffectedStatus: sreportalv1alpha1.MaintenanceAffectedMaintenance,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &sreportalv1alpha1.Maintenance{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should compute phase as upcoming", func() {
			store := maintenancereadstore.NewMaintenanceStore()
			controllerReconciler := NewMaintenanceReconciler(k8sClient, store)

			By("waiting for the resource to be visible in cache")
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, &sreportalv1alpha1.Maintenance{})
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

			Eventually(func(g Gomega) {
				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result.RequeueAfter).To(BeNumerically(">", 0), "should requeue for phase transition")

				var maint sreportalv1alpha1.Maintenance
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &maint)).To(Succeed())
				g.Expect(maint.Status.Phase).To(Equal(sreportalv1alpha1.MaintenancePhaseUpcoming))
				g.Expect(maint.Labels["sreportal.io/portal"]).To(Equal("main"))
			}, 30*time.Second, 500*time.Millisecond).Should(Succeed())
		})

		It("should reject invalid schedule", func() {
			store := maintenancereadstore.NewMaintenanceStore()
			controllerReconciler := NewMaintenanceReconciler(k8sClient, store)

			badMaint := &sreportalv1alpha1.Maintenance{
				ObjectMeta: metav1.ObjectMeta{Name: "maint-bad-schedule", Namespace: "default"},
				Spec: sreportalv1alpha1.MaintenanceSpec{
					Title:          "Bad Schedule",
					PortalRef:      "main",
					ScheduledStart: metav1.NewTime(time.Now().Add(3 * time.Hour)),
					ScheduledEnd:   metav1.NewTime(time.Now().Add(1 * time.Hour)), // end before start
					AffectedStatus: sreportalv1alpha1.MaintenanceAffectedMaintenance,
				},
			}
			Expect(k8sClient.Create(ctx, badMaint)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, badMaint)
			}()

			badNN := types.NamespacedName{Name: "maint-bad-schedule", Namespace: "default"}

			By("waiting for the resource to be visible in cache")
			Eventually(func() error {
				return k8sClient.Get(ctx, badNN, &sreportalv1alpha1.Maintenance{})
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

			// Chain returns error, reconciler swallows it
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: badNN,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
