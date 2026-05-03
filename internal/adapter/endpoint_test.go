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
			mapping := &config.GroupMappingConfig{DefaultGroup: tValDefault}
			result := EndpointsToGroups(nil, mapping)
			Expect(result).To(BeEmpty())
		})

		It("should return empty groups for empty slice", func() {
			mapping := &config.GroupMappingConfig{DefaultGroup: tValDefault}
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
			Expect(result[0].Name).To(Equal(defaultGroupServices))
			Expect(result[0].Source).To(Equal(SourceExternalDNS))
		})
	})

	Context("with single endpoint", func() {
		It("should create group with default name", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpoint(tFQDNAPI),
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: "My Services"}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("My Services"))
			Expect(result[0].Source).To(Equal(SourceExternalDNS))
			Expect(result[0].FQDNs).To(HaveLen(1))
			Expect(result[0].FQDNs[0].FQDN).To(Equal(tFQDNAPI))
			Expect(result[0].FQDNs[0].RecordType).To(Equal("A"))
			Expect(result[0].FQDNs[0].Targets).To(Equal([]string{tIP10001}))
		})
	})

	Context("with multiple endpoints same group", func() {
		It("should group endpoints together", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpoint(tFQDNAPI),
				newTestEndpoint("web.example.com"),
				newTestEndpoint("admin.example.com"),
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal(defaultGroupServices))
			Expect(result[0].FQDNs).To(HaveLen(3))
			// Should be sorted alphabetically
			Expect(result[0].FQDNs[0].FQDN).To(Equal("admin.example.com"))
			Expect(result[0].FQDNs[1].FQDN).To(Equal(tFQDNAPI))
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
					tEnvProd:    "Production Services",
					tEnvStaging: "Staging Services",
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
				newTestEndpointWithLabels(tFQDNAPI, map[string]string{
					tLabelGroup: "API Services",
				}),
				newTestEndpointWithLabels("web.example.com", map[string]string{
					tLabelGroup: "Web Services",
				}),
				newTestEndpoint("other.example.com"), // No group label
			}
			mapping := &config.GroupMappingConfig{
				DefaultGroup: tValDefault,
				LabelKey:     tLabelGroup,
			}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(3))
			// Sorted alphabetically by group name
			Expect(result[0].Name).To(Equal("API Services"))
			Expect(result[0].FQDNs[0].FQDN).To(Equal(tFQDNAPI))

			Expect(result[1].Name).To(Equal(tValDefault))
			Expect(result[1].FQDNs[0].FQDN).To(Equal("other.example.com"))

			Expect(result[2].Name).To(Equal("Web Services"))
			Expect(result[2].FQDNs[0].FQDN).To(Equal("web.example.com"))
		})

		It("should prioritize label key over namespace mapping", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpointWithLabels(tFQDNAPI, map[string]string{
					tLabelGroup:               "Custom Group",
					endpoint.ResourceLabelKey: "service/production/api",
				}),
			}
			mapping := &config.GroupMappingConfig{
				DefaultGroup: tValDefault,
				LabelKey:     tLabelGroup,
				ByNamespace: map[string]string{
					tEnvProd: "Production Services",
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
					DNSName:    tFQDNAPI,
					RecordType: tRecordCNAME,
					Targets:    []string{tFQDNLB},
					Labels:     map[string]string{},
				},
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].FQDNs[0].RecordType).To(Equal(tRecordCNAME))
			Expect(result[0].FQDNs[0].Targets).To(Equal([]string{tFQDNLB}))
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
			mapping := &config.GroupMappingConfig{DefaultGroup: tValDefault}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(3))
			// Sorted: APIs, Applications, Default
			Expect(result[0].Name).To(Equal("APIs"))
			Expect(result[0].FQDNs).To(HaveLen(1))
			Expect(result[0].FQDNs[0].FQDN).To(Equal("shared.example.com"))

			Expect(result[1].Name).To(Equal("Applications"))
			Expect(result[1].FQDNs).To(HaveLen(1))
			Expect(result[1].FQDNs[0].FQDN).To(Equal("shared.example.com"))

			Expect(result[2].Name).To(Equal(tValDefault))
			Expect(result[2].FQDNs).To(HaveLen(1))
			Expect(result[2].FQDNs[0].FQDN).To(Equal("single.example.com"))
		})

		It("should trim whitespace around group names", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpointWithLabels("test.example.com", map[string]string{
					GroupsAnnotationKey: " Group A , Group B , Group C ",
				}),
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: tValDefault}

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
			mapping := &config.GroupMappingConfig{DefaultGroup: tValDefault}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("SingleGroup"))
		})
	})
})

