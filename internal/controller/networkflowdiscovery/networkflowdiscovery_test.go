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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
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
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := NewNetworkFlowDiscoveryReconciler(
				k8sClient,
				k8sClient.Scheme(),
				remoteclient.NewCache(),
			)

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(networkFlowDiscoveryRequeueAfter))

			Eventually(func(g Gomega) {
				var updated sreportalv1alpha1.NetworkFlowDiscovery
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &updated)).To(Succeed())
				g.Expect(updated.Status.LastReconcileTime).NotTo(BeNil())
			}).Should(Succeed())
		})
	})
})
