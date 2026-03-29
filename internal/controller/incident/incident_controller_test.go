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

package incident

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
	incidentreadstore "github.com/golgoth31/sreportal/internal/readstore/incident"
)

var _ = Describe("Incident Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-incident"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the Incident resource")
			inc := &sreportalv1alpha1.Incident{}
			err := k8sClient.Get(ctx, typeNamespacedName, inc)
			if err != nil && errors.IsNotFound(err) {
				resource := &sreportalv1alpha1.Incident{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: "default"},
					Spec: sreportalv1alpha1.IncidentSpec{
						Title:     "Test Incident",
						PortalRef: "main",
						Severity:  sreportalv1alpha1.IncidentSeverityMajor,
						Updates: []sreportalv1alpha1.IncidentUpdate{
							{
								Timestamp: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
								Phase:     sreportalv1alpha1.IncidentPhaseInvestigating,
								Message:   "Investigating the issue",
							},
							{
								Timestamp: metav1.NewTime(time.Now()),
								Phase:     sreportalv1alpha1.IncidentPhaseResolved,
								Message:   "Issue resolved",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &sreportalv1alpha1.Incident{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should compute phase as resolved with duration", func() {
			store := incidentreadstore.NewIncidentStore()
			controllerReconciler := NewIncidentReconciler(k8sClient, store)

			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				g.Expect(err).NotTo(HaveOccurred())

				var inc sreportalv1alpha1.Incident
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &inc)).To(Succeed())
				g.Expect(inc.Status.CurrentPhase).To(Equal(sreportalv1alpha1.IncidentPhaseResolved))
				g.Expect(inc.Status.DurationMinutes).To(BeNumerically(">=", 59))
				g.Expect(inc.Status.ResolvedAt).NotTo(BeNil())
				g.Expect(inc.Labels["sreportal.io/portal"]).To(Equal("main"))
			}, 10*time.Second, 500*time.Millisecond).Should(Succeed())
		})

		It("should handle incident with no updates gracefully", func() {
			store := incidentreadstore.NewIncidentStore()
			controllerReconciler := NewIncidentReconciler(k8sClient, store)

			emptyInc := &sreportalv1alpha1.Incident{
				ObjectMeta: metav1.ObjectMeta{Name: "inc-no-updates", Namespace: "default"},
				Spec: sreportalv1alpha1.IncidentSpec{
					Title:     "No Updates Incident",
					PortalRef: "main",
					Severity:  sreportalv1alpha1.IncidentSeverityMinor,
				},
			}
			Expect(k8sClient.Create(ctx, emptyInc)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, emptyInc)
			}()

			// Chain returns error (ErrNoUpdates), reconciler swallows it
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "inc-no-updates", Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
