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

package source_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/source"
	"github.com/golgoth31/sreportal/internal/source/service"
)

func TestSource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Source Suite")
}

var _ = Describe("ParseLabelSelector", func() {
	It("should return Everything for empty selector", func() {
		sel, err := source.ParseLabelSelector("")
		Expect(err).NotTo(HaveOccurred())
		Expect(sel.String()).To(Equal(""))
	})

	It("should parse valid selector", func() {
		sel, err := source.ParseLabelSelector("app=nginx")
		Expect(err).NotTo(HaveOccurred())
		Expect(sel.String()).To(Equal("app=nginx"))
	})

	It("should parse complex selector", func() {
		sel, err := source.ParseLabelSelector("app=nginx,env in (prod,staging)")
		Expect(err).NotTo(HaveOccurred())
		Expect(sel.Empty()).To(BeFalse())
	})

	It("should return error for invalid selector", func() {
		_, err := source.ParseLabelSelector("invalid===selector")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Factory.BuildTypedSources", func() {
	var (
		ctx     context.Context
		factory *source.Factory
	)

	Context("with no builders", func() {
		BeforeEach(func() {
			ctx = context.Background()
			factory = source.NewFactory(nil, nil, nil)
		})

		It("should return empty sources even with enabled config", func() {
			cfg := &config.OperatorConfig{
				Sources: config.SourcesConfig{
					Service: &config.ServiceConfig{Enabled: true},
				},
			}

			sources, err := factory.BuildTypedSources(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(sources).To(BeEmpty())
		})
	})

	Context("with service builder but disabled config", func() {
		BeforeEach(func() {
			ctx = context.Background()
			factory = source.NewFactory(nil, nil, []source.Builder{
				service.NewBuilder(),
			})
		})

		It("should return empty sources when all disabled", func() {
			cfg := &config.OperatorConfig{
				Sources: config.SourcesConfig{},
			}

			sources, err := factory.BuildTypedSources(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(sources).To(BeEmpty())
		})

		It("should return empty when explicitly disabled", func() {
			cfg := &config.OperatorConfig{
				Sources: config.SourcesConfig{
					Service: &config.ServiceConfig{Enabled: false},
				},
			}

			sources, err := factory.BuildTypedSources(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(sources).To(BeEmpty())
		})
	})
})

var _ = Describe("Factory.EnabledSourceTypes", func() {
	It("should return empty when all disabled", func() {
		factory := source.NewFactory(nil, nil, []source.Builder{service.NewBuilder()})
		cfg := &config.OperatorConfig{Sources: config.SourcesConfig{}}
		Expect(factory.EnabledSourceTypes(cfg)).To(BeEmpty())
	})

	It("should return enabled types", func() {
		factory := source.NewFactory(nil, nil, []source.Builder{service.NewBuilder()})
		cfg := &config.OperatorConfig{
			Sources: config.SourcesConfig{
				Service: &config.ServiceConfig{Enabled: true},
			},
		}
		types := factory.EnabledSourceTypes(cfg)
		Expect(types).To(Equal([]source.SourceType{service.SourceTypeService}))
	})
})