var _ = Describe("IsIgnored", func() {
	It("should return true when ignore label is 'true'", func() {
		ep := newTestEndpointWithLabels("test.example.com", map[string]string{
			IgnoreAnnotationKey: annotationValueTrue,
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
			Labels:  map[string]string{IgnoreAnnotationKey: annotationValueTrue},
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
				IgnoreAnnotationKey: annotationValueTrue,
			}),
		}
		mapping := &config.GroupMappingConfig{DefaultGroup: tValDefault}

		result := EndpointsToGroups(eps, mapping)

		Expect(result).To(BeEmpty())
	})

	It("should preserve non-ignored endpoints alongside ignored ones", func() {
		eps := []*endpoint.Endpoint{
			newTestEndpoint("visible.example.com"),
			newTestEndpointWithLabels("ignored.example.com", map[string]string{
				IgnoreAnnotationKey: annotationValueTrue,
			}),
			newTestEndpoint("also-visible.example.com"),
		}
		mapping := &config.GroupMappingConfig{DefaultGroup: tValDefault}

		result := EndpointsToGroups(eps, mapping)

		Expect(result).To(HaveLen(1))
		Expect(result[0].FQDNs).To(HaveLen(2))
		Expect(result[0].FQDNs[0].FQDN).To(Equal("also-visible.example.com"))
		Expect(result[0].FQDNs[1].FQDN).To(Equal("visible.example.com"))
	})
})

