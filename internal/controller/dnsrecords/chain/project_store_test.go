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

package chain

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"sigs.k8s.io/external-dns/endpoint"
)

const (
	tPortalMain = "main"
	tNsDefault  = "default"
	tSrcService = "service"
	tPortalMy   = "my-portal"
	tIP1234     = "1.2.3.4"
)

func TestChain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DNSRecord Chain Suite")
}

var _ = Describe("DNSRecordToFQDNViews", func() {
	Context("with endpoints", func() {
		It("should convert endpoints to FQDNViews with PortalRef and SourceType", func() {
			record := &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-portal-service",
					Namespace: "test-ns",
				},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:     v1alpha2.DNSRecordOriginAuto,
					SourceType: tSrcService,
					PortalRef:  tPortalMy,
				},
				Status: v1alpha2.DNSRecordStatus{
					Endpoints: []v1alpha2.EndpointStatus{
						{
							DNSName:    "api.example.com",
							RecordType: "A",
							Targets:    []string{tIP1234},
							LastSeen:   metav1.Now(),
						},
						{
							DNSName:    "web.example.com",
							RecordType: "CNAME",
							Targets:    []string{"lb.example.com"},
							LastSeen:   metav1.Now(),
						},
					},
				},
			}

			views := DNSRecordToFQDNViews(record, nil)

			Expect(views).To(HaveLen(2))
			for _, v := range views {
				Expect(v.FirstPortal()).To(Equal(tPortalMy))
				Expect(v.Namespace).To(Equal("test-ns"))
				Expect(v.Source).To(Equal(domaindns.SourceExternalDNS))
				Expect(v.SourceType).To(Equal(tSrcService))
			}
		})
	})

	Context("with empty endpoints", func() {
		It("should return nil", func() {
			record := &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-record",
					Namespace: tNsDefault,
				},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:     v1alpha2.DNSRecordOriginAuto,
					SourceType: tSrcService,
					PortalRef:  tPortalMain,
				},
			}

			views := DNSRecordToFQDNViews(record, nil)
			Expect(views).To(BeNil())
		})
	})

	Context("with OriginRef on endpoints", func() {
		It("should propagate OriginRef to FQDNView", func() {
			record := &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "origin-record", Namespace: tNsDefault},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:     v1alpha2.DNSRecordOriginAuto,
					SourceType: tSrcService,
					PortalRef:  tPortalMain,
				},
				Status: v1alpha2.DNSRecordStatus{
					Endpoints: []v1alpha2.EndpointStatus{
						{
							DNSName:    "svc.example.com",
							RecordType: "A",
							Targets:    []string{"10.0.0.1"},
							LastSeen:   metav1.Now(),
							Labels: map[string]string{
								endpoint.ResourceLabelKey: "service/prod/my-svc",
							},
						},
					},
				},
			}

			views := DNSRecordToFQDNViews(record, nil)

			Expect(views).To(HaveLen(1))
			Expect(views[0].OriginRef).NotTo(BeNil())
			Expect(views[0].OriginRef.Kind()).To(Equal("service"))
			Expect(views[0].OriginRef.Namespace()).To(Equal("prod"))
			Expect(views[0].OriginRef.Name()).To(Equal("my-svc"))
		})
	})

	Context("with group mapping config", func() {
		It("should apply group mapping from config", func() {
			record := &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mapped-record",
					Namespace: tNsDefault,
				},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:     v1alpha2.DNSRecordOriginAuto,
					SourceType: tSrcService,
					PortalRef:  tPortalMain,
				},
				Status: v1alpha2.DNSRecordStatus{
					Endpoints: []v1alpha2.EndpointStatus{
						{
							DNSName:    "app.example.com",
							RecordType: "A",
							Targets:    []string{tIP1234},
							LastSeen:   metav1.Now(),
						},
					},
				},
			}

			mapping := &v1alpha2.GroupMappingSpec{
				DefaultGroup: "Custom Group",
			}

			views := DNSRecordToFQDNViews(record, mapping)

			Expect(views).To(HaveLen(1))
			Expect(views[0].Groups).To(ContainElement("Custom Group"))
		})
	})

	Context("with duplicate FQDNs across groups", func() {
		It("should deduplicate and merge groups", func() {
			record := &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dedup-record",
					Namespace: tNsDefault,
				},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:     v1alpha2.DNSRecordOriginAuto,
					SourceType: tSrcService,
					PortalRef:  tPortalMain,
				},
				Status: v1alpha2.DNSRecordStatus{
					Endpoints: []v1alpha2.EndpointStatus{
						{
							DNSName:    "shared.example.com",
							RecordType: "A",
							Targets:    []string{tIP1234},
							LastSeen:   metav1.Now(),
							Labels: map[string]string{
								"sreportal.io/groups": "group-a,group-b",
							},
						},
					},
				},
			}

			views := DNSRecordToFQDNViews(record, nil)

			Expect(views).To(HaveLen(1))
			Expect(views[0].Groups).To(ContainElements("group-a", "group-b"))
		})
	})

	Context("with manual origin", func() {
		It("should set Source to SourceManual", func() {
			record := &v1alpha2.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "main-manual", Namespace: tNsDefault},
				Spec: v1alpha2.DNSRecordSpec{
					Origin:    v1alpha2.DNSRecordOriginManual,
					PortalRef: tPortalMain,
				},
				Status: v1alpha2.DNSRecordStatus{
					Endpoints: []v1alpha2.EndpointStatus{
						{
							DNSName:    "api.example.com",
							RecordType: "A",
							Targets:    []string{tIP1234},
							LastSeen:   metav1.Now(),
						},
					},
				},
			}
			views := DNSRecordToFQDNViews(record, nil)
			Expect(views).To(HaveLen(1))
			Expect(views[0].Source).To(Equal(domaindns.SourceManual))
		})
	})
})
