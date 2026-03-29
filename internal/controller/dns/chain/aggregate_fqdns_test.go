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
	dnspkg "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

var _ = Describe("AggregateFQDNsHandler", func() {
	var handler *dnspkg.AggregateFQDNsHandler

	BeforeEach(func() {
		handler = dnspkg.NewAggregateFQDNsHandler()
	})

	newRC := func(manual []sreportalv1alpha1.DNSGroup) *reconciler.ReconcileContext[*sreportalv1alpha1.DNS, dnspkg.ChainData] {
		return &reconciler.ReconcileContext[*sreportalv1alpha1.DNS, dnspkg.ChainData]{
			Resource: &sreportalv1alpha1.DNS{},
			Data: dnspkg.ChainData{
				ManualGroups: manual,
			},
		}
	}

	Context("when manual groups are present", func() {
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
			rc := newRC(manual)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data.AggregatedGroups
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

	Context("when multiple manual groups are present", func() {
		It("should produce sorted output", func() {
			manual := []sreportalv1alpha1.DNSGroup{
				{Name: "Zebra", Entries: []sreportalv1alpha1.DNSEntry{{FQDN: "z.example.com"}}},
				{Name: "Alpha", Entries: []sreportalv1alpha1.DNSEntry{{FQDN: "a.example.com"}}},
			}
			rc := newRC(manual)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data.AggregatedGroups
			Expect(groups).To(HaveLen(2))
			Expect(groups[0].Name).To(Equal("Alpha"))
			Expect(groups[0].Source).To(Equal(dnspkg.SourceManual))
			Expect(groups[1].Name).To(Equal("Zebra"))
			Expect(groups[1].Source).To(Equal(dnspkg.SourceManual))
		})
	})

	Context("when no manual groups are present", func() {
		It("should produce an empty aggregated groups slice", func() {
			rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DNS, dnspkg.ChainData]{
				Resource: &sreportalv1alpha1.DNS{},
			}

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			Expect(rc.Data.AggregatedGroups).To(BeEmpty())
		})
	})
})
