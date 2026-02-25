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

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
)

func TestMCP(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Suite")
}

// newCallToolRequest creates a CallToolRequest with the given arguments
func newCallToolRequest(name string, args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	}
}

// extractTextContent extracts text from a CallToolResult
func extractTextContent(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			return textContent.Text
		}
	}
	return ""
}

// isErrorResult checks if the result is an error
func isErrorResult(result *mcp.CallToolResult) bool {
	return result != nil && result.IsError
}

var _ = Describe("MCP Server", func() {
	var (
		scheme       *runtime.Scheme
		groupMapping *config.GroupMappingConfig
		ctx          context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(sreportalv1alpha1.AddToScheme(scheme)).To(Succeed())

		groupMapping = &config.GroupMappingConfig{
			DefaultGroup: "default",
			LabelKey:     "sreportal.io/group",
		}
	})

	Describe("Server creation", func() {
		It("should create server with all tools registered", func() {
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			server := New(k8sClient, groupMapping)
			Expect(server).NotTo(BeNil())
			Expect(server.mcpServer).NotTo(BeNil())
			Expect(server.client).NotTo(BeNil())
			Expect(server.groupMapping).To(Equal(groupMapping))
		})

		It("should handle nil groupMapping gracefully", func() {
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			server := New(k8sClient, nil)
			Expect(server).NotTo(BeNil())
			Expect(server.groupMapping).To(BeNil())
		})
	})

	Describe("handleSearchFQDNs", func() {
		var (
			dns1 *sreportalv1alpha1.DNS
			dns2 *sreportalv1alpha1.DNS
		)

		BeforeEach(func() {
			dns1 = &sreportalv1alpha1.DNS{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dns-1",
					Namespace: "default",
				},
				Spec: sreportalv1alpha1.DNSSpec{
					PortalRef: "main",
				},
				Status: sreportalv1alpha1.DNSStatus{
					Groups: []sreportalv1alpha1.FQDNGroupStatus{
						{
							Name:   "web",
							Source: "external-dns",
							FQDNs: []sreportalv1alpha1.FQDNStatus{
								{
									FQDN:        "api.example.com",
									Description: "Main API",
									RecordType:  "A",
									Targets:     []string{"192.168.1.1"},
									LastSeen:    metav1.NewTime(time.Now()),
								},
								{
									FQDN:       "web.example.com",
									RecordType: "A",
									Targets:    []string{"192.168.1.2"},
									LastSeen:   metav1.NewTime(time.Now()),
								},
							},
						},
						{
							Name:   "internal",
							Source: "manual",
							FQDNs: []sreportalv1alpha1.FQDNStatus{
								{
									FQDN:       "internal.example.com",
									RecordType: "A",
									Targets:    []string{"10.0.0.1"},
								},
							},
						},
					},
				},
			}

			dns2 = &sreportalv1alpha1.DNS{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dns-2",
					Namespace: "production",
				},
				Spec: sreportalv1alpha1.DNSSpec{
					PortalRef: "prod",
				},
				Status: sreportalv1alpha1.DNSStatus{
					Groups: []sreportalv1alpha1.FQDNGroupStatus{
						{
							Name:   "services",
							Source: "external-dns",
							FQDNs: []sreportalv1alpha1.FQDNStatus{
								{
									FQDN:       "prod-api.example.com",
									RecordType: "A",
									Targets:    []string{"10.10.10.1"},
								},
							},
						},
					},
				},
			}
		})

		Context("without filters", func() {
			It("should return all FQDNs when no filters are applied", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns1, dns2).
					WithStatusSubresource(dns1, dns2).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())

				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 4 FQDN(s)"))
				Expect(text).To(ContainSubstring("api.example.com"))
				Expect(text).To(ContainSubstring("web.example.com"))
				Expect(text).To(ContainSubstring("internal.example.com"))
				Expect(text).To(ContainSubstring("prod-api.example.com"))
			})
		})

		Context("with query filter", func() {
			It("should filter FQDNs by name substring", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns1, dns2).
					WithStatusSubresource(dns1, dns2).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{
					"query": "api",
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())

				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 2 FQDN(s)"))
				Expect(text).To(ContainSubstring("api.example.com"))
				Expect(text).To(ContainSubstring("prod-api.example.com"))
				Expect(text).NotTo(ContainSubstring("web.example.com"))
				Expect(text).NotTo(ContainSubstring("internal.example.com"))
			})

			It("should be case-insensitive", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns1).
					WithStatusSubresource(dns1).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{
					"query": "API",
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("api.example.com"))
			})
		})

		Context("with source filter", func() {
			It("should filter by manual source", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns1).
					WithStatusSubresource(dns1).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{
					"source": "manual",
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 1 FQDN(s)"))
				Expect(text).To(ContainSubstring("internal.example.com"))
				Expect(text).NotTo(ContainSubstring("api.example.com"))
			})

			It("should filter by external-dns source", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns1).
					WithStatusSubresource(dns1).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{
					"source": "external-dns",
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 2 FQDN(s)"))
				Expect(text).To(ContainSubstring("api.example.com"))
				Expect(text).To(ContainSubstring("web.example.com"))
				Expect(text).NotTo(ContainSubstring("internal.example.com"))
			})
		})

		Context("with group filter", func() {
			It("should filter by group name (case-insensitive)", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns1).
					WithStatusSubresource(dns1).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{
					"group": "WEB",
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 2 FQDN(s)"))
				Expect(text).To(ContainSubstring("api.example.com"))
				Expect(text).To(ContainSubstring("web.example.com"))
			})
		})

		Context("with namespace filter", func() {
			It("should filter by namespace", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns1, dns2).
					WithStatusSubresource(dns1, dns2).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{
					"namespace": "production",
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 1 FQDN(s)"))
				Expect(text).To(ContainSubstring("prod-api.example.com"))
			})
		})

		Context("with multiple filters", func() {
			It("should combine query and source filters", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns1, dns2).
					WithStatusSubresource(dns1, dns2).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{
					"query":  "example",
					"source": "manual",
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 1 FQDN(s)"))
				Expect(text).To(ContainSubstring("internal.example.com"))
			})
		})

		Context("with no results", func() {
			It("should return appropriate message when no FQDNs match", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns1).
					WithStatusSubresource(dns1).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{
					"query": "nonexistent",
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(Equal("No FQDNs found matching the search criteria."))
			})

			It("should return appropriate message when no DNS resources exist", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(Equal("No FQDNs found matching the search criteria."))
			})
		})

		Context("with client errors", func() {
			It("should return error when listing DNS fails", func() {
				expectedErr := errors.New("connection refused")
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
							if _, ok := list.(*sreportalv1alpha1.DNSList); ok {
								return expectedErr
							}
							return nil
						},
					}).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeTrue())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("failed to list DNS resources"))
			})
		})

		Context("with duplicate FQDNs", func() {
			It("should deduplicate FQDNs across resources", func() {
				dns1Copy := dns1.DeepCopy()
				dns1Copy.Name = "test-dns-copy"
				// Same FQDNs in status

				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns1, dns1Copy).
					WithStatusSubresource(dns1, dns1Copy).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				// Should only count each unique FQDN once
				Expect(text).To(ContainSubstring("Found 3 FQDN(s)"))
			})
		})
	})

	Describe("handleListPortals", func() {
		Context("with existing portals", func() {
			It("should list all portals with their details", func() {
				portal1 := &sreportalv1alpha1.Portal{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main",
						Namespace: "sreportal-system",
					},
					Spec: sreportalv1alpha1.PortalSpec{
						Title: "Main Portal",
						Main:  true,
					},
					Status: sreportalv1alpha1.PortalStatus{
						Ready: true,
					},
				}

				portal2 := &sreportalv1alpha1.Portal{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dev",
						Namespace: "sreportal-system",
					},
					Spec: sreportalv1alpha1.PortalSpec{
						Title:   "Dev Portal",
						SubPath: "dev",
					},
					Status: sreportalv1alpha1.PortalStatus{
						Ready: false,
					},
				}

				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(portal1, portal2).
					WithStatusSubresource(portal1, portal2).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("list_portals", map[string]any{})

				result, err := server.handleListPortals(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())

				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 2 portal(s)"))
				Expect(text).To(ContainSubstring("Main Portal"))
				Expect(text).To(ContainSubstring("Dev Portal"))
				Expect(text).To(ContainSubstring(`"main": true`))
				Expect(text).To(ContainSubstring(`"subPath": "dev"`))
			})

			It("should include remote URL when configured", func() {
				portal := &sreportalv1alpha1.Portal{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "remote",
						Namespace: "sreportal-system",
					},
					Spec: sreportalv1alpha1.PortalSpec{
						Title: "Remote Portal",
						Remote: &sreportalv1alpha1.RemotePortalSpec{
							URL: "https://remote.example.com",
						},
					},
					Status: sreportalv1alpha1.PortalStatus{
						Ready: true,
					},
				}

				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(portal).
					WithStatusSubresource(portal).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("list_portals", map[string]any{})

				result, err := server.handleListPortals(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("https://remote.example.com"))
			})
		})

		Context("with no portals", func() {
			It("should return appropriate message", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("list_portals", map[string]any{})

				result, err := server.handleListPortals(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(Equal("No portals found."))
			})
		})

		Context("with client errors", func() {
			It("should return error when listing portals fails", func() {
				expectedErr := errors.New("connection refused")
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
							return expectedErr
						},
					}).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("list_portals", map[string]any{})

				result, err := server.handleListPortals(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeTrue())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("failed to list Portal resources"))
			})
		})
	})

	Describe("handleGetFQDNDetails", func() {
		var dns *sreportalv1alpha1.DNS

		BeforeEach(func() {
			dns = &sreportalv1alpha1.DNS{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dns",
					Namespace: "default",
				},
				Spec: sreportalv1alpha1.DNSSpec{
					PortalRef: "main",
				},
				Status: sreportalv1alpha1.DNSStatus{
					Groups: []sreportalv1alpha1.FQDNGroupStatus{
						{
							Name:        "api",
							Description: "API services",
							Source:      "external-dns",
							FQDNs: []sreportalv1alpha1.FQDNStatus{
								{
									FQDN:        "api.example.com",
									Description: "Main API endpoint",
									RecordType:  "A",
									Targets:     []string{"192.168.1.1", "192.168.1.2"},
									LastSeen:    metav1.NewTime(time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)),
								},
								{
									FQDN:       "api-v2.example.com",
									RecordType: "CNAME",
									Targets:    []string{"api.example.com"},
								},
							},
						},
					},
				},
			}
		})

		Context("with existing FQDN", func() {
			It("should return full details for the FQDN", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns).
					WithStatusSubresource(dns).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("get_fqdn_details", map[string]any{
					"fqdn": "api.example.com",
				})

				result, err := server.handleGetFQDNDetails(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())

				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("FQDN details for 'api.example.com'"))
				Expect(text).To(ContainSubstring(`"name": "api.example.com"`))
				Expect(text).To(ContainSubstring(`"source": "external-dns"`))
				Expect(text).To(ContainSubstring(`"group": "api"`))
				Expect(text).To(ContainSubstring(`"description": "Main API endpoint"`))
				Expect(text).To(ContainSubstring(`"record_type": "A"`))
				Expect(text).To(ContainSubstring("192.168.1.1"))
				Expect(text).To(ContainSubstring("192.168.1.2"))
				Expect(text).To(ContainSubstring(`"portal": "main"`))
				Expect(text).To(ContainSubstring(`"dns_resource": "default/test-dns"`))
				Expect(text).To(ContainSubstring("2026-01-15"))
			})

			It("should be case-insensitive", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns).
					WithStatusSubresource(dns).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("get_fqdn_details", map[string]any{
					"fqdn": "API.EXAMPLE.COM",
				})

				result, err := server.handleGetFQDNDetails(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("api.example.com"))
			})

			It("should handle trailing dot in FQDN", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns).
					WithStatusSubresource(dns).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("get_fqdn_details", map[string]any{
					"fqdn": "api.example.com.",
				})

				result, err := server.handleGetFQDNDetails(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("api.example.com"))
			})
		})

		Context("with non-existing FQDN", func() {
			It("should return not found message", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns).
					WithStatusSubresource(dns).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("get_fqdn_details", map[string]any{
					"fqdn": "nonexistent.example.com",
				})

				result, err := server.handleGetFQDNDetails(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())
				text := extractTextContent(result)
				Expect(text).To(Equal("FQDN 'nonexistent.example.com' not found."))
			})
		})

		Context("with missing required parameter", func() {
			It("should return error when fqdn is not provided", func() {
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("get_fqdn_details", map[string]any{})

				result, err := server.handleGetFQDNDetails(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeTrue())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("fqdn parameter is required"))
			})
		})

		Context("with client errors", func() {
			It("should return error when listing DNS fails", func() {
				expectedErr := errors.New("connection refused")
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
							if _, ok := list.(*sreportalv1alpha1.DNSList); ok {
								return expectedErr
							}
							return nil
						},
					}).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("get_fqdn_details", map[string]any{
					"fqdn": "api.example.com",
				})

				result, err := server.handleGetFQDNDetails(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeTrue())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("failed to list DNS resources"))
			})
		})
	})

	Describe("JSON output format", func() {
		It("should produce valid JSON in search results", func() {
			dns := &sreportalv1alpha1.DNS{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dns",
					Namespace: "default",
				},
				Spec: sreportalv1alpha1.DNSSpec{
					PortalRef: "main",
				},
				Status: sreportalv1alpha1.DNSStatus{
					Groups: []sreportalv1alpha1.FQDNGroupStatus{
						{
							Name:   "web",
							Source: "external-dns",
							FQDNs: []sreportalv1alpha1.FQDNStatus{
								{
									FQDN:       "api.example.com",
									RecordType: "A",
									Targets:    []string{"192.168.1.1"},
								},
							},
						},
					},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(dns).
				WithStatusSubresource(dns).
				Build()

			server := New(k8sClient, groupMapping)
			request := newCallToolRequest("search_fqdns", map[string]any{})

			result, err := server.handleSearchFQDNs(ctx, request)

			Expect(err).NotTo(HaveOccurred())
			text := extractTextContent(result)

			// Extract JSON part (after the "Found X FQDN(s):" line)
			jsonStart := strings.Index(text, "[")
			Expect(jsonStart).To(BeNumerically(">", 0))
			jsonStr := text[jsonStart:]

			var results []FQDNResult
			err = json.Unmarshal([]byte(jsonStr), &results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].Name).To(Equal("api.example.com"))
		})

		It("should produce valid JSON in portal list results", func() {
			portal := &sreportalv1alpha1.Portal{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "main",
					Namespace: "sreportal-system",
				},
				Spec: sreportalv1alpha1.PortalSpec{
					Title: "Main Portal",
					Main:  true,
				},
				Status: sreportalv1alpha1.PortalStatus{
					Ready: true,
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(portal).
				WithStatusSubresource(portal).
				Build()

			server := New(k8sClient, groupMapping)
			request := newCallToolRequest("list_portals", map[string]any{})

			result, err := server.handleListPortals(ctx, request)

			Expect(err).NotTo(HaveOccurred())
			text := extractTextContent(result)

			jsonStart := strings.Index(text, "[")
			Expect(jsonStart).To(BeNumerically(">", 0))
			jsonStr := text[jsonStart:]

			var results []PortalResult
			err = json.Unmarshal([]byte(jsonStr), &results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].Title).To(Equal("Main Portal"))
			Expect(results[0].Main).To(BeTrue())
		})

		It("should produce valid JSON in FQDN details results", func() {
			dns := &sreportalv1alpha1.DNS{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dns",
					Namespace: "default",
				},
				Spec: sreportalv1alpha1.DNSSpec{
					PortalRef: "main",
				},
				Status: sreportalv1alpha1.DNSStatus{
					Groups: []sreportalv1alpha1.FQDNGroupStatus{
						{
							Name:   "api",
							Source: "external-dns",
							FQDNs: []sreportalv1alpha1.FQDNStatus{
								{
									FQDN:       "api.example.com",
									RecordType: "A",
									Targets:    []string{"192.168.1.1"},
								},
							},
						},
					},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(dns).
				WithStatusSubresource(dns).
				Build()

			server := New(k8sClient, groupMapping)
			request := newCallToolRequest("get_fqdn_details", map[string]any{
				"fqdn": "api.example.com",
			})

			result, err := server.handleGetFQDNDetails(ctx, request)

			Expect(err).NotTo(HaveOccurred())
			text := extractTextContent(result)

			jsonStart := strings.Index(text, "{")
			Expect(jsonStart).To(BeNumerically(">", 0))
			jsonStr := text[jsonStart:]

			var details FQDNDetails
			err = json.Unmarshal([]byte(jsonStr), &details)
			Expect(err).NotTo(HaveOccurred())
			Expect(details.Name).To(Equal("api.example.com"))
			Expect(details.RecordType).To(Equal("A"))
		})
	})

	Describe("DNSRecord integration", func() {
		Context("search_fqdns reads only from DNS status", func() {
			It("should only return FQDNs present in DNS.Status.Groups", func() {
				dns := &sreportalv1alpha1.DNS{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-dns",
						Namespace: "default",
					},
					Spec: sreportalv1alpha1.DNSSpec{
						PortalRef: "main",
					},
					Status: sreportalv1alpha1.DNSStatus{
						Groups: []sreportalv1alpha1.FQDNGroupStatus{
							{
								Name:   "web",
								Source: "external-dns",
								FQDNs: []sreportalv1alpha1.FQDNStatus{
									{
										FQDN:       "svc.example.com",
										RecordType: "A",
										Targets:    []string{"10.0.0.5"},
									},
								},
							},
						},
					},
				}

				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dns).
					WithStatusSubresource(dns).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 1 FQDN(s)"))
				Expect(text).To(ContainSubstring("svc.example.com"))
			})

			It("should ignore DNSRecords not yet aggregated into DNS status", func() {
				// Only a DNSRecord exists, no DNS resource with aggregated status yet
				dnsRecord := &sreportalv1alpha1.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-dnsrecord",
						Namespace: "default",
					},
					Spec: sreportalv1alpha1.DNSRecordSpec{
						SourceType: "service",
						PortalRef:  "main",
					},
					Status: sreportalv1alpha1.DNSRecordStatus{
						Endpoints: []sreportalv1alpha1.EndpointStatus{
							{
								DNSName:    "not-yet-aggregated.example.com",
								RecordType: "A",
								Targets:    []string{"10.0.0.5"},
							},
						},
					},
				}

				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dnsRecord).
					WithStatusSubresource(dnsRecord).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("search_fqdns", map[string]any{})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(Equal("No FQDNs found matching the search criteria."))
			})
		})

		Context("get_fqdn_details with DNSRecords", func() {
			It("should find FQDN details from DNSRecord when not in DNS", func() {
				dnsRecord := &sreportalv1alpha1.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-dnsrecord",
						Namespace: "default",
					},
					Spec: sreportalv1alpha1.DNSRecordSpec{
						SourceType: "ingress",
						PortalRef:  "main",
					},
					Status: sreportalv1alpha1.DNSRecordStatus{
						Endpoints: []sreportalv1alpha1.EndpointStatus{
							{
								DNSName:    "ingress.example.com",
								RecordType: "A",
								Targets:    []string{"10.0.0.10"},
								Labels: map[string]string{
									"sreportal.io/groups": "ingress-group",
								},
							},
						},
					},
				}

				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(dnsRecord).
					WithStatusSubresource(dnsRecord).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("get_fqdn_details", map[string]any{
					"fqdn": "ingress.example.com",
				})

				result, err := server.handleGetFQDNDetails(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("ingress.example.com"))
				Expect(text).To(ContainSubstring("10.0.0.10"))
			})

			It("should return error when listing DNSRecords fails in get_fqdn_details", func() {
				expectedErr := errors.New("dnsrecord list error")
				k8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
							if _, ok := list.(*sreportalv1alpha1.DNSRecordList); ok {
								return expectedErr
							}
							return c.List(ctx, list, opts...)
						},
					}).
					Build()

				server := New(k8sClient, groupMapping)
				request := newCallToolRequest("get_fqdn_details", map[string]any{
					"fqdn": "test.example.com",
				})

				result, err := server.handleGetFQDNDetails(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeTrue())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("failed to list DNSRecord resources"))
			})
		})
	})
})
