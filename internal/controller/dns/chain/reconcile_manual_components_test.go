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

package dns_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	dnspkg "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/statuspage"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	Expect(sreportalv1alpha1.AddToScheme(s)).To(Succeed())
	return s
}

var _ = Describe("ReconcileManualComponentsHandler", func() {
	var (
		scheme *runtime.Scheme
		portal *sreportalv1alpha1.Portal
	)

	BeforeEach(func() {
		scheme = newTestScheme()
		portal = &sreportalv1alpha1.Portal{
			ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
			Spec:       sreportalv1alpha1.PortalSpec{Title: "Main", Main: true},
		}
	})

	newDNS := func(annotations map[string]string) *sreportalv1alpha1.DNS {
		return &sreportalv1alpha1.DNS{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-dns",
				Namespace:   "default",
				Annotations: annotations,
			},
			Spec: sreportalv1alpha1.DNSSpec{
				PortalRef: "main",
			},
		}
	}

	newRC := func(dns *sreportalv1alpha1.DNS) *reconciler.ReconcileContext[*sreportalv1alpha1.DNS, dnspkg.ChainData] {
		return &reconciler.ReconcileContext[*sreportalv1alpha1.DNS, dnspkg.ChainData]{
			Resource: dns,
		}
	}

	Context("when DNS CR has component annotation", func() {
		It("should create a Component CR", func() {
			dns := newDNS(map[string]string{
				adapter.ComponentAnnotationKey:            "DNS Service",
				adapter.ComponentGroupAnnotationKey:       "Core",
				adapter.ComponentDescriptionAnnotationKey: "Internal DNS",
				adapter.ComponentLinkAnnotationKey:        "https://console.cloud.google.com/dns",
				adapter.ComponentStatusAnnotationKey:      "operational",
			})
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()
			handler := dnspkg.NewReconcileManualComponentsHandler(c)

			rc := newRC(dns)
			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			name := statuspage.GenerateCRName("main", "DNS Service")
			var comp sreportalv1alpha1.Component
			Expect(c.Get(context.Background(), types.NamespacedName{
				Name: name, Namespace: "default",
			}, &comp)).To(Succeed())

			Expect(comp.Spec.DisplayName).To(Equal("DNS Service"))
			Expect(comp.Spec.Group).To(Equal("Core"))
			Expect(comp.Spec.Description).To(Equal("Internal DNS"))
			Expect(comp.Spec.Link).To(Equal("https://console.cloud.google.com/dns"))
			Expect(comp.Spec.PortalRef).To(Equal("main"))
			Expect(comp.Spec.Status).To(Equal(sreportalv1alpha1.ComponentStatusOperational))
			Expect(comp.Labels[adapter.ManagedByLabelKey]).To(Equal(adapter.ManagedByDNSController))
			Expect(comp.Labels[adapter.PortalAnnotationKey]).To(Equal("main"))
		})
	})

	Context("when DNS CR has no component annotation", func() {
		It("should not create any Component CR", func() {
			dns := newDNS(nil)
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()
			handler := dnspkg.NewReconcileManualComponentsHandler(c)

			rc := newRC(dns)
			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var list sreportalv1alpha1.ComponentList
			Expect(c.List(context.Background(), &list)).To(Succeed())
			Expect(list.Items).To(BeEmpty())
		})
	})

	Context("when component annotation is removed", func() {
		It("should delete the previously managed Component", func() {
			name := statuspage.GenerateCRName("main", "DNS Service")
			existing := &sreportalv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
					Labels: map[string]string{
						adapter.ManagedByLabelKey:   adapter.ManagedByDNSController,
						adapter.PortalAnnotationKey: "main",
					},
				},
				Spec: sreportalv1alpha1.ComponentSpec{
					DisplayName: "DNS Service",
					Group:       "Core",
					PortalRef:   "main",
					Status:      sreportalv1alpha1.ComponentStatusOperational,
				},
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, existing).Build()
			handler := dnspkg.NewReconcileManualComponentsHandler(c)

			dns := newDNS(nil) // no component annotation
			rc := newRC(dns)
			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var comp sreportalv1alpha1.Component
			err := c.Get(context.Background(), types.NamespacedName{
				Name: name, Namespace: "default",
			}, &comp)
			Expect(err).To(HaveOccurred(), "managed component should be deleted")
		})
	})

	Context("when updating an existing component", func() {
		It("should sync metadata but not overwrite status", func() {
			name := statuspage.GenerateCRName("main", "DNS Service")
			existing := &sreportalv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
					Labels: map[string]string{
						adapter.ManagedByLabelKey:   adapter.ManagedByDNSController,
						adapter.PortalAnnotationKey: "main",
					},
				},
				Spec: sreportalv1alpha1.ComponentSpec{
					DisplayName: "DNS Service",
					Group:       "Old Group",
					Description: "Old desc",
					PortalRef:   "main",
					Status:      sreportalv1alpha1.ComponentStatusDegraded, // manually changed
				},
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, existing).Build()
			handler := dnspkg.NewReconcileManualComponentsHandler(c)

			dns := newDNS(map[string]string{
				adapter.ComponentAnnotationKey:            "DNS Service",
				adapter.ComponentGroupAnnotationKey:       "New Group",
				adapter.ComponentDescriptionAnnotationKey: "New desc",
				adapter.ComponentStatusAnnotationKey:      "operational",
			})
			rc := newRC(dns)
			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var comp sreportalv1alpha1.Component
			Expect(c.Get(context.Background(), types.NamespacedName{
				Name: name, Namespace: "default",
			}, &comp)).To(Succeed())

			Expect(comp.Spec.Group).To(Equal("New Group"))
			Expect(comp.Spec.Description).To(Equal("New desc"))
			// Status NOT overwritten
			Expect(comp.Spec.Status).To(Equal(sreportalv1alpha1.ComponentStatusDegraded))
		})
	})

	Context("when status page feature is disabled", func() {
		It("should not create a component", func() {
			disabled := false
			portal.Spec.Features = &sreportalv1alpha1.PortalFeatures{StatusPage: &disabled}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()
			handler := dnspkg.NewReconcileManualComponentsHandler(c)

			dns := newDNS(map[string]string{
				adapter.ComponentAnnotationKey:      "DNS Service",
				adapter.ComponentGroupAnnotationKey: "Core",
			})
			rc := newRC(dns)
			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			var list sreportalv1alpha1.ComponentList
			Expect(c.List(context.Background(), &list)).To(Succeed())
			Expect(list.Items).To(BeEmpty())
		})
	})

	Context("when status annotation is empty", func() {
		It("should default to operational", func() {
			dns := newDNS(map[string]string{
				adapter.ComponentAnnotationKey:      "DNS Service",
				adapter.ComponentGroupAnnotationKey: "Core",
			})
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()
			handler := dnspkg.NewReconcileManualComponentsHandler(c)

			rc := newRC(dns)
			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			name := statuspage.GenerateCRName("main", "DNS Service")
			var comp sreportalv1alpha1.Component
			Expect(c.Get(context.Background(), types.NamespacedName{
				Name: name, Namespace: "default",
			}, &comp)).To(Succeed())
			Expect(comp.Spec.Status).To(Equal(sreportalv1alpha1.ComponentStatusOperational))
		})
	})
})
