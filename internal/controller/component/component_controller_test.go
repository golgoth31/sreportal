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

package component

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
	componentreadstore "github.com/golgoth31/sreportal/internal/readstore/component"
	maintenancereadstore "github.com/golgoth31/sreportal/internal/readstore/maintenance"
)

var _ = Describe("Component Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-component"
		const portalName = "test-portal-comp"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: tNsDefault,
		}

		BeforeEach(func() {
			By("creating the Portal dependency")
			portal := &sreportalv1alpha1.Portal{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: portalName, Namespace: tNsDefault}, portal)
			if err != nil && errors.IsNotFound(err) {
				portal = &sreportalv1alpha1.Portal{
					ObjectMeta: metav1.ObjectMeta{Name: portalName, Namespace: tNsDefault},
					Spec:       sreportalv1alpha1.PortalSpec{Title: "Test Portal"},
				}
				Expect(k8sClient.Create(ctx, portal)).To(Succeed())
			}

			By("creating the Component resource")
			comp := &sreportalv1alpha1.Component{}
			err = k8sClient.Get(ctx, typeNamespacedName, comp)
			if err != nil && errors.IsNotFound(err) {
				resource := &sreportalv1alpha1.Component{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: tNsDefault},
					Spec: sreportalv1alpha1.ComponentSpec{
						DisplayName: "Test Component",
						Group:       "Infrastructure",
						PortalRef:   portalName,
						Status:      sreportalv1alpha1.ComponentStatusOperational,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("waiting for cache to sync the Component")
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, &sreportalv1alpha1.Component{})
			}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
		})

		AfterEach(func() {
			resource := &sreportalv1alpha1.Component{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			portal := &sreportalv1alpha1.Portal{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: portalName, Namespace: tNsDefault}, portal)
			if err == nil {
				Expect(k8sClient.Delete(ctx, portal)).To(Succeed())
			}
		})

		It("should successfully reconcile and set computedStatus", func() {
			maintStore := maintenancereadstore.NewMaintenanceStore()
			compStore := componentreadstore.NewComponentStore()
			controllerReconciler := NewComponentReconciler(k8sClient, maintStore, compStore)

			Eventually(func(g Gomega) {
				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				g.Expect(err).NotTo(HaveOccurred())

				var comp sreportalv1alpha1.Component
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &comp)).To(Succeed())
				g.Expect(comp.Status.ComputedStatus).To(Equal(sreportalv1alpha1.ComputedStatusOperational))
				g.Expect(comp.Labels["sreportal.io/portal"]).To(Equal(portalName))

				// Daily status: should have at least one entry for today
				g.Expect(comp.Status.DailyWorstStatus).NotTo(BeEmpty(), "should have daily status entries")
				today := time.Now().UTC().Format("2006-01-02")
				g.Expect(comp.Status.DailyWorstStatus[len(comp.Status.DailyWorstStatus)-1].Date).To(Equal(today))

				// Requeue: should be scheduled for next UTC midnight
				g.Expect(result.RequeueAfter).To(BeNumerically(">", 0), "should requeue for midnight")
				g.Expect(result.RequeueAfter).To(BeNumerically("<=", 24*time.Hour), "requeue must be within 24h")
			}, 10*time.Second, 500*time.Millisecond).Should(Succeed())
		})

		It("should update computedStatus when spec.status changes", func() {
			const statusChangeComp = "comp-status-change"
			const statusChangePortal = "portal-status-change"
			statusChangeNN := types.NamespacedName{Name: statusChangeComp, Namespace: tNsDefault}

			By("creating dedicated Portal and Component")
			portal := &sreportalv1alpha1.Portal{
				ObjectMeta: metav1.ObjectMeta{Name: statusChangePortal, Namespace: tNsDefault},
				Spec:       sreportalv1alpha1.PortalSpec{Title: "Status Change Portal"},
			}
			Expect(k8sClient.Create(ctx, portal)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, portal) }()

			comp := &sreportalv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{Name: statusChangeComp, Namespace: tNsDefault},
				Spec: sreportalv1alpha1.ComponentSpec{
					DisplayName: "Status Change Comp",
					Group:       tKindTest,
					PortalRef:   statusChangePortal,
					Status:      sreportalv1alpha1.ComponentStatusOperational,
				},
			}
			Expect(k8sClient.Create(ctx, comp)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, comp) }()

			maintStore := maintenancereadstore.NewMaintenanceStore()
			compStore := componentreadstore.NewComponentStore()
			controllerReconciler := NewComponentReconciler(k8sClient, maintStore, compStore)

			// First reconcile — sets condition Ready=True and computedStatus=operational
			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: statusChangeNN,
				})
				g.Expect(err).NotTo(HaveOccurred())

				var fetched sreportalv1alpha1.Component
				g.Expect(k8sClient.Get(ctx, statusChangeNN, &fetched)).To(Succeed())
				g.Expect(fetched.Status.ComputedStatus).To(Equal(sreportalv1alpha1.ComputedStatusOperational))
			}, 10*time.Second, 500*time.Millisecond).Should(Succeed())

			// Update spec.status to degraded
			Eventually(func(g Gomega) {
				var fetched sreportalv1alpha1.Component
				g.Expect(k8sClient.Get(ctx, statusChangeNN, &fetched)).To(Succeed())
				fetched.Spec.Status = sreportalv1alpha1.ComponentStatusDegraded
				g.Expect(k8sClient.Update(ctx, &fetched)).To(Succeed())
			}, 10*time.Second, 500*time.Millisecond).Should(Succeed())

			// Second reconcile — computedStatus must reflect the new spec.status
			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: statusChangeNN,
				})
				g.Expect(err).NotTo(HaveOccurred())

				var fetched sreportalv1alpha1.Component
				g.Expect(k8sClient.Get(ctx, statusChangeNN, &fetched)).To(Succeed())
				g.Expect(fetched.Status.ComputedStatus).To(Equal(sreportalv1alpha1.ComputedStatusDegraded))
				g.Expect(fetched.Status.LastStatusChange).NotTo(BeNil())
			}, 10*time.Second, 500*time.Millisecond).Should(Succeed())
		})

		It("should requeue when portalRef does not exist", func() {
			maintStore := maintenancereadstore.NewMaintenanceStore()
			compStore := componentreadstore.NewComponentStore()
			controllerReconciler := NewComponentReconciler(k8sClient, maintStore, compStore)

			badComp := &sreportalv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{Name: "comp-bad-portal", Namespace: tNsDefault},
				Spec: sreportalv1alpha1.ComponentSpec{
					DisplayName: "Bad Portal Comp",
					Group:       tKindTest,
					PortalRef:   "nonexistent-portal",
					Status:      sreportalv1alpha1.ComponentStatusOperational,
				},
			}
			Expect(k8sClient.Create(ctx, badComp)).To(Succeed())
			defer func() {
				_ = k8sClient.Delete(ctx, badComp)
			}()

			// Wait for cache to sync, then reconcile — the chain should requeue
			Eventually(func(g Gomega) {
				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: "comp-bad-portal", Namespace: tNsDefault},
				})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result.RequeueAfter).To(BeNumerically(">", 0), "should requeue on portal not found")
			}, 10*time.Second, 500*time.Millisecond).Should(Succeed())
		})

		It("should override computedStatus based on active incident severity", func() {
			const incCompName = "comp-incident"
			const incPortalName = "portal-incident"
			incCompNN := types.NamespacedName{Name: incCompName, Namespace: tNsDefault}

			By("creating dedicated Portal and Component")
			portal := &sreportalv1alpha1.Portal{
				ObjectMeta: metav1.ObjectMeta{Name: incPortalName, Namespace: tNsDefault},
				Spec:       sreportalv1alpha1.PortalSpec{Title: "Incident Portal"},
			}
			Expect(k8sClient.Create(ctx, portal)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, portal) }()

			comp := &sreportalv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{Name: incCompName, Namespace: tNsDefault},
				Spec: sreportalv1alpha1.ComponentSpec{
					DisplayName: "Incident Comp",
					Group:       tKindTest,
					PortalRef:   incPortalName,
					Status:      sreportalv1alpha1.ComponentStatusOperational,
				},
			}
			Expect(k8sClient.Create(ctx, comp)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, comp) }()

			By("creating an Incident CR with a critical investigating update")
			inc := &sreportalv1alpha1.Incident{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "inc-1",
					Namespace: tNsDefault,
					Labels:    map[string]string{"sreportal.io/portal": incPortalName},
				},
				Spec: sreportalv1alpha1.IncidentSpec{
					Title:      "Test Incident",
					PortalRef:  incPortalName,
					Components: []string{incCompName},
					Severity:   sreportalv1alpha1.IncidentSeverityCritical,
					Updates: []sreportalv1alpha1.IncidentUpdate{
						{
							Timestamp: metav1.Now(),
							Phase:     sreportalv1alpha1.IncidentPhaseInvestigating,
							Message:   "investigating",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, inc)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, inc) }()

			maintStore := maintenancereadstore.NewMaintenanceStore()
			compStore := componentreadstore.NewComponentStore()
			controllerReconciler := NewComponentReconciler(k8sClient, maintStore, compStore)

			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: incCompNN,
				})
				g.Expect(err).NotTo(HaveOccurred())

				var fetched sreportalv1alpha1.Component
				g.Expect(k8sClient.Get(ctx, incCompNN, &fetched)).To(Succeed())
				g.Expect(fetched.Status.ComputedStatus).To(Equal(sreportalv1alpha1.ComputedStatusMajorOutage))
				g.Expect(fetched.Status.ActiveIncidents).To(Equal(1))
			}, 10*time.Second, 500*time.Millisecond).Should(Succeed())
		})

		It("should revert computedStatus when incident is resolved", func() {
			const resCompName = "comp-resolve"
			const resPortalName = "portal-resolve"
			resCompNN := types.NamespacedName{Name: resCompName, Namespace: tNsDefault}

			By("creating dedicated Portal and Component")
			portal := &sreportalv1alpha1.Portal{
				ObjectMeta: metav1.ObjectMeta{Name: resPortalName, Namespace: tNsDefault},
				Spec:       sreportalv1alpha1.PortalSpec{Title: "Resolve Portal"},
			}
			Expect(k8sClient.Create(ctx, portal)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, portal) }()

			comp := &sreportalv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{Name: resCompName, Namespace: tNsDefault},
				Spec: sreportalv1alpha1.ComponentSpec{
					DisplayName: "Resolve Comp",
					Group:       tKindTest,
					PortalRef:   resPortalName,
					Status:      sreportalv1alpha1.ComponentStatusOperational,
				},
			}
			Expect(k8sClient.Create(ctx, comp)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, comp) }()

			maintStore := maintenancereadstore.NewMaintenanceStore()
			compStore := componentreadstore.NewComponentStore()
			controllerReconciler := NewComponentReconciler(k8sClient, maintStore, compStore)

			By("creating an Incident CR with a critical investigating update")
			now := metav1.Now()
			inc := &sreportalv1alpha1.Incident{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "inc-res",
					Namespace: tNsDefault,
					Labels:    map[string]string{"sreportal.io/portal": resPortalName},
				},
				Spec: sreportalv1alpha1.IncidentSpec{
					Title:      "Resolve Test Incident",
					PortalRef:  resPortalName,
					Components: []string{resCompName},
					Severity:   sreportalv1alpha1.IncidentSeverityCritical,
					Updates: []sreportalv1alpha1.IncidentUpdate{
						{
							Timestamp: now,
							Phase:     sreportalv1alpha1.IncidentPhaseInvestigating,
							Message:   "investigating",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, inc)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, inc) }()

			By("reconciling with the active critical incident")
			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: resCompNN,
				})
				g.Expect(err).NotTo(HaveOccurred())

				var fetched sreportalv1alpha1.Component
				g.Expect(k8sClient.Get(ctx, resCompNN, &fetched)).To(Succeed())
				g.Expect(fetched.Status.ComputedStatus).To(Equal(sreportalv1alpha1.ComputedStatusMajorOutage))
				g.Expect(fetched.Status.ActiveIncidents).To(Equal(1))
			}, 10*time.Second, 500*time.Millisecond).Should(Succeed())

			By("resolving the incident by adding a resolved update")
			Eventually(func(g Gomega) {
				var fetched sreportalv1alpha1.Incident
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "inc-res", Namespace: tNsDefault}, &fetched)).To(Succeed())
				resolvedAt := metav1.NewTime(now.Add(time.Minute))
				fetched.Spec.Updates = append(fetched.Spec.Updates, sreportalv1alpha1.IncidentUpdate{
					Timestamp: resolvedAt,
					Phase:     sreportalv1alpha1.IncidentPhaseResolved,
					Message:   "resolved",
				})
				g.Expect(k8sClient.Update(ctx, &fetched)).To(Succeed())
			}, 10*time.Second, 500*time.Millisecond).Should(Succeed())

			By("reconciling again — status should revert to operational")
			Eventually(func(g Gomega) {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: resCompNN,
				})
				g.Expect(err).NotTo(HaveOccurred())

				var fetched sreportalv1alpha1.Component
				g.Expect(k8sClient.Get(ctx, resCompNN, &fetched)).To(Succeed())
				g.Expect(fetched.Status.ComputedStatus).To(Equal(sreportalv1alpha1.ComputedStatusOperational))
				g.Expect(fetched.Status.ActiveIncidents).To(Equal(0))
			}, 10*time.Second, 500*time.Millisecond).Should(Succeed())
		})
	})
})
