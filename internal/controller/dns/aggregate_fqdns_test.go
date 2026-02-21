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

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	dnspkg "github.com/golgoth31/sreportal/internal/controller/dns"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

var _ = Describe("AggregateFQDNsHandler", func() {
	var handler *dnspkg.AggregateFQDNsHandler

	BeforeEach(func() {
		handler = dnspkg.NewAggregateFQDNsHandler()
	})

	newRC := func(external []sreportalv1alpha1.FQDNGroupStatus, manual []sreportalv1alpha1.DNSGroup) *reconciler.ReconcileContext[*sreportalv1alpha1.DNS] {
		data := make(map[string]any)
		if external != nil {
			data[dnspkg.DataKeyExternalGroups] = external
		}
		if manual != nil {
			data[dnspkg.DataKeyManualGroups] = manual
		}
		return &reconciler.ReconcileContext[*sreportalv1alpha1.DNS]{
			Resource: &sreportalv1alpha1.DNS{},
			Data:     data,
		}
	}

	Context("when only external groups are present", func() {
		It("should pass external groups through unchanged", func() {
			external := []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "APIs",
					Source: dnspkg.SourceExternalDNS,
					FQDNs:  []sreportalv1alpha1.FQDNStatus{{FQDN: "api.example.com", RecordType: "A"}},
				},
			}
			rc := newRC(external, nil)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(HaveLen(1))
			Expect(groups[0].Name).To(Equal("APIs"))
			Expect(groups[0].Source).To(Equal(dnspkg.SourceExternalDNS))
		})
	})

	Context("when only manual groups are present", func() {
		It("should convert manual entries to FQDNGroupStatus", func() {
			manual := []sreportalv1alpha1.DNSGroup{
				{
					Name:        "Internal",
					Description: "Internal services",
					Entries: []sreportalv1alpha1.DNSEntry{
						{FQDN: "db.internal.com", Description: "Database"},
						{FQDN: "cache.internal.com"},
					},
				},
			}
			rc := newRC(nil, manual)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(HaveLen(1))
			Expect(groups[0].Name).To(Equal("Internal"))
			Expect(groups[0].Description).To(Equal("Internal services"))
			Expect(groups[0].Source).To(Equal(dnspkg.SourceManual))
			Expect(groups[0].FQDNs).To(HaveLen(2))
			// FQDNs within the group are sorted alphabetically
			Expect(groups[0].FQDNs[0].FQDN).To(Equal("cache.internal.com"))
			Expect(groups[0].FQDNs[1].FQDN).To(Equal("db.internal.com"))
			Expect(groups[0].FQDNs[1].Description).To(Equal("Database"))
		})
	})

	Context("when manual and external groups have different names", func() {
		It("should merge both into the output, sorted by name", func() {
			external := []sreportalv1alpha1.FQDNGroupStatus{
				{Name: "Web", Source: dnspkg.SourceExternalDNS, FQDNs: []sreportalv1alpha1.FQDNStatus{{FQDN: "web.example.com"}}},
			}
			manual := []sreportalv1alpha1.DNSGroup{
				{Name: "Infra", Entries: []sreportalv1alpha1.DNSEntry{{FQDN: "db.example.com"}}},
			}
			rc := newRC(external, manual)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(HaveLen(2))
			Expect(groups[0].Name).To(Equal("Infra"))
			Expect(groups[0].Source).To(Equal(dnspkg.SourceManual))
			Expect(groups[1].Name).To(Equal("Web"))
			Expect(groups[1].Source).To(Equal(dnspkg.SourceExternalDNS))
		})
	})

	Context("when manual and external groups share a name", func() {
		It("should merge FQDNs from both sources into a single group", func() {
			external := []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Services",
					Source: dnspkg.SourceExternalDNS,
					FQDNs:  []sreportalv1alpha1.FQDNStatus{{FQDN: "auto.example.com", RecordType: "A"}},
				},
			}
			manual := []sreportalv1alpha1.DNSGroup{
				{
					Name:        "Services",
					Description: "Curated services",
					Entries:     []sreportalv1alpha1.DNSEntry{{FQDN: "curated.example.com", Description: "Manual entry"}},
				},
			}
			rc := newRC(external, manual)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(HaveLen(1))
			Expect(groups[0].Name).To(Equal("Services"))
			// Source stays external-dns (manual description is applied)
			Expect(groups[0].Source).To(Equal(dnspkg.SourceExternalDNS))
			Expect(groups[0].Description).To(Equal("Curated services"))
			// Both FQDNs present, sorted alphabetically
			Expect(groups[0].FQDNs).To(HaveLen(2))
			Expect(groups[0].FQDNs[0].FQDN).To(Equal("auto.example.com"))
			Expect(groups[0].FQDNs[1].FQDN).To(Equal("curated.example.com"))
			Expect(groups[0].FQDNs[1].Description).To(Equal("Manual entry"))
		})
	})

	Context("when no Data keys are present", func() {
		It("should produce an empty aggregated groups slice", func() {
			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DNS]{
				Resource: &sreportalv1alpha1.DNS{},
				Data:     make(map[string]any),
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(BeEmpty())
		})
	})

	Context("when aggregated groups are sorted", func() {
		It("should always produce alphabetically sorted groups", func() {
			external := []sreportalv1alpha1.FQDNGroupStatus{
				{Name: "Zebra", Source: dnspkg.SourceExternalDNS},
				{Name: "Alpha", Source: dnspkg.SourceExternalDNS},
				{Name: "Mango", Source: dnspkg.SourceExternalDNS},
			}
			rc := newRC(external, nil)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(HaveLen(3))
			Expect(groups[0].Name).To(Equal("Alpha"))
			Expect(groups[1].Name).To(Equal("Mango"))
			Expect(groups[2].Name).To(Equal("Zebra"))
		})
	})
})
