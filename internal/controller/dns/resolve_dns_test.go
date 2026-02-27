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
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	dnspkg "github.com/golgoth31/sreportal/internal/controller/dns"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// fakeResolver implements domain/dns.Resolver for testing.
type fakeResolver struct {
	hosts    map[string][]string
	cnames   map[string]string
	hostErr  map[string]error
	cnameErr map[string]error
}

func newFakeResolver() *fakeResolver {
	return &fakeResolver{
		hosts:    make(map[string][]string),
		cnames:   make(map[string]string),
		hostErr:  make(map[string]error),
		cnameErr: make(map[string]error),
	}
}

func (r *fakeResolver) LookupHost(_ context.Context, fqdn string) ([]string, error) {
	if err, ok := r.hostErr[fqdn]; ok {
		return nil, err
	}
	addrs, ok := r.hosts[fqdn]
	if !ok {
		return nil, errors.New("no such host: " + fqdn)
	}
	return addrs, nil
}

func (r *fakeResolver) LookupCNAME(_ context.Context, fqdn string) (string, error) {
	if err, ok := r.cnameErr[fqdn]; ok {
		return "", err
	}
	cname, ok := r.cnames[fqdn]
	if !ok {
		return "", errors.New("no such host: " + fqdn)
	}
	return cname, nil
}

