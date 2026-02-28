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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golgoth31/sreportal/internal/config"
)

func TestSource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Source Suite")
}

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

var _ = Describe("Factory.BuildTypedSources", func() {
	var (
		ctx     context.Context
		factory *Factory
	)

	BeforeEach(func() {
		ctx = context.Background()
		// Factory without kubeClient - will fail for actual source creation
		// but we test the configuration logic (enabled/disabled checks).
		factory = NewFactory(nil, nil)
	})

	Context("with all sources disabled", func() {
		It("should return empty sources", func() {
			cfg := &config.OperatorConfig{
				Sources: config.SourcesConfig{
					// All nil/disabled
				},
			}

			sources, err := factory.BuildTypedSources(ctx, cfg)
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

			sources, err := factory.BuildTypedSources(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(sources).To(BeEmpty())
		})
	})
})
