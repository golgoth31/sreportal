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

package adapter

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
)

func TestAdapter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Adapter Suite")
}

var _ = Describe("EndpointsToGroups", func() {
	Context("with empty endpoints", func() {
		It("should return empty groups", func() {
			mapping := &config.GroupMappingConfig{DefaultGroup: "Default"}
			result := EndpointsToGroups(nil, mapping)
			Expect(result).To(BeEmpty())
		})

		It("should return empty groups for empty slice", func() {
			mapping := &config.GroupMappingConfig{DefaultGroup: "Default"}
			result := EndpointsToGroups([]*endpoint.Endpoint{}, mapping)
			Expect(result).To(BeEmpty())
		})
	})

	Context("with nil mapping", func() {
		It("should use default group name 'Services'", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpoint("test.example.com"),
			}
			result := EndpointsToGroups(eps, nil)
			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("Services"))
			Expect(result[0].Source).To(Equal(SourceExternalDNS))
		})
	})

	Context("with single endpoint", func() {
		It("should create group with default name", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpoint("api.example.com"),
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: "My Services"}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("My Services"))
			Expect(result[0].Source).To(Equal(SourceExternalDNS))
			Expect(result[0].FQDNs).To(HaveLen(1))
			Expect(result[0].FQDNs[0].FQDN).To(Equal("api.example.com"))
			Expect(result[0].FQDNs[0].RecordType).To(Equal("A"))
			Expect(result[0].FQDNs[0].Targets).To(Equal([]string{"10.0.0.1"}))
		})
	})

	Context("with multiple endpoints same group", func() {
		It("should group endpoints together", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpoint("api.example.com"),
				newTestEndpoint("web.example.com"),
				newTestEndpoint("admin.example.com"),
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: "Services"}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("Services"))
			Expect(result[0].FQDNs).To(HaveLen(3))
			// Should be sorted alphabetically
			Expect(result[0].FQDNs[0].FQDN).To(Equal("admin.example.com"))
			Expect(result[0].FQDNs[1].FQDN).To(Equal("api.example.com"))
			Expect(result[0].FQDNs[2].FQDN).To(Equal("web.example.com"))
		})
	})

	Context("with namespace mapping", func() {
		It("should map namespace to group name", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpointWithLabels("prod-api.example.com", map[string]string{
					endpoint.ResourceLabelKey: "service/production/api",
				}),
				newTestEndpointWithLabels("staging-api.example.com", map[string]string{
					endpoint.ResourceLabelKey: "service/staging/api",
				}),
				newTestEndpoint("other.example.com"), // No resource label
			}
			mapping := &config.GroupMappingConfig{
				DefaultGroup: "Other",
				ByNamespace: map[string]string{
					"production": "Production Services",
					"staging":    "Staging Services",
				},
			}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(3))
			// Should be sorted by group name
			Expect(result[0].Name).To(Equal("Other"))
			Expect(result[0].FQDNs).To(HaveLen(1))
			Expect(result[0].FQDNs[0].FQDN).To(Equal("other.example.com"))

			Expect(result[1].Name).To(Equal("Production Services"))
			Expect(result[1].FQDNs).To(HaveLen(1))
			Expect(result[1].FQDNs[0].FQDN).To(Equal("prod-api.example.com"))

			Expect(result[2].Name).To(Equal("Staging Services"))
			Expect(result[2].FQDNs).To(HaveLen(1))
			Expect(result[2].FQDNs[0].FQDN).To(Equal("staging-api.example.com"))
		})
	})

	Context("with label key mapping", func() {
		It("should use label value as group name", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpointWithLabels("api.example.com", map[string]string{
					"sreportal.my.domain/group": "API Services",
				}),
				newTestEndpointWithLabels("web.example.com", map[string]string{
					"sreportal.my.domain/group": "Web Services",
				}),
				newTestEndpoint("other.example.com"), // No group label
			}
			mapping := &config.GroupMappingConfig{
				DefaultGroup: "Default",
				LabelKey:     "sreportal.my.domain/group",
			}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(3))
			// Sorted alphabetically by group name
			Expect(result[0].Name).To(Equal("API Services"))
			Expect(result[0].FQDNs[0].FQDN).To(Equal("api.example.com"))

			Expect(result[1].Name).To(Equal("Default"))
			Expect(result[1].FQDNs[0].FQDN).To(Equal("other.example.com"))

			Expect(result[2].Name).To(Equal("Web Services"))
			Expect(result[2].FQDNs[0].FQDN).To(Equal("web.example.com"))
		})

		It("should prioritize label key over namespace mapping", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpointWithLabels("api.example.com", map[string]string{
					"sreportal.my.domain/group": "Custom Group",
					endpoint.ResourceLabelKey:   "service/production/api",
				}),
			}
			mapping := &config.GroupMappingConfig{
				DefaultGroup: "Default",
				LabelKey:     "sreportal.my.domain/group",
				ByNamespace: map[string]string{
					"production": "Production Services",
				},
			}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("Custom Group")) // Label takes priority
		})
	})

	Context("with different record types", func() {
		It("should preserve record type information", func() {
			eps := []*endpoint.Endpoint{
				{
					DNSName:    "api.example.com",
					RecordType: "CNAME",
					Targets:    []string{"lb.example.com"},
					Labels:     map[string]string{},
				},
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: "Services"}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].FQDNs[0].RecordType).To(Equal("CNAME"))
			Expect(result[0].FQDNs[0].Targets).To(Equal([]string{"lb.example.com"}))
		})
	})

	Context("with comma-separated groups annotation", func() {
		It("should place FQDN in multiple groups", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpointWithLabels("shared.example.com", map[string]string{
					GroupsAnnotationKey: "APIs, Applications",
				}),
				newTestEndpoint("single.example.com"),
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: "Default"}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(3))
			// Sorted: APIs, Applications, Default
			Expect(result[0].Name).To(Equal("APIs"))
			Expect(result[0].FQDNs).To(HaveLen(1))
			Expect(result[0].FQDNs[0].FQDN).To(Equal("shared.example.com"))

			Expect(result[1].Name).To(Equal("Applications"))
			Expect(result[1].FQDNs).To(HaveLen(1))
			Expect(result[1].FQDNs[0].FQDN).To(Equal("shared.example.com"))

			Expect(result[2].Name).To(Equal("Default"))
			Expect(result[2].FQDNs).To(HaveLen(1))
			Expect(result[2].FQDNs[0].FQDN).To(Equal("single.example.com"))
		})

		It("should trim whitespace around group names", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpointWithLabels("test.example.com", map[string]string{
					GroupsAnnotationKey: " Group A , Group B , Group C ",
				}),
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: "Default"}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(3))
			Expect(result[0].Name).To(Equal("Group A"))
			Expect(result[1].Name).To(Equal("Group B"))
			Expect(result[2].Name).To(Equal("Group C"))
		})

		It("should handle single group in annotation (no comma)", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpointWithLabels("test.example.com", map[string]string{
					GroupsAnnotationKey: "SingleGroup",
				}),
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: "Default"}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("SingleGroup"))
		})
	})
})