var _ = Describe("ResolveDNSHandler", func() {
	var (
		ctx      context.Context
		resolver *fakeResolver
		handler  *dnspkg.ResolveDNSHandler
		rc       *reconciler.ReconcileContext[*sreportalv1alpha1.DNS]
	)

	BeforeEach(func() {
		ctx = context.Background()
		resolver = newFakeResolver()
		handler = dnspkg.NewResolveDNSHandler(resolver)
		rc = &reconciler.ReconcileContext[*sreportalv1alpha1.DNS]{
			Resource: &sreportalv1alpha1.DNS{
				ObjectMeta: metav1.ObjectMeta{Name: "test-dns", Namespace: "default"},
			},
			Data: make(map[string]any),
		}
	})

	Context("when all FQDNs are in sync", func() {
		BeforeEach(func() {
			resolver.hosts["app.example.com"] = []string{"10.0.0.1"}
			resolver.hosts["api.example.com"] = []string{"10.0.0.2"}
			rc.Data[dnspkg.DataKeyAggregatedGroups] = []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Services",
					Source: "external-dns",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.2"}},
						{FQDN: "app.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}},
					},
				},
			}
		})

		It("should set all statuses to sync", func() {
			err := handler.Handle(ctx, rc)
			Expect(err).NotTo(HaveOccurred())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(HaveLen(1))
			Expect(groups[0].FQDNs[0].SyncStatus).To(Equal("sync"))
			Expect(groups[0].FQDNs[1].SyncStatus).To(Equal("sync"))
		})
	})

	Context("when an FQDN is not available", func() {
		BeforeEach(func() {
			resolver.hosts["app.example.com"] = []string{"10.0.0.1"}
			// gone.example.com is not in resolver — will return error
			rc.Data[dnspkg.DataKeyAggregatedGroups] = []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Services",
					Source: "external-dns",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "app.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}},
						{FQDN: "gone.example.com", RecordType: "A", Targets: []string{"10.0.0.99"}},
					},
				},
			}
		})

		It("should set notavailable for missing FQDN and sync for existing", func() {
			err := handler.Handle(ctx, rc)
			Expect(err).NotTo(HaveOccurred())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups[0].FQDNs[0].SyncStatus).To(Equal("sync"))
			Expect(groups[0].FQDNs[1].SyncStatus).To(Equal("notavailable"))
		})
	})

	Context("when an FQDN has different targets", func() {
		BeforeEach(func() {
			resolver.hosts["drift.example.com"] = []string{"10.0.0.99"}
			rc.Data[dnspkg.DataKeyAggregatedGroups] = []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Services",
					Source: "external-dns",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "drift.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}},
					},
				},
			}
		})

		It("should set notsync", func() {
			err := handler.Handle(ctx, rc)
			Expect(err).NotTo(HaveOccurred())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups[0].FQDNs[0].SyncStatus).To(Equal("notsync"))
		})
	})

	Context("when CNAME records are checked", func() {
		BeforeEach(func() {
			resolver.cnames["alias.example.com"] = "real.example.com."
			rc.Data[dnspkg.DataKeyAggregatedGroups] = []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Aliases",
					Source: "external-dns",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "alias.example.com", RecordType: "CNAME", Targets: []string{"real.example.com."}},
					},
				},
			}
		})

		It("should set sync for matching CNAME", func() {
			err := handler.Handle(ctx, rc)
			Expect(err).NotTo(HaveOccurred())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups[0].FQDNs[0].SyncStatus).To(Equal("sync"))
		})
	})

	Context("when manual entries are checked", func() {
		BeforeEach(func() {
			resolver.hosts["manual-exists.example.com"] = []string{"10.0.0.1"}
			// manual-gone.example.com not in resolver
			rc.Data[dnspkg.DataKeyAggregatedGroups] = []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Manual",
					Source: "manual",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "manual-exists.example.com"},
						{FQDN: "manual-gone.example.com"},
					},
				},
			}
		})

		It("should check existence only for manual entries", func() {
			err := handler.Handle(ctx, rc)
			Expect(err).NotTo(HaveOccurred())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups[0].FQDNs[0].SyncStatus).To(Equal("sync"))
			Expect(groups[0].FQDNs[1].SyncStatus).To(Equal("notavailable"))
		})
	})

	Context("when groups have source remote", func() {
		BeforeEach(func() {
			// Do NOT register any hosts in the resolver — if resolution is attempted it will return "notavailable".
			rc.Data[dnspkg.DataKeyAggregatedGroups] = []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "RemoteServices",
					Source: "remote",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "remote.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, SyncStatus: "sync"},
						{FQDN: "remote-gone.example.com", RecordType: "A", Targets: []string{"10.0.0.2"}, SyncStatus: "notavailable"},
					},
				},
				{
					Name:   "LocalServices",
					Source: "external-dns",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "local.example.com", RecordType: "A", Targets: []string{"10.0.0.3"}},
					},
				},
			}
			resolver.hosts["local.example.com"] = []string{"10.0.0.3"}
		})

		It("should preserve remote SyncStatus and only resolve local FQDNs", func() {
			err := handler.Handle(ctx, rc)
			Expect(err).NotTo(HaveOccurred())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups).To(HaveLen(2))

			// Remote group: SyncStatus must remain untouched
			Expect(groups[0].FQDNs[0].SyncStatus).To(Equal("sync"))
			Expect(groups[0].FQDNs[1].SyncStatus).To(Equal("notavailable"))

			// Local group: should be resolved
			Expect(groups[1].FQDNs[0].SyncStatus).To(Equal("sync"))
		})
	})

	Context("when no aggregated groups exist", func() {
		It("should succeed without error", func() {
			err := handler.Handle(ctx, rc)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when groups have mixed record types", func() {
		BeforeEach(func() {
			resolver.hosts["a.example.com"] = []string{"10.0.0.1"}
			resolver.cnames["c.example.com"] = "target.example.com."
			resolver.hosts["manual.example.com"] = []string{"1.2.3.4"}
			rc.Data[dnspkg.DataKeyAggregatedGroups] = []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Mixed",
					Source: "external-dns",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "a.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}},
						{FQDN: "c.example.com", RecordType: "CNAME", Targets: []string{"target.example.com."}},
					},
				},
				{
					Name:   "Manual",
					Source: "manual",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "manual.example.com"},
					},
				},
			}
		})

		It("should resolve each FQDN according to its type", func() {
			err := handler.Handle(ctx, rc)
			Expect(err).NotTo(HaveOccurred())

			groups := rc.Data[dnspkg.DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
			Expect(groups[0].FQDNs[0].SyncStatus).To(Equal("sync"))
			Expect(groups[0].FQDNs[1].SyncStatus).To(Equal("sync"))
			Expect(groups[1].FQDNs[0].SyncStatus).To(Equal("sync"))
		})
	})
})
