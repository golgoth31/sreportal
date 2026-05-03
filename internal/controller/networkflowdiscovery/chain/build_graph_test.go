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

package chain_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	nfdchain "github.com/golgoth31/sreportal/internal/controller/networkflowdiscovery/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

var _ = Describe("BuildGraphHandler", func() {
	newRC := func(isRemote bool, namespaces ...string) *reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, nfdchain.ChainData] {
		return &reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, nfdchain.ChainData]{
			Resource: &sreportalv1alpha1.NetworkFlowDiscovery{
				ObjectMeta: metav1.ObjectMeta{Name: "test-nfd", Namespace: tNsDefault},
				Spec: sreportalv1alpha1.NetworkFlowDiscoverySpec{
					PortalRef:  "main",
					IsRemote:   isRemote,
					Namespaces: namespaces,
				},
			},
		}
	}

	Context("when IsRemote is true", func() {
		It("should be a no-op and leave ChainData empty", func() {
			k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).Build()
			handler := nfdchain.NewBuildGraphHandler(k8sClient)
			rc := newRC(true)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			Expect(rc.Data.Nodes).To(BeNil())
			Expect(rc.Data.Edges).To(BeNil())
		})
	})

	Context("when no NetworkPolicies exist", func() {
		It("should produce empty graph", func() {
			k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).Build()
			handler := nfdchain.NewBuildGraphHandler(k8sClient)
			rc := newRC(false)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			Expect(rc.Data.Nodes).To(BeEmpty())
			Expect(rc.Data.Edges).To(BeEmpty())
		})
	})

	Context("when ingress policies exist", func() {
		It("should build nodes and edges from ingress rules", func() {
			// Create two apps: api and web. web-ingress-policy allows ingress from api.
			apiEgressPolicy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "api-egress-policy", Namespace: tCore},
				Spec: networkingv1.NetworkPolicySpec{
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				},
			}
			webIngressPolicy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "web-ingress-policy", Namespace: tCore},
				Spec: networkingv1.NetworkPolicySpec{
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									PodSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app.kubernetes.io/name",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{tNameAPI},
											},
										},
									},
								},
							},
						},
					},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(newScheme()).
				WithObjects(apiEgressPolicy, webIngressPolicy).
				Build()
			handler := nfdchain.NewBuildGraphHandler(k8sClient)
			rc := newRC(false)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			Expect(rc.Data.Nodes).To(HaveLen(2))
			Expect(rc.Data.Edges).To(HaveLen(1))

			// Edge: api -> web (internal, same namespace)
			Expect(rc.Data.Edges[0].From).To(ContainSubstring(tNameAPI))
			Expect(rc.Data.Edges[0].To).To(ContainSubstring("web"))
			Expect(rc.Data.Edges[0].EdgeType).To(Equal("internal"))
		})

		It("should detect cross-namespace edges", func() {
			// api in "backend" namespace, web-ingress-policy in "frontend" allows ingress from api
			apiEgressPolicy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "api-egress-policy", Namespace: "backend"},
				Spec: networkingv1.NetworkPolicySpec{
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				},
			}
			webIngressPolicy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "web-ingress-policy", Namespace: "frontend"},
				Spec: networkingv1.NetworkPolicySpec{
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									PodSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app.kubernetes.io/name",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{tNameAPI},
											},
										},
									},
								},
							},
						},
					},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(newScheme()).
				WithObjects(apiEgressPolicy, webIngressPolicy).
				Build()
			handler := nfdchain.NewBuildGraphHandler(k8sClient)
			rc := newRC(false)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			Expect(rc.Data.Edges).To(HaveLen(1))
			Expect(rc.Data.Edges[0].EdgeType).To(Equal("cross-ns"))
		})

		It("should detect cron edges from basename selector", func() {
			cronIngressPolicy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "api-ingress-policy", Namespace: tCore},
				Spec: networkingv1.NetworkPolicySpec{
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									PodSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "basename",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"sync-job"},
											},
										},
									},
								},
							},
						},
					},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(newScheme()).
				WithObjects(cronIngressPolicy).
				Build()
			handler := nfdchain.NewBuildGraphHandler(k8sClient)
			rc := newRC(false)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			Expect(rc.Data.Nodes).To(HaveLen(2))
			Expect(rc.Data.Edges).To(HaveLen(1))
			Expect(rc.Data.Edges[0].EdgeType).To(Equal("cron"))
		})
	})
})