var _ = Describe("IsIgnored", func() {
	It("should return true when ignore label is 'true'", func() {
		ep := newTestEndpointWithLabels("test.example.com", map[string]string{
			IgnoreAnnotationKey: "true",
		})
		Expect(IsIgnored(ep)).To(BeTrue())
	})

	It("should return false when ignore label is absent", func() {
		ep := newTestEndpoint("test.example.com")
		Expect(IsIgnored(ep)).To(BeFalse())
	})

	It("should return false when ignore label is not 'true'", func() {
		ep := newTestEndpointWithLabels("test.example.com", map[string]string{
			IgnoreAnnotationKey: "false",
		})
		Expect(IsIgnored(ep)).To(BeFalse())
	})

	It("should return false for nil endpoint", func() {
		Expect(IsIgnored(nil)).To(BeFalse())
	})
})

var _ = Describe("IsEndpointStatusIgnored", func() {
	It("should return true when ignore label is 'true'", func() {
		ep := &sreportalv1alpha1.EndpointStatus{
			DNSName: "test.example.com",
			Labels:  map[string]string{IgnoreAnnotationKey: "true"},
		}
		Expect(IsEndpointStatusIgnored(ep)).To(BeTrue())
	})

	It("should return false when ignore label is absent", func() {
		ep := &sreportalv1alpha1.EndpointStatus{
			DNSName: "test.example.com",
			Labels:  map[string]string{},
		}
		Expect(IsEndpointStatusIgnored(ep)).To(BeFalse())
	})

	It("should return false for nil endpoint", func() {
		Expect(IsEndpointStatusIgnored(nil)).To(BeFalse())
	})
})

