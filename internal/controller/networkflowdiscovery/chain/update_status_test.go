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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	nfdchain "github.com/golgoth31/sreportal/internal/controller/networkflowdiscovery/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

var _ = Describe("UpdateStatusHandler", func() {
	buildClient := func(obj client.Object) client.Client {
		return fake.NewClientBuilder().
			WithScheme(newScheme()).
			WithObjects(obj).
			WithStatusSubresource(
				obj,
				&sreportalv1alpha1.FlowNodeSet{},
				&sreportalv1alpha1.FlowEdgeSet{},
			).
			Build()
	}

	newNFD := func() *sreportalv1alpha1.NetworkFlowDiscovery {
		return &sreportalv1alpha1.NetworkFlowDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tNameTestNFD,
				Namespace: tNsDefault,
				UID:       "test-uid",
			},
			Spec: sreportalv1alpha1.NetworkFlowDiscoverySpec{
				PortalRef: "main",
			},
		}
	}

	Context("when nodes and edges are present in Data", func() {
		It("should create FlowNodeSet and FlowEdgeSet and update NFD status", func() {
			nfd := newNFD()
			c := buildClient(nfd)
			handler := nfdchain.NewUpdateStatusHandler(c)

			nodes := []sreportalv1alpha1.FlowNode{
				{ID: "service:core:api", Label: tNameAPI, Namespace: tCore, NodeType: tNodeTypeService, Group: tCore},
				{ID: "service:core:web", Label: "web", Namespace: tCore, NodeType: tNodeTypeService, Group: tCore},
			}
			edges := []sreportalv1alpha1.FlowEdge{
				{From: "service:core:api", To: "service:core:web", EdgeType: "internal"},
			}

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, nfdchain.ChainData]{
				Resource: nfd,
				Data: nfdchain.ChainData{
					Nodes: nodes,
					Edges: edges,
				},
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			// Verify FlowNodeSet was created with correct status
			var nodeSet sreportalv1alpha1.FlowNodeSet
			Expect(c.Get(context.Background(), types.NamespacedName{Name: tNameNFDNodes, Namespace: tNsDefault}, &nodeSet)).To(Succeed())
			Expect(nodeSet.Spec.DiscoveryRef).To(Equal(tNameTestNFD))
			Expect(nodeSet.Status.Nodes).To(HaveLen(2))
			Expect(nodeSet.Status.Nodes[0].ID).To(Equal("service:core:api"))

			// Verify FlowEdgeSet was created with correct status
			var edgeSet sreportalv1alpha1.FlowEdgeSet
			Expect(c.Get(context.Background(), types.NamespacedName{Name: tNameNFDEdges, Namespace: tNsDefault}, &edgeSet)).To(Succeed())
			Expect(edgeSet.Spec.DiscoveryRef).To(Equal(tNameTestNFD))
			Expect(edgeSet.Status.Edges).To(HaveLen(1))
			Expect(edgeSet.Status.Edges[0].From).To(Equal("service:core:api"))

			// Verify NFD status was updated
			var updated sreportalv1alpha1.NetworkFlowDiscovery
			Expect(c.Get(context.Background(), types.NamespacedName{Name: tNameTestNFD, Namespace: tNsDefault}, &updated)).To(Succeed())
			Expect(updated.Status.NodeCount).To(Equal(2))
			Expect(updated.Status.EdgeCount).To(Equal(1))
			Expect(updated.Status.LastReconcileTime).NotTo(BeNil())

			Expect(updated.Status.Conditions).NotTo(BeEmpty())
			readyCond := findCondition(updated.Status.Conditions, nfdchain.ConditionTypeReady)
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionTrue))
			Expect(readyCond.Message).To(ContainSubstring("2 nodes"))
			Expect(readyCond.Message).To(ContainSubstring("1 edges"))
		})
	})

	Context("when Data has empty nodes and edges", func() {
		It("should create FlowNodeSet and FlowEdgeSet with empty status", func() {
			nfd := newNFD()
			c := buildClient(nfd)
			handler := nfdchain.NewUpdateStatusHandler(c)

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, nfdchain.ChainData]{
				Resource: nfd,
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var nodeSet sreportalv1alpha1.FlowNodeSet
			Expect(c.Get(context.Background(), types.NamespacedName{Name: tNameNFDNodes, Namespace: tNsDefault}, &nodeSet)).To(Succeed())
			Expect(nodeSet.Status.Nodes).To(BeEmpty())

			var edgeSet sreportalv1alpha1.FlowEdgeSet
			Expect(c.Get(context.Background(), types.NamespacedName{Name: tNameNFDEdges, Namespace: tNsDefault}, &edgeSet)).To(Succeed())
			Expect(edgeSet.Status.Edges).To(BeEmpty())

			var updated sreportalv1alpha1.NetworkFlowDiscovery
			Expect(c.Get(context.Background(), types.NamespacedName{Name: tNameTestNFD, Namespace: tNsDefault}, &updated)).To(Succeed())
			Expect(updated.Status.NodeCount).To(Equal(0))
			Expect(updated.Status.EdgeCount).To(Equal(0))
		})
	})

	Context("when FlowNodeSet and FlowEdgeSet already exist", func() {
		It("should update their status with new data", func() {
			nfd := newNFD()
			existingNodeSet := &sreportalv1alpha1.FlowNodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tNameNFDNodes,
					Namespace: tNsDefault,
				},
				Spec: sreportalv1alpha1.FlowNodeSetSpec{DiscoveryRef: tNameTestNFD},
				Status: sreportalv1alpha1.FlowNodeSetStatus{
					Nodes: []sreportalv1alpha1.FlowNode{
						{ID: "old-node", Label: tOldVal, Namespace: tOldVal, NodeType: tNodeTypeService, Group: tOldVal},
					},
				},
			}
			existingEdgeSet := &sreportalv1alpha1.FlowEdgeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tNameNFDEdges,
					Namespace: tNsDefault,
				},
				Spec: sreportalv1alpha1.FlowEdgeSetSpec{DiscoveryRef: tNameTestNFD},
				Status: sreportalv1alpha1.FlowEdgeSetStatus{
					Edges: []sreportalv1alpha1.FlowEdge{
						{From: "old-a", To: "old-b", EdgeType: "internal"},
					},
				},
			}

			c := fake.NewClientBuilder().
				WithScheme(newScheme()).
				WithObjects(nfd, existingNodeSet, existingEdgeSet).
				WithStatusSubresource(nfd, existingNodeSet, existingEdgeSet).
				Build()
			handler := nfdchain.NewUpdateStatusHandler(c)

			newNodes := []sreportalv1alpha1.FlowNode{
				{ID: "service:core:new-api", Label: "new-api", Namespace: tCore, NodeType: tNodeTypeService, Group: tCore},
			}
			newEdges := []sreportalv1alpha1.FlowEdge{
				{From: "service:core:new-api", To: "external:core:db.example.com", EdgeType: "database"},
			}

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, nfdchain.ChainData]{
				Resource: nfd,
				Data: nfdchain.ChainData{
					Nodes: newNodes,
					Edges: newEdges,
				},
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var nodeSet sreportalv1alpha1.FlowNodeSet
			Expect(c.Get(context.Background(), types.NamespacedName{Name: tNameNFDNodes, Namespace: tNsDefault}, &nodeSet)).To(Succeed())
			Expect(nodeSet.Status.Nodes).To(HaveLen(1))
			Expect(nodeSet.Status.Nodes[0].ID).To(Equal("service:core:new-api"))

			var edgeSet sreportalv1alpha1.FlowEdgeSet
			Expect(c.Get(context.Background(), types.NamespacedName{Name: tNameNFDEdges, Namespace: tNsDefault}, &edgeSet)).To(Succeed())
			Expect(edgeSet.Status.Edges).To(HaveLen(1))
			Expect(edgeSet.Status.Edges[0].EdgeType).To(Equal("database"))
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