var _ = Describe("EndpointStatusToGroups", func() {
	Context("with duplicate FQDNs from multiple DNSRecords", func() {
		It("should deduplicate FQDNs within the same group", func() {
			endpoints := []sreportalv1alpha1.EndpointStatus{
				{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
				{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10002}},
				{DNSName: "web.example.com", RecordType: "A", Targets: []string{tIP10003}},
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}

			result := EndpointStatusToGroups(endpoints, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal(defaultGroupServices))
			// api.example.com should appear only once, with merged targets
			Expect(result[0].FQDNs).To(HaveLen(2))
			for _, fqdn := range result[0].FQDNs {
				if fqdn.FQDN == tFQDNAPI {
					Expect(fqdn.Targets).To(ConsistOf(tIP10001, tIP10002))
				}
			}
		})

		It("should deduplicate FQDNs with same targets", func() {
			endpoints := []sreportalv1alpha1.EndpointStatus{
				{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
				{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}

			result := EndpointStatusToGroups(endpoints, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].FQDNs).To(HaveLen(1))
			Expect(result[0].FQDNs[0].FQDN).To(Equal(tFQDNAPI))
			Expect(result[0].FQDNs[0].Targets).To(Equal([]string{tIP10001}))
		})

		It("should keep different record types as separate entries", func() {
			endpoints := []sreportalv1alpha1.EndpointStatus{
				{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
				{DNSName: tFQDNAPI, RecordType: tRecordCNAME, Targets: []string{tFQDNLB}},
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}

			result := EndpointStatusToGroups(endpoints, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].FQDNs).To(HaveLen(2))
		})
	})
})

var _ = Describe("MergeGroups", func() {
	Context("with only manual groups", func() {
		It("should preserve all manual groups", func() {
			existing := []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Manual Group",
					Source: tValManual,
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "manual.example.com"},
					},
				},
			}
			external := []sreportalv1alpha1.FQDNGroupStatus{}

			result := MergeGroups(existing, external)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("Manual Group"))
			Expect(result[0].Source).To(Equal(tValManual))
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
					Source: tValManual,
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
			Expect(result[0].Source).To(Equal(tValManual))
			Expect(result[1].Name).To(Equal("New External"))
			Expect(result[1].Source).To(Equal(SourceExternalDNS))
		})

		It("should preserve multiple manual groups", func() {
			existing := []sreportalv1alpha1.FQDNGroupStatus{
				{Name: "Manual A", Source: tValManual},
				{Name: "Manual B", Source: tValManual},
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
		Expect(extractNamespace("service/production/api")).To(Equal(tEnvProd))
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

var _ = Describe("EndpointsToGroups OriginRef", func() {
	Context("when endpoint has a valid resource label", func() {
		It("should populate OriginRef with kind, namespace and name", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpointWithLabels(tFQDNAPI, map[string]string{
					endpoint.ResourceLabelKey: tEndpointSvcProdAPI,
				}),
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}

			result := EndpointsToGroups(eps, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].FQDNs).To(HaveLen(1))
			fqdn := result[0].FQDNs[0]
			Expect(fqdn.OriginRef).NotTo(BeNil())
			Expect(fqdn.OriginRef.Kind).To(Equal(tSrcService))
			Expect(fqdn.OriginRef.Namespace).To(Equal(tEnvProd))
			Expect(fqdn.OriginRef.Name).To(Equal("api-svc"))
		})

		It("should populate OriginRef for ingress resources", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpointWithLabels("web.example.com", map[string]string{
					endpoint.ResourceLabelKey: "ingress/default/web-ingress",
				}),
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}

			result := EndpointsToGroups(eps, mapping)

			fqdn := result[0].FQDNs[0]
			Expect(fqdn.OriginRef).NotTo(BeNil())
			Expect(fqdn.OriginRef.Kind).To(Equal(tSrcIngress))
			Expect(fqdn.OriginRef.Namespace).To(Equal("default"))
			Expect(fqdn.OriginRef.Name).To(Equal("web-ingress"))
		})
	})

	Context("when endpoint has no resource label", func() {
		It("should leave OriginRef nil", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpoint(tFQDNAPI),
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}

			result := EndpointsToGroups(eps, mapping)

			Expect(result[0].FQDNs[0].OriginRef).To(BeNil())
		})
	})

	Context("when endpoint has a malformed resource label", func() {
		It("should leave OriginRef nil", func() {
			eps := []*endpoint.Endpoint{
				newTestEndpointWithLabels(tFQDNAPI, map[string]string{
					endpoint.ResourceLabelKey: "not-a-valid-label",
				}),
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}

			result := EndpointsToGroups(eps, mapping)

			Expect(result[0].FQDNs[0].OriginRef).To(BeNil())
		})
	})
})