var _ = Describe("EndpointsToGroups with ignored endpoints", func() {
	It("should skip ignored endpoints", func() {
		eps := []*endpoint.Endpoint{
			newTestEndpointWithLabels("ignored.example.com", map[string]string{
				IgnoreAnnotationKey: "true",
			}),
		}
		mapping := &config.GroupMappingConfig{DefaultGroup: "Default"}

		result := EndpointsToGroups(eps, mapping)

		Expect(result).To(BeEmpty())
	})

	It("should preserve non-ignored endpoints alongside ignored ones", func() {
		eps := []*endpoint.Endpoint{
			newTestEndpoint("visible.example.com"),
			newTestEndpointWithLabels("ignored.example.com", map[string]string{
				IgnoreAnnotationKey: "true",
			}),
			newTestEndpoint("also-visible.example.com"),
		}
		mapping := &config.GroupMappingConfig{DefaultGroup: "Default"}

		result := EndpointsToGroups(eps, mapping)

		Expect(result).To(HaveLen(1))
		Expect(result[0].FQDNs).To(HaveLen(2))
		Expect(result[0].FQDNs[0].FQDN).To(Equal("also-visible.example.com"))
		Expect(result[0].FQDNs[1].FQDN).To(Equal("visible.example.com"))
	})
})

var _ = Describe("MergeGroups", func() {
	Context("with only manual groups", func() {
		It("should preserve all manual groups", func() {
			existing := []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Manual Group",
					Source: "manual",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "manual.example.com"},
					},
				},
			}
			external := []sreportalv1alpha1.FQDNGroupStatus{}

			result := MergeGroups(existing, external)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("Manual Group"))
			Expect(result[0].Source).To(Equal("manual"))
		})
	})

	Context("with only external groups", func() {
		It("should return external groups", func() {
			existing := []sreportalv1alpha1.FQDNGroupStatus{}
			external := []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "External Group",
					Source: SourceExternalDNS,
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "external.example.com"},
					},
				},
			}

			result := MergeGroups(existing, external)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("External Group"))
			Expect(result[0].Source).To(Equal(SourceExternalDNS))
		})
	})

	Context("with mixed groups", func() {
		It("should merge preserving manual and adding external", func() {
			existing := []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Manual Group",
					Source: "manual",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "manual.example.com"},
					},
				},
				{
					Name:   "Old External",
					Source: SourceExternalDNS, // This should be replaced
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "old.example.com"},
					},
				},
			}
			external := []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "New External",
					Source: SourceExternalDNS,
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "new.example.com"},
					},
				},
			}

			result := MergeGroups(existing, external)

			Expect(result).To(HaveLen(2))
			// Should be sorted alphabetically
			Expect(result[0].Name).To(Equal("Manual Group"))
			Expect(result[0].Source).To(Equal("manual"))
			Expect(result[1].Name).To(Equal("New External"))
			Expect(result[1].Source).To(Equal(SourceExternalDNS))
		})

		It("should preserve multiple manual groups", func() {
			existing := []sreportalv1alpha1.FQDNGroupStatus{
				{Name: "Manual A", Source: "manual"},
				{Name: "Manual B", Source: "manual"},
				{Name: "Old External", Source: SourceExternalDNS},
			}
			external := []sreportalv1alpha1.FQDNGroupStatus{
				{Name: "External A", Source: SourceExternalDNS},
				{Name: "External B", Source: SourceExternalDNS},
			}

			result := MergeGroups(existing, external)

			Expect(result).To(HaveLen(4))
			// Sorted: External A, External B, Manual A, Manual B
			Expect(result[0].Name).To(Equal("External A"))
			Expect(result[1].Name).To(Equal("External B"))
			Expect(result[2].Name).To(Equal("Manual A"))
			Expect(result[3].Name).To(Equal("Manual B"))
		})
	})

	Context("with empty inputs", func() {
		It("should handle both empty", func() {
			result := MergeGroups(nil, nil)
			Expect(result).To(BeEmpty())
		})

		It("should handle empty existing", func() {
			external := []sreportalv1alpha1.FQDNGroupStatus{
				{Name: "External", Source: SourceExternalDNS},
			}
			result := MergeGroups(nil, external)
			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("External"))
		})
	})
})

