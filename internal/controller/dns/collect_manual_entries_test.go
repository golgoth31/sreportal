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

var _ = Describe("CollectManualEntriesHandler", func() {
	var (
		handler *dnspkg.CollectManualEntriesHandler
		rc      *reconciler.ReconcileContext[*sreportalv1alpha1.DNS]
	)

	BeforeEach(func() {
		handler = dnspkg.NewCollectManualEntriesHandler()
	})

	newRC := func(dns *sreportalv1alpha1.DNS) *reconciler.ReconcileContext[*sreportalv1alpha1.DNS] {
		return &reconciler.ReconcileContext[*sreportalv1alpha1.DNS]{
			Resource: dns,
			Data:     make(map[string]any),
		}
	}

	Context("when the DNS spec has no groups", func() {
		It("should store an empty slice in Data", func() {
			rc = newRC(&sreportalv1alpha1.DNS{})

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			raw, ok := rc.Data[dnspkg.DataKeyManualGroups]
			Expect(ok).To(BeTrue())
			groups, ok := raw.([]sreportalv1alpha1.DNSGroup)
			Expect(ok).To(BeTrue())
			Expect(groups).To(BeNil())
		})
	})

	Context("when the DNS spec has manual groups", func() {
		It("should store all groups in Data unchanged", func() {
			dns := &sreportalv1alpha1.DNS{
				Spec: sreportalv1alpha1.DNSSpec{
					Groups: []sreportalv1alpha1.DNSGroup{
						{
							Name:        "Infra",
							Description: "Infrastructure services",
							Entries: []sreportalv1alpha1.DNSEntry{
								{FQDN: "db.example.com", Description: "Database"},
								{FQDN: "cache.example.com"},
							},
						},
						{
							Name: "Web",
							Entries: []sreportalv1alpha1.DNSEntry{
								{FQDN: "www.example.com"},
							},
						},
					},
				},
			}
			rc = newRC(dns)

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			raw, ok := rc.Data[dnspkg.DataKeyManualGroups]
			Expect(ok).To(BeTrue())
			groups, ok := raw.([]sreportalv1alpha1.DNSGroup)
			Expect(ok).To(BeTrue())
			Expect(groups).To(HaveLen(2))
			Expect(groups[0].Name).To(Equal("Infra"))
			Expect(groups[0].Description).To(Equal("Infrastructure services"))
			Expect(groups[0].Entries).To(HaveLen(2))
			Expect(groups[1].Name).To(Equal("Web"))
		})
	})

	Context("when called multiple times", func() {
		It("should overwrite the previous Data value", func() {
			rc = newRC(&sreportalv1alpha1.DNS{
				Spec: sreportalv1alpha1.DNSSpec{
					Groups: []sreportalv1alpha1.DNSGroup{{Name: "First"}},
				},
			})

			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			// Simulate a second call (shouldn't happen in practice, but confirms idempotency)
			rc.Resource.Spec.Groups = []sreportalv1alpha1.DNSGroup{{Name: "Second"}}
			Expect(handler.Handle(context.Background(), rc)).To(Succeed())

			groups := rc.Data[dnspkg.DataKeyManualGroups].([]sreportalv1alpha1.DNSGroup)
			Expect(groups).To(HaveLen(1))
			Expect(groups[0].Name).To(Equal("Second"))
		})
	})
})