var _ = Describe("EndpointStatusToGroups OriginRef", func() {
	Context("when EndpointStatus has a valid resource label", func() {
		It("should propagate OriginRef to the resulting FQDNStatus", func() {
			endpoints := []sreportalv1alpha1.EndpointStatus{
				{
					DNSName:    tFQDNAPI,
					RecordType: "A",
					Targets:    []string{tIP10001},
					Labels: map[string]string{
						endpoint.ResourceLabelKey: tEndpointSvcProdAPI,
					},
				},
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}

			result := EndpointStatusToGroups(endpoints, mapping)

			Expect(result).To(HaveLen(1))
			fqdn := result[0].FQDNs[0]
			Expect(fqdn.OriginRef).NotTo(BeNil())
			Expect(fqdn.OriginRef.Kind).To(Equal(tSrcService))
			Expect(fqdn.OriginRef.Namespace).To(Equal(tEnvProd))
			Expect(fqdn.OriginRef.Name).To(Equal("api-svc"))
		})
	})

	Context("when duplicate FQDNs are merged", func() {
		It("should keep OriginRef from the first occurrence", func() {
			endpoints := []sreportalv1alpha1.EndpointStatus{
				{
					DNSName:    tFQDNAPI,
					RecordType: "A",
					Targets:    []string{tIP10001},
					Labels: map[string]string{
						endpoint.ResourceLabelKey: tEndpointSvcProdAPI,
					},
				},
				{
					DNSName:    tFQDNAPI,
					RecordType: "A",
					Targets:    []string{tIP10002},
					Labels: map[string]string{
						endpoint.ResourceLabelKey: tEndpointSvcProdAPI,
					},
				},
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}

			result := EndpointStatusToGroups(endpoints, mapping)

			Expect(result).To(HaveLen(1))
			Expect(result[0].FQDNs).To(HaveLen(1))
			fqdn := result[0].FQDNs[0]
			Expect(fqdn.OriginRef).NotTo(BeNil())
			Expect(fqdn.OriginRef.Name).To(Equal("api-svc"))
			// Both targets must be merged
			Expect(fqdn.Targets).To(ConsistOf(tIP10001, tIP10002))
		})
	})

	Context("when EndpointStatus has no resource label", func() {
		It("should leave OriginRef nil", func() {
			endpoints := []sreportalv1alpha1.EndpointStatus{
				{
					DNSName:    tFQDNAPI,
					RecordType: "A",
					Targets:    []string{tIP10001},
					Labels:     map[string]string{},
				},
			}
			mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}

			result := EndpointStatusToGroups(endpoints, mapping)

			Expect(result[0].FQDNs[0].OriginRef).To(BeNil())
		})
	})
})

