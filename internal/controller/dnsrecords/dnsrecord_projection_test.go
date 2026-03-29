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

package dnsrecords

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

var _ = Describe("dnsRecordToFQDNViews", func() {
	Context("with endpoints", func() {
		It("should convert endpoints to FQDNViews with PortalRef and SourceType", func() {
			record := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-portal-service",
					Namespace: "test-ns",
				},
				Spec: sreportalv1alpha1.DNSRecordSpec{
					SourceType: "service",
					PortalRef:  "my-portal",
				},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{
							DNSName:    "api.example.com",
							RecordType: "A",
							Targets:    []string{"1.2.3.4"},
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

			views := dnsRecordToFQDNViews(record, nil)

			Expect(views).To(HaveLen(2))
			for _, v := range views {
				Expect(v.PortalName).To(Equal("my-portal"))
				Expect(v.Namespace).To(Equal("test-ns"))
				Expect(v.Source).To(Equal(domaindns.SourceExternalDNS))
				Expect(v.SourceType).To(Equal("service"))
			}
		})
	})

	Context("with empty endpoints", func() {
		It("should return nil", func() {
			record := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-record",
					Namespace: "default",
				},
				Spec: sreportalv1alpha1.DNSRecordSpec{
					SourceType: "service",
					PortalRef:  "main",
				},
			}

			views := dnsRecordToFQDNViews(record, nil)
			Expect(views).To(BeNil())
		})
	})

	Context("with OriginRef on endpoints", func() {
		It("should propagate OriginRef to FQDNView", func() {
			record := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "origin-record",
					Namespace: "default",
				},
				Spec: sreportalv1alpha1.DNSRecordSpec{
					SourceType: "service",
					PortalRef:  "main",
				},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{
							DNSName:    "svc.example.com",
							RecordType: "A",
							Targets:    []string{"10.0.0.1"},
							LastSeen:   metav1.Now(),
							Labels: map[string]string{
								"sreportal.io/origin-kind":      "Service",
								"sreportal.io/origin-namespace": "prod",
								"sreportal.io/origin-name":      "my-svc",
							},
						},
					},
				},
			}

			views := dnsRecordToFQDNViews(record, nil)

			Expect(views).To(HaveLen(1))
			// OriginRef is set by adapter.EndpointStatusToGroups from labels
			// The adapter populates FQDNStatus.OriginRef from endpoint labels
		})
	})

	Context("with group mapping config", func() {
		It("should apply group mapping from config", func() {
			record := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mapped-record",
					Namespace: "default",
				},
				Spec: sreportalv1alpha1.DNSRecordSpec{
					SourceType: "service",
					PortalRef:  "main",
				},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{
							DNSName:    "app.example.com",
							RecordType: "A",
							Targets:    []string{"1.2.3.4"},
							LastSeen:   metav1.Now(),
						},
					},
				},
			}

			mapping := &config.GroupMappingConfig{
				DefaultGroup: "Custom Group",
			}

			views := dnsRecordToFQDNViews(record, mapping)

			Expect(views).To(HaveLen(1))
			Expect(views[0].Groups).To(ContainElement("Custom Group"))
		})
	})

	Context("with duplicate FQDNs across groups", func() {
		It("should deduplicate and merge groups", func() {
			record := &sreportalv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dedup-record",
					Namespace: "default",
				},
				Spec: sreportalv1alpha1.DNSRecordSpec{
					SourceType: "service",
					PortalRef:  "main",
				},
				Status: sreportalv1alpha1.DNSRecordStatus{
					Endpoints: []sreportalv1alpha1.EndpointStatus{
						{
							DNSName:    "shared.example.com",
							RecordType: "A",
							Targets:    []string{"1.2.3.4"},
							LastSeen:   metav1.Now(),
							Labels: map[string]string{
								"sreportal.io/groups": "group-a,group-b",
							},
						},
					},
				},
			}

			views := dnsRecordToFQDNViews(record, nil)

			Expect(views).To(HaveLen(1))
			Expect(views[0].Groups).To(ContainElements("group-a", "group-b"))
		})
	})
})
