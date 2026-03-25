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

package networkflowdiscovery_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	nfdchain "github.com/golgoth31/sreportal/internal/controller/networkflowdiscovery"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
)

var _ = Describe("FetchRemoteGraphHandler", func() {
	newRC := func(isRemote bool) *reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, nfdchain.ChainData] {
		return &reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, nfdchain.ChainData]{
			Resource: &sreportalv1alpha1.NetworkFlowDiscovery{
				ObjectMeta: metav1.ObjectMeta{Name: "remote-test", Namespace: "default"},
				Spec: sreportalv1alpha1.NetworkFlowDiscoverySpec{
					PortalRef: "test-portal",
					IsRemote:  isRemote,
					RemoteURL: "http://remote:8090",
				},
			},
		}
	}

	Context("when IsRemote is false", func() {
		It("should be a no-op and leave ChainData empty", func() {
			k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).Build()
			handler := nfdchain.NewFetchRemoteGraphHandler(k8sClient, remoteclient.NewCache())
			rc := newRC(false)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			Expect(rc.Data.Nodes).To(BeNil())
			Expect(rc.Data.Edges).To(BeNil())
		})
	})

	Context("when IsRemote is true", func() {
		It("should fail when portal is not found", func() {
			k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).Build()
			handler := nfdchain.NewFetchRemoteGraphHandler(k8sClient, remoteclient.NewCache())
			rc := newRC(true)

			err := handler.Handle(context.Background(), rc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("get portal"))
		})

		It("should fail when portal has no remote configuration", func() {
			portal := &sreportalv1alpha1.Portal{
				ObjectMeta: metav1.ObjectMeta{Name: "test-portal", Namespace: "default"},
				Spec:       sreportalv1alpha1.PortalSpec{Title: "Test"},
			}
			k8sClient := fake.NewClientBuilder().
				WithScheme(newScheme()).
				WithObjects(portal).
				Build()
			handler := nfdchain.NewFetchRemoteGraphHandler(k8sClient, remoteclient.NewCache())
			rc := newRC(true)

			err := handler.Handle(context.Background(), rc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no remote configuration"))
		})
	})
})