var _ = Describe("ApplySourcePriority", func() {
	Context("with empty priority", func() {
		It("should return all endpoints flattened from all sources", func() {
			endpointsBySource := map[string][]sreportalv1alpha1.EndpointStatus{
				tSrcService: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
				},
				tSrcIngress: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10002}},
				},
			}

			result := ApplySourcePriority(endpointsBySource, nil)
			// Without priority, both endpoints are returned (existing merge behaviour)
			Expect(result).To(HaveLen(2))
		})
	})

	Context("when same FQDN+RecordType appears in two sources", func() {
		It("should keep the endpoint from the highest-priority source", func() {
			endpointsBySource := map[string][]sreportalv1alpha1.EndpointStatus{
				tSrcService: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
				},
				tSrcIngress: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10002}},
				},
			}

			result := ApplySourcePriority(endpointsBySource, []string{tSrcService, tSrcIngress})

			Expect(result).To(HaveLen(1))
			Expect(result[0].DNSName).To(Equal(tFQDNAPI))
			Expect(result[0].Targets).To(Equal([]string{tIP10001})) // service wins
		})

		It("should use ingress target when ingress has higher priority", func() {
			endpointsBySource := map[string][]sreportalv1alpha1.EndpointStatus{
				tSrcService: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
				},
				tSrcIngress: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10002}},
				},
			}

			result := ApplySourcePriority(endpointsBySource, []string{tSrcIngress, tSrcService})

			Expect(result).To(HaveLen(1))
			Expect(result[0].DNSName).To(Equal(tFQDNAPI))
			Expect(result[0].Targets).To(Equal([]string{tIP10002})) // ingress wins
		})
	})

	Context("when FQDN only exists in one source", func() {
		It("should always be included regardless of priority", func() {
			endpointsBySource := map[string][]sreportalv1alpha1.EndpointStatus{
				tSrcService: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
				},
			}

			result := ApplySourcePriority(endpointsBySource, []string{tSrcIngress, tSrcService})

			Expect(result).To(HaveLen(1))
			Expect(result[0].DNSName).To(Equal(tFQDNAPI))
		})
	})

	Context("when source is not in the priority list", func() {
		It("should lose to any listed source on conflict", func() {
			endpointsBySource := map[string][]sreportalv1alpha1.EndpointStatus{
				tSrcDNSEndpoint: { // not in priority list
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10003}},
				},
				tSrcService: { // in priority list
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
				},
			}

			result := ApplySourcePriority(endpointsBySource, []string{tSrcService, tSrcIngress})

			Expect(result).To(HaveLen(1))
			Expect(result[0].Targets).To(Equal([]string{tIP10001})) // service wins over unlisted dnsendpoint
		})

		It("should still contribute FQDNs it uniquely owns", func() {
			endpointsBySource := map[string][]sreportalv1alpha1.EndpointStatus{
				tSrcDNSEndpoint: {
					{DNSName: tFQDNDNS, RecordType: "A", Targets: []string{tIP10003}},
				},
				tSrcService: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
				},
			}

			result := ApplySourcePriority(endpointsBySource, []string{tSrcService, tSrcIngress})

			Expect(result).To(HaveLen(2)) // both included, no conflict
			names := make([]string, len(result))
			for i, ep := range result {
				names[i] = ep.DNSName
			}
			Expect(names).To(ConsistOf(tFQDNAPI, tFQDNDNS))
		})
	})

	Context("with multiple FQDNs in different sources", func() {
		It("should resolve each FQDN independently", func() {
			endpointsBySource := map[string][]sreportalv1alpha1.EndpointStatus{
				tSrcService: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
					{DNSName: "web.example.com", RecordType: "A", Targets: []string{tIP10002}},
				},
				tSrcIngress: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{"10.0.0.99"}},       // conflicts
					{DNSName: "app.example.com", RecordType: "A", Targets: []string{tIP10003}}, // unique
				},
			}

			result := ApplySourcePriority(endpointsBySource, []string{tSrcService, tSrcIngress})

			Expect(result).To(HaveLen(3)) // api (service wins), web (service only), app (ingress only)
			names := make([]string, len(result))
			for i, ep := range result {
				names[i] = ep.DNSName
			}
			Expect(names).To(ConsistOf(tFQDNAPI, "web.example.com", "app.example.com"))
			for _, ep := range result {
				if ep.DNSName == tFQDNAPI {
					Expect(ep.Targets).To(Equal([]string{tIP10001})) // service target wins
				}
			}
		})
	})

	Context("with empty input", func() {
		It("should return nil for nil input", func() {
			result := ApplySourcePriority(nil, []string{tSrcService})
			Expect(result).To(BeNil())
		})

		It("should return nil for empty map", func() {
			result := ApplySourcePriority(map[string][]sreportalv1alpha1.EndpointStatus{}, []string{tSrcService})
			Expect(result).To(BeNil())
		})
	})

	Context("when same FQDN has different RecordType in different sources", func() {
		It("should suppress the lower-priority source even when RecordTypes differ", func() {
			// Real-world case: Service publishes an A record (ClusterIP via publishInternal)
			// while Istio Gateway publishes a CNAME (cloud LB hostname). With service having
			// higher priority, the CNAME from istio-gateway must be dropped entirely.
			endpointsBySource := map[string][]sreportalv1alpha1.EndpointStatus{
				tSrcService: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
				},
				tSrcIstioGateway: {
					{DNSName: tFQDNAPI, RecordType: tRecordCNAME, Targets: []string{tFQDNLB}},
				},
			}

			result := ApplySourcePriority(endpointsBySource, []string{tSrcService, tSrcIstioGateway})

			Expect(result).To(HaveLen(1))
			Expect(result[0].DNSName).To(Equal(tFQDNAPI))
			Expect(result[0].RecordType).To(Equal("A"))
			Expect(result[0].Targets).To(Equal([]string{tIP10001}))
		})

		It("should keep all record types from the winning source", func() {
			// Service publishes both A and AAAA for the same hostname; istio-gateway
			// publishes only a CNAME. Service wins → both A and AAAA are kept.
			endpointsBySource := map[string][]sreportalv1alpha1.EndpointStatus{
				tSrcService: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
					{DNSName: tFQDNAPI, RecordType: "AAAA", Targets: []string{"::1"}},
				},
				tSrcIstioGateway: {
					{DNSName: tFQDNAPI, RecordType: tRecordCNAME, Targets: []string{tFQDNLB}},
				},
			}

			result := ApplySourcePriority(endpointsBySource, []string{tSrcService, tSrcIstioGateway})

			Expect(result).To(HaveLen(2))
			recordTypes := make([]string, len(result))
			for i, ep := range result {
				recordTypes[i] = ep.RecordType
			}
			Expect(recordTypes).To(ConsistOf("A", "AAAA"))
		})
	})

	Context("when the same source has intra-source duplicate keys", func() {
		It("should merge targets from intra-source duplicates when priority is configured", func() {
			endpointsBySource := map[string][]sreportalv1alpha1.EndpointStatus{
				tSrcService: {
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10001}},
					{DNSName: tFQDNAPI, RecordType: "A", Targets: []string{tIP10002}},
				},
			}

			result := ApplySourcePriority(endpointsBySource, []string{tSrcService, tSrcIngress})

			Expect(result).To(HaveLen(1))
			Expect(result[0].DNSName).To(Equal(tFQDNAPI))
			Expect(result[0].Targets).To(ConsistOf(tIP10001, tIP10002))
		})

		It("should merge targets from intra-source duplicates even when the source is not in the priority list", func() {
			endpointsBySource := map[string][]sreportalv1alpha1.EndpointStatus{
				tSrcDNSEndpoint: { // unlisted
					{DNSName: tFQDNDNS, RecordType: "A", Targets: []string{tIP10001}},
					{DNSName: tFQDNDNS, RecordType: "A", Targets: []string{tIP10002}},
				},
			}

			result := ApplySourcePriority(endpointsBySource, []string{tSrcService, tSrcIngress})

			Expect(result).To(HaveLen(1))
			Expect(result[0].Targets).To(ConsistOf(tIP10001, tIP10002))
		})
	})
})