var _ = Describe("extractNamespace", func() {
	It("should extract namespace from resource label (kind/namespace/name)", func() {
		Expect(extractNamespace("service/production/api")).To(Equal("production"))
		Expect(extractNamespace("ingress/default/web")).To(Equal("default"))
		Expect(extractNamespace("gateway/kube-system/coredns")).To(Equal("kube-system"))
	})

	It("should return empty for non-namespaced resources", func() {
		Expect(extractNamespace("node/coredns")).To(Equal(""))
		Expect(extractNamespace("namespace")).To(Equal(""))
	})

	It("should handle empty resource", func() {
		Expect(extractNamespace("")).To(Equal(""))
	})
})

// Benchmarks — these are standard Go benchmarks (not Ginkgo), placed in the
// same package test file so they can reuse the helper constructors below.

func BenchmarkEndpointsToGroups_SmallFlat(b *testing.B) {
	mapping := &config.GroupMappingConfig{DefaultGroup: "Services"}
	eps := []*endpoint.Endpoint{
		newTestEndpoint("api.example.com"),
		newTestEndpoint("web.example.com"),
		newTestEndpoint("admin.example.com"),
		newTestEndpoint("db.example.com"),
		newTestEndpoint("cache.example.com"),
	}
	b.ResetTimer()
	for b.Loop() {
		EndpointsToGroups(eps, mapping)
	}
}

func BenchmarkEndpointsToGroups_MultiGroup(b *testing.B) {
	mapping := &config.GroupMappingConfig{DefaultGroup: "Default"}
	groups := []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"}
	eps := make([]*endpoint.Endpoint, 50)
	for i := range eps {
		eps[i] = newTestEndpointWithLabels(
			"svc.example.com",
			map[string]string{GroupsAnnotationKey: groups[i%len(groups)]},
		)
	}
	b.ResetTimer()
	for b.Loop() {
		EndpointsToGroups(eps, mapping)
	}
}

func BenchmarkEndpointsToGroups_NamespaceMapping(b *testing.B) {
	mapping := &config.GroupMappingConfig{
		DefaultGroup: "Default",
		ByNamespace:  map[string]string{"production": "Prod", "staging": "Stage", "dev": "Dev"},
	}
	namespaces := []string{"production", "staging", "dev", "other"}
	eps := make([]*endpoint.Endpoint, 50)
	for i := range eps {
		eps[i] = newTestEndpointWithLabels("svc.example.com", map[string]string{
			endpoint.ResourceLabelKey: "service/" + namespaces[i%len(namespaces)] + "/svc",
		})
	}
	b.ResetTimer()
	for b.Loop() {
		EndpointsToGroups(eps, mapping)
	}
}

// Test helper functions

func newTestEndpoint(dnsName string) *endpoint.Endpoint {
	return &endpoint.Endpoint{
		DNSName:    dnsName,
		RecordType: "A",
		Targets:    []string{"10.0.0.1"},
		Labels:     map[string]string{},
	}
}

func newTestEndpointWithLabels(dnsName string, labels map[string]string) *endpoint.Endpoint {
	return &endpoint.Endpoint{
		DNSName:    dnsName,
		RecordType: "A",
		Targets:    []string{"10.0.0.1"},
		Labels:     labels,
	}
}
