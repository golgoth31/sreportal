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

package source

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/config"
)

func TestSource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Source Suite")
}

// mockSource implements source.Source for testing.
type mockSource struct {
	endpoints []*endpoint.Endpoint
	err       error
	handlers  []func()
}

func (m *mockSource) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error) {
	return m.endpoints, m.err
}

func (m *mockSource) AddEventHandler(ctx context.Context, handler func()) {
	m.handlers = append(m.handlers, handler)
}

func newMockSource(endpoints ...*endpoint.Endpoint) *mockSource {
	return &mockSource{endpoints: endpoints}
}

func newMockSourceWithError(err error) *mockSource {
	return &mockSource{err: err}
}

var _ = Describe("multiSource", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("with multiple sources", func() {
		It("should collect endpoints from all sources", func() {
			src1 := newMockSource(
				newTestEndpoint("api.example.com"),
				newTestEndpoint("web.example.com"),
			)
			src2 := newMockSource(
				newTestEndpoint("admin.example.com"),
			)

			multi := &multiSource{sources: []Source{src1, src2}}
			eps, err := multi.Endpoints(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(eps).To(HaveLen(3))
		})

		It("should deduplicate by DNSName+RecordType", func() {
			src1 := newMockSource(
				newTestEndpoint("api.example.com"),
				newTestEndpoint("web.example.com"),
			)
			src2 := newMockSource(
				newTestEndpoint("api.example.com"), // Duplicate
				newTestEndpoint("admin.example.com"),
			)

			multi := &multiSource{sources: []Source{src1, src2}}
			eps, err := multi.Endpoints(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(eps).To(HaveLen(3))

			// Check that we have the expected unique entries
			dnsNames := make([]string, 0, len(eps))
			for _, ep := range eps {
				dnsNames = append(dnsNames, ep.DNSName)
			}
			Expect(dnsNames).To(ContainElements("api.example.com", "web.example.com", "admin.example.com"))
		})

		It("should keep first endpoint when duplicate found", func() {
			src1 := newMockSource(
				&endpoint.Endpoint{
					DNSName:    "api.example.com",
					RecordType: "A",
					Targets:    endpoint.Targets{"10.0.0.1"}, // First target
				},
			)
			src2 := newMockSource(
				&endpoint.Endpoint{
					DNSName:    "api.example.com",
					RecordType: "A",
					Targets:    endpoint.Targets{"10.0.0.2"}, // Different target
				},
			)

			multi := &multiSource{sources: []Source{src1, src2}}
			eps, err := multi.Endpoints(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(eps).To(HaveLen(1))
			Expect(eps[0].Targets).To(Equal(endpoint.Targets{"10.0.0.1"})) // First wins
		})

		It("should not deduplicate different record types", func() {
			src1 := newMockSource(
				&endpoint.Endpoint{
					DNSName:    "api.example.com",
					RecordType: "A",
					Targets:    endpoint.Targets{"10.0.0.1"},
				},
			)
			src2 := newMockSource(
				&endpoint.Endpoint{
					DNSName:    "api.example.com",
					RecordType: "AAAA", // Different record type
					Targets:    endpoint.Targets{"::1"},
				},
			)

			multi := &multiSource{sources: []Source{src1, src2}}
			eps, err := multi.Endpoints(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(eps).To(HaveLen(2)) // Both should be present
		})

		It("should continue on source error", func() {
			src1 := newMockSource(
				newTestEndpoint("api.example.com"),
			)
			src2 := newMockSourceWithError(errors.New("source error"))
			src3 := newMockSource(
				newTestEndpoint("web.example.com"),
			)

			multi := &multiSource{sources: []Source{src1, src2, src3}}
			eps, err := multi.Endpoints(ctx)

			Expect(err).NotTo(HaveOccurred()) // Should not fail
			Expect(eps).To(HaveLen(2))        // Both working sources' endpoints
		})

		It("should return empty when all sources fail", func() {
			src1 := newMockSourceWithError(errors.New("error 1"))
			src2 := newMockSourceWithError(errors.New("error 2"))

			multi := &multiSource{sources: []Source{src1, src2}}
			eps, err := multi.Endpoints(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(eps).To(BeEmpty())
		})
	})

	Context("with empty sources", func() {
		It("should return empty endpoints", func() {
			multi := &multiSource{sources: []Source{}}
			eps, err := multi.Endpoints(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(eps).To(BeEmpty())
		})
	})

	Context("AddEventHandler", func() {
		It("should propagate handler to all sources", func() {
			src1 := newMockSource()
			src2 := newMockSource()

			multi := &multiSource{sources: []Source{src1, src2}}

			handler := func() {}
			multi.AddEventHandler(ctx, handler)

			Expect(src1.handlers).To(HaveLen(1))
			Expect(src2.handlers).To(HaveLen(1))
		})
	})
})

var _ = Describe("parseLabelSelector", func() {
	It("should return Everything for empty selector", func() {
		sel, err := parseLabelSelector("")
		Expect(err).NotTo(HaveOccurred())
		Expect(sel.String()).To(Equal(""))
	})

	It("should parse valid selector", func() {
		sel, err := parseLabelSelector("app=nginx")
		Expect(err).NotTo(HaveOccurred())
		Expect(sel.String()).To(Equal("app=nginx"))
	})

	It("should parse complex selector", func() {
		sel, err := parseLabelSelector("app=nginx,env in (prod,staging)")
		Expect(err).NotTo(HaveOccurred())
		Expect(sel.Empty()).To(BeFalse())
	})

	It("should return error for invalid selector", func() {
		_, err := parseLabelSelector("invalid===selector")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Factory.BuildSources", func() {
	var (
		ctx     context.Context
		factory *Factory
	)

	BeforeEach(func() {
		ctx = context.Background()
		// Factory without kubeClient - will fail for actual source creation
		// but we test the configuration logic
		factory = NewFactory(nil, nil)
	})

	Context("with all sources disabled", func() {
		It("should return empty sources", func() {
			cfg := &config.OperatorConfig{
				Sources: config.SourcesConfig{
					// All nil/disabled
				},
			}

			sources, err := factory.BuildSources(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(sources).To(BeEmpty())
		})

		It("should return empty when explicitly disabled", func() {
			cfg := &config.OperatorConfig{
				Sources: config.SourcesConfig{
					Service: &config.ServiceConfig{Enabled: false},
					Ingress: &config.IngressConfig{Enabled: false},
				},
			}

			sources, err := factory.BuildSources(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(sources).To(BeEmpty())
		})
	})
})

// Test helper for creating test endpoints
func newTestEndpoint(dnsName string) *endpoint.Endpoint {
	return &endpoint.Endpoint{
		DNSName:    dnsName,
		RecordType: "A",
		Targets:    []string{"10.0.0.1"},
		Labels:     map[string]string{},
	}
}