var _ = Describe("ResolveComponentSpec", func() {
	Context("when endpoint has no component annotation", func() {
		It("should return nil", func() {
			ep := newTestEndpoint(tFQDNAPI)
			Expect(ResolveComponentSpec(ep)).To(BeNil())
		})
	})

	Context("when endpoint has empty component annotation", func() {
		It("should return nil", func() {
			ep := newTestEndpointWithLabels(tFQDNAPI, map[string]string{
				ComponentAnnotationKey: "",
			})
			Expect(ResolveComponentSpec(ep)).To(BeNil())
		})
	})

	Context("when endpoint is nil", func() {
		It("should return nil", func() {
			Expect(ResolveComponentSpec(nil)).To(BeNil())
		})
	})

	Context("when endpoint has component annotation only", func() {
		It("should return spec with display name and empty optional fields", func() {
			ep := newTestEndpointWithLabels(tFQDNAPI, map[string]string{
				ComponentAnnotationKey: tCompAPIGateway,
			})
			spec := ResolveComponentSpec(ep)
			Expect(spec).NotTo(BeNil())
			Expect(spec.DisplayName).To(Equal(tCompAPIGateway))
			Expect(spec.Group).To(BeEmpty())
			Expect(spec.Description).To(BeEmpty())
			Expect(spec.Link).To(BeEmpty())
			Expect(spec.Status).To(BeEmpty())
		})
	})

	Context("when endpoint has all component annotations", func() {
		It("should extract all fields", func() {
			ep := newTestEndpointWithLabels(tFQDNAPI, map[string]string{
				ComponentAnnotationKey:            tCompAPIGateway,
				ComponentGroupAnnotationKey:       "Infrastructure",
				ComponentDescriptionAnnotationKey: "Main API ingress",
				ComponentLinkAnnotationKey:        "https://grafana.internal/d/api",
				ComponentStatusAnnotationKey:      "operational",
			})
			spec := ResolveComponentSpec(ep)
			Expect(spec).NotTo(BeNil())
			Expect(spec.DisplayName).To(Equal(tCompAPIGateway))
			Expect(spec.Group).To(Equal("Infrastructure"))
			Expect(spec.Description).To(Equal("Main API ingress"))
			Expect(spec.Link).To(Equal("https://grafana.internal/d/api"))
			Expect(spec.Status).To(Equal("operational"))
		})
	})
})

