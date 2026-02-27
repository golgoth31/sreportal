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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	dnspkg "github.com/golgoth31/sreportal/internal/controller/dns"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

var _ = Describe("AggregateDNSRecordsHandler", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(sreportalv1alpha1.AddToScheme(scheme)).To(Succeed())
	})

	buildClient := func(objs ...client.Object) client.Client {
		return fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objs...).
			WithIndex(&sreportalv1alpha1.DNSRecord{}, dnspkg.IndexFieldPortalRef, func(o client.Object) []string {
				rec := o.(*sreportalv1alpha1.DNSRecord)
				if rec.Spec.PortalRef == "" {
					return nil
				}
				return []string{rec.Spec.PortalRef}
			}).
			Build()
	}

	newRC := func(portalRef string) *reconciler.ReconcileContext[*sreportalv1alpha1.DNS] {
		return &reconciler.ReconcileContext[*sreportalv1alpha1.DNS]{
			Resource: &sreportalv1alpha1.DNS{
				ObjectMeta: metav1.ObjectMeta{Name: "test-dns", Namespace: "default"},
				Spec:       sreportalv1alpha1.DNSSpec{PortalRef: portalRef},
			},
			Data: make(map[string]any),
		}
	}

	defaultMapping := &config.GroupMappingConfig{DefaultGroup: "Services"}

	Context("when no DNSRecords exist for the portal", func() {
		It("should store an empty external groups slice", func() {
			c := buildClient()
			handler := dnspkg.NewAggregateDNSRecordsHandler(c, defaultMapping, nil)
			rc := newRC("main")

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			raw, ok := rc.Data[dnspkg.DataKeyExternalGroups]
			Expect(ok).To(BeTrue())
			groups, ok := raw.([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(ok).To(BeTrue())
			Expect(groups).To(BeEmpty())
		})
	})

	Context("when DNSRecords exist for the matching portal", func() {
		It("should aggregate endpoints from all matching DNSRecords into groups", func() {
			now := metav1.Now()
			rec1 := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "main-service", Namespace: "default"},
				Spec:       sreportalv1alpha1.DNSRecordSpec{PortalRef: "main", SourceType: "service"},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, LastSeen: now},
						{DNSName: "web.example.com", RecordType: "A", Targets: []string{"10.0.0.2"}, LastSeen: now},
					},
				},
			}
			rec2 := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "main-ingress", Namespace: "default"},
				Spec:       sreportalv1alpha1.DNSRecordSpec{PortalRef: "main", SourceType: "ingress"},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{DNSName: "app.example.com", RecordType: "CNAME", Targets: []string{"lb.example.com"}, LastSeen: now},
					},
				},
			}

			c := buildClient(rec1, rec2)
			handler := dnspkg.NewAggregateDNSRecordsHandler(c, defaultMapping, nil)
			rc := newRC("main")

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyExternalGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(HaveLen(1)) // All go to "Services" (default group)
			Expect(groups[0].Name).To(Equal("Services"))
			Expect(groups[0].FQDNs).To(HaveLen(3))
			// FQDNs are sorted alphabetically within the group
			Expect(groups[0].FQDNs[0].FQDN).To(Equal("api.example.com"))
			Expect(groups[0].FQDNs[1].FQDN).To(Equal("app.example.com"))
			Expect(groups[0].FQDNs[2].FQDN).To(Equal("web.example.com"))
		})
	})

	Context("when DNSRecords belong to different portals", func() {
		It("should only aggregate endpoints for the DNS resource's portal", func() {
			now := metav1.Now()
			mainRec := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "main-service", Namespace: "default"},
				Spec:       sreportalv1alpha1.DNSRecordSpec{PortalRef: "main", SourceType: "service"},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{DNSName: "main.example.com", RecordType: "A", LastSeen: now},
					},
				},
			}
			otherRec := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "other-service", Namespace: "default"},
				Spec:       sreportalv1alpha1.DNSRecordSpec{PortalRef: "other", SourceType: "service"},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{DNSName: "other.example.com", RecordType: "A", LastSeen: now},
					},
				},
			}

			c := buildClient(mainRec, otherRec)
			handler := dnspkg.NewAggregateDNSRecordsHandler(c, defaultMapping, nil)
			rc := newRC("main")

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyExternalGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(HaveLen(1))
			Expect(groups[0].FQDNs).To(HaveLen(1))
			Expect(groups[0].FQDNs[0].FQDN).To(Equal("main.example.com"))
		})
	})

	Context("with source priority configured", func() {
		It("should use service target when service has higher priority than ingress", func() {
			now := metav1.Now()
			serviceRec := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "main-service", Namespace: "default"},
				Spec:       sreportalv1alpha1.DNSRecordSpec{PortalRef: "main", SourceType: "service"},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, LastSeen: now},
					},
				},
			}
			ingressRec := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "main-ingress", Namespace: "default"},
				Spec:       sreportalv1alpha1.DNSRecordSpec{PortalRef: "main", SourceType: "ingress"},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.99"}, LastSeen: now},
					},
				},
			}

			c := buildClient(serviceRec, ingressRec)
			handler := dnspkg.NewAggregateDNSRecordsHandler(c, defaultMapping, []string{"service", "ingress"})
			rc := newRC("main")

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyExternalGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(HaveLen(1))
			Expect(groups[0].FQDNs).To(HaveLen(1))
			Expect(groups[0].FQDNs[0].FQDN).To(Equal("api.example.com"))
			Expect(groups[0].FQDNs[0].Targets).To(Equal([]string{"10.0.0.1"})) // service wins
		})

		It("should use ingress target when ingress has higher priority", func() {
			now := metav1.Now()
			serviceRec := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "main-service", Namespace: "default"},
				Spec:       sreportalv1alpha1.DNSRecordSpec{PortalRef: "main", SourceType: "service"},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, LastSeen: now},
					},
				},
			}
			ingressRec := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "main-ingress", Namespace: "default"},
				Spec:       sreportalv1alpha1.DNSRecordSpec{PortalRef: "main", SourceType: "ingress"},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.99"}, LastSeen: now},
					},
				},
			}

			c := buildClient(serviceRec, ingressRec)
			handler := dnspkg.NewAggregateDNSRecordsHandler(c, defaultMapping, []string{"ingress", "service"})
			rc := newRC("main")

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyExternalGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(HaveLen(1))
			Expect(groups[0].FQDNs).To(HaveLen(1))
			Expect(groups[0].FQDNs[0].Targets).To(Equal([]string{"10.0.0.99"})) // ingress wins
		})

		It("should merge targets when priority is nil (existing behaviour)", func() {
			now := metav1.Now()
			serviceRec := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "main-service", Namespace: "default"},
				Spec:       sreportalv1alpha1.DNSRecordSpec{PortalRef: "main", SourceType: "service"},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, LastSeen: now},
					},
				},
			}
			ingressRec := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "main-ingress", Namespace: "default"},
				Spec:       sreportalv1alpha1.DNSRecordSpec{PortalRef: "main", SourceType: "ingress"},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.99"}, LastSeen: now},
					},
				},
			}

			c := buildClient(serviceRec, ingressRec)
			handler := dnspkg.NewAggregateDNSRecordsHandler(c, defaultMapping, nil) // empty priority
			rc := newRC("main")

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyExternalGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(HaveLen(1))
			Expect(groups[0].FQDNs).To(HaveLen(1))
			// Both targets merged
			Expect(groups[0].FQDNs[0].Targets).To(ConsistOf("10.0.0.1", "10.0.0.99"))
		})
	})

	Context("when a DNSRecord has no endpoints", func() {
		It("should produce an empty groups slice", func() {
			emptyRec := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "empty", Namespace: "default"},
				Spec:       sreportalv1alpha1.DNSRecordSpec{PortalRef: "main"},
				Status:     sreportalv1alpha1.DNSRecordStatus{},
			}

			c := buildClient(emptyRec)
			handler := dnspkg.NewAggregateDNSRecordsHandler(c, defaultMapping, nil)
			rc := newRC("main")

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyExternalGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(BeEmpty())
		})
	})
})
