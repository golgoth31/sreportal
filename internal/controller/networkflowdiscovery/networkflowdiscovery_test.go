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

package networkflowdiscovery

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	netpolreadstore "github.com/golgoth31/sreportal/internal/readstore/netpol"
	"github.com/golgoth31/sreportal/internal/remoteclient"
)

var _ = Describe("NetworkFlowDiscovery Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-nfd"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the custom resource for the Kind NetworkFlowDiscovery")
			nfd := &sreportalv1alpha1.NetworkFlowDiscovery{}
			err := k8sClient.Get(ctx, typeNamespacedName, nfd)
			if err != nil && errors.IsNotFound(err) {
				resource := &sreportalv1alpha1.NetworkFlowDiscovery{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: sreportalv1alpha1.NetworkFlowDiscoverySpec{
						PortalRef: "main",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &sreportalv1alpha1.NetworkFlowDiscovery{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup the specific resource instance NetworkFlowDiscovery")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, typeNamespacedName, resource))
				}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := NewNetworkFlowDiscoveryReconciler(
				k8sClient,
				k8sClient.Scheme(),
				remoteclient.NewCache(),
			)

			Eventually(func(g Gomega) {
				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result.RequeueAfter).To(Equal(networkFlowDiscoveryRequeueAfter))
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

			Eventually(func(g Gomega) {
				var updated sreportalv1alpha1.NetworkFlowDiscovery
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &updated)).To(Succeed())
				g.Expect(updated.Status.LastReconcileTime).NotTo(BeNil())
			}).Should(Succeed())
		})
	})

	Context("When networkPolicy feature is disabled on the referenced portal", func() {
		const (
			nfdName    = "nfd-feature-toggle"
			portalName = "portal-netpol-toggle"
		)
		ctx := context.Background()
		nfdNN := types.NamespacedName{Name: nfdName, Namespace: "default"}
		portalNN := types.NamespacedName{Name: portalName, Namespace: "default"}

		AfterEach(func() {
			nfd := &sreportalv1alpha1.NetworkFlowDiscovery{}
			if err := k8sClient.Get(ctx, nfdNN, nfd); err == nil {
				Expect(k8sClient.Delete(ctx, nfd)).To(Succeed())
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, nfdNN, nfd))
				}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())
			}
			portal := &sreportalv1alpha1.Portal{}
			if err := k8sClient.Get(ctx, portalNN, portal); err == nil {
				Expect(k8sClient.Delete(ctx, portal)).To(Succeed())
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, portalNN, portal))
				}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())
			}
		})

		It("should purge flow graph data from the read store", func() {
			store := netpolreadstore.NewFlowGraphStore()
			reconciler := NewNetworkFlowDiscoveryReconciler(k8sClient, k8sClient.Scheme(), remoteclient.NewCache())
			reconciler.SetFlowGraphWriter(store)

			By("creating a portal with networkPolicy disabled")
			netpolOff := false
			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.Portal{
				ObjectMeta: metav1.ObjectMeta{Name: portalName, Namespace: "default"},
				Spec: sreportalv1alpha1.PortalSpec{
					Title:    "Netpol Toggle Portal",
					Features: &sreportalv1alpha1.PortalFeatures{NetworkPolicy: &netpolOff},
				},
			})).To(Succeed())

			By("creating an NFD referencing the portal")
			Expect(k8sClient.Create(ctx, &sreportalv1alpha1.NetworkFlowDiscovery{
				ObjectMeta: metav1.ObjectMeta{Name: nfdName, Namespace: "default"},
				Spec: sreportalv1alpha1.NetworkFlowDiscoverySpec{
					PortalRef: portalName,
				},
			})).To(Succeed())

			By("pre-populating the flow graph store")
			Expect(store.ReplaceNodes(ctx, nfdName, portalName, []domainnetpol.FlowNode{
				{ID: "service:default:app1", Label: "app1", Namespace: "default", NodeType: "service", Group: "default"},
			})).To(Succeed())
			Expect(store.ReplaceEdges(ctx, nfdName, portalName, []domainnetpol.FlowEdge{
				{From: "service:default:app1", To: "service:default:app2", EdgeType: "internal"},
			})).To(Succeed())

			nodes, _ := store.ListNodes(ctx, domainnetpol.FlowGraphFilters{Portal: portalName})
			Expect(nodes).To(HaveLen(1), "pre-condition: store should have 1 node")

			By("reconciling — flow graph data should be purged")
			Eventually(func(g Gomega) {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nfdNN})
				g.Expect(err).NotTo(HaveOccurred())

				gotNodes, err := store.ListNodes(ctx, domainnetpol.FlowGraphFilters{Portal: portalName})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(gotNodes).To(BeEmpty(), "nodes should be purged when feature is disabled")

				gotEdges, err := store.ListEdges(ctx, domainnetpol.FlowGraphFilters{Portal: portalName})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(gotEdges).To(BeEmpty(), "edges should be purged when feature is disabled")
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
		})
	})
})
