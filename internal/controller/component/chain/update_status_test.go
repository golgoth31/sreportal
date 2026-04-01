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

package component_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	componentchain "github.com/golgoth31/sreportal/internal/controller/component/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

var _ = Describe("UpdateStatusHandler", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(sreportalv1alpha1.AddToScheme(scheme)).To(Succeed())
	})

	buildClient := func(obj client.Object) client.Client {
		return fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(obj).
			WithStatusSubresource(obj).
			Build()
	}

	newComponent := func() *sreportalv1alpha1.Component {
		return &sreportalv1alpha1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-component",
				Namespace: "default",
			},
			Spec: sreportalv1alpha1.ComponentSpec{
				PortalRef: "my-portal",
				Status:    sreportalv1alpha1.ComponentStatusOperational,
			},
		}
	}

	Context("requeue duration", func() {
		It("should schedule requeue at next UTC midnight", func() {
			comp := newComponent()
			c := buildClient(comp)
			fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
			handler := componentchain.NewUpdateStatusHandler(c, nil, func() time.Time { return fixedNow })

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Component, componentchain.ChainData]{
				Resource: comp,
				Data: componentchain.ChainData{
					ComputedStatus: sreportalv1alpha1.ComputedStatusOperational,
				},
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			Expect(rc.Result.RequeueAfter).To(Equal(12 * time.Hour))
		})
	})

	Context("when status changes", func() {
		It("should set LastStatusChange and persist", func() {
			comp := newComponent()
			comp.Status.ComputedStatus = sreportalv1alpha1.ComputedStatusOperational
			c := buildClient(comp)
			fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
			handler := componentchain.NewUpdateStatusHandler(c, nil, func() time.Time { return fixedNow })

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Component, componentchain.ChainData]{
				Resource: comp,
				Data: componentchain.ChainData{
					ComputedStatus: sreportalv1alpha1.ComputedStatusDegraded,
				},
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var updated sreportalv1alpha1.Component
			Expect(c.Get(context.Background(), types.NamespacedName{Name: "test-component", Namespace: "default"}, &updated)).To(Succeed())
			Expect(updated.Status.ComputedStatus).To(Equal(sreportalv1alpha1.ComputedStatusDegraded))
			Expect(updated.Status.LastStatusChange).NotTo(BeNil())
		})
	})

	Context("portal label", func() {
		It("should add the sreportal.io/portal label", func() {
			comp := newComponent()
			c := buildClient(comp)
			fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
			handler := componentchain.NewUpdateStatusHandler(c, nil, func() time.Time { return fixedNow })

			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Component, componentchain.ChainData]{
				Resource: comp,
				Data: componentchain.ChainData{
					ComputedStatus: sreportalv1alpha1.ComputedStatusOperational,
				},
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var updated sreportalv1alpha1.Component
			Expect(c.Get(context.Background(), types.NamespacedName{Name: "test-component", Namespace: "default"}, &updated)).To(Succeed())
			Expect(updated.Labels).To(HaveKeyWithValue("sreportal.io/portal", "my-portal"))
		})
	})
})