var _ = Describe("ComponentAnnotationsFromMap", func() {
	Context("when annotations are empty", func() {
		It("should return nil", func() {
			Expect(ComponentAnnotationsFromMap(nil)).To(BeNil())
			Expect(ComponentAnnotationsFromMap(map[string]string{})).To(BeNil())
		})
	})

	Context("when component annotation is absent", func() {
		It("should return nil", func() {
			ann := map[string]string{"some/other": "value"}
			Expect(ComponentAnnotationsFromMap(ann)).To(BeNil())
		})
	})

	Context("when component annotation is present", func() {
		It("should extract all fields", func() {
			ann := map[string]string{
				ComponentAnnotationKey:            "DNS Service",
				ComponentGroupAnnotationKey:       "Core",
				ComponentDescriptionAnnotationKey: "Internal DNS",
				ComponentLinkAnnotationKey:        "https://console.cloud.google.com/dns",
				ComponentStatusAnnotationKey:      "degraded",
			}
			spec := ComponentAnnotationsFromMap(ann)
			Expect(spec).NotTo(BeNil())
			Expect(spec.DisplayName).To(Equal("DNS Service"))
			Expect(spec.Group).To(Equal("Core"))
			Expect(spec.Description).To(Equal("Internal DNS"))
			Expect(spec.Link).To(Equal("https://console.cloud.google.com/dns"))
			Expect(spec.Status).To(Equal("degraded"))
		})
	})
})

var _ = Describe("EnrichEndpointLabels with component annotations", func() {
	It("should propagate component annotations from K8s resource to endpoint", func() {
		ep := newTestEndpoint(tFQDNAPI)
		annotations := map[string]string{
			ComponentAnnotationKey:       tCompAPIGateway,
			ComponentGroupAnnotationKey:  "Infrastructure",
			ComponentStatusAnnotationKey: "operational",
		}
		EnrichEndpointLabels(ep, annotations)

		Expect(ep.Labels[ComponentAnnotationKey]).To(Equal(tCompAPIGateway))
		Expect(ep.Labels[ComponentGroupAnnotationKey]).To(Equal("Infrastructure"))
		Expect(ep.Labels[ComponentStatusAnnotationKey]).To(Equal("operational"))
	})
})

// Benchmarks — these are standard Go benchmarks (not Ginkgo), placed in the
// same package test file so they can reuse the helper constructors below.

func BenchmarkEndpointsToGroups_SmallFlat(b *testing.B) {
	mapping := &config.GroupMappingConfig{DefaultGroup: defaultGroupServices}
	eps := []*endpoint.Endpoint{
		newTestEndpoint(tFQDNAPI),
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
	mapping := &config.GroupMappingConfig{DefaultGroup: tValDefault}
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
		DefaultGroup: tValDefault,
		ByNamespace:  map[string]string{tEnvProd: "Prod", tEnvStaging: "Stage", "dev": "Dev"},
	}
	namespaces := []string{tEnvProd, tEnvStaging, "dev", "other"}
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
		Targets:    []string{tIP10001},
		Labels:     map[string]string{},
	}
}

func newTestEndpointWithLabels(dnsName string, labels map[string]string) *endpoint.Endpoint {
	return &endpoint.Endpoint{
		DNSName:    dnsName,
		RecordType: "A",
		Targets:    []string{tIP10001},
		Labels:     labels,
	}
}
