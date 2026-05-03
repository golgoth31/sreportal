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
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	domainmetrics "github.com/golgoth31/sreportal/internal/domain/metrics"
	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	dnsstore "github.com/golgoth31/sreportal/internal/readstore/dns"
	imagestore "github.com/golgoth31/sreportal/internal/readstore/image"
	portalstore "github.com/golgoth31/sreportal/internal/readstore/portal"
	releasestore "github.com/golgoth31/sreportal/internal/readstore/release"
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

// seedDNSStore creates and populates an FQDNStore with test data equivalent
// to the old CRD-based setup (dns1 in default/test-dns-1, dns2 in production/test-dns-2).
func seedDNSStore() *dnsstore.FQDNStore {
	store := dnsstore.NewFQDNStore(nil)
	ctx := context.Background()
	now := time.Now()

	// dns1: default/test-dns-1 with portalRef portalMain
	_ = store.Replace(ctx, "default/test-dns-1", []domaindns.FQDNView{
		{
			Name: fqdnAPI, Source: domaindns.SourceExternalDNS,
			Groups: []string{fqdnWeb}, Description: "Main API",
			RecordType: "A", Targets: []string{ip192dot1},
			LastSeen: now, PortalName: portalMain, Namespace: nsDefault,
		},
		{
			Name: "web.example.com", Source: domaindns.SourceExternalDNS,
			Groups: []string{fqdnWeb}, RecordType: "A",
			Targets: []string{ip192dot2}, LastSeen: now,
			PortalName: portalMain, Namespace: nsDefault,
		},
		{
			Name: "internal.example.com", Source: domaindns.SourceManual,
			Groups: []string{"internal"}, RecordType: "A",
			Targets:    []string{ip10dot1},
			PortalName: portalMain, Namespace: nsDefault,
		},
	})

	// dns2: production/test-dns-2 with portalRef "prod"
	_ = store.Replace(ctx, "production/test-dns-2", []domaindns.FQDNView{
		{
			Name: "prod-api.example.com", Source: domaindns.SourceExternalDNS,
			Groups: []string{"services"}, RecordType: "A",
			Targets:    []string{"10.10.10.1"},
			PortalName: "prod", Namespace: "production",
		},
	})

	return store
}

// emptyPortalStore returns an empty PortalStore for tests that don't need portal data.
func emptyPortalStore() *portalstore.PortalStore {
	return portalstore.NewPortalStore()
}

var _ = Describe("MCP Server", func() {
	var (
		scheme *runtime.Scheme
		ctx    context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(sreportalv1alpha1.AddToScheme(scheme)).To(Succeed())
	})

	Describe("DNSServer creation", func() {
		It("should create server with all tools registered", func() {
			store := dnsstore.NewFQDNStore(nil)
			pStore := emptyPortalStore()

			dnsServer := NewDNSServer(store, pStore)
			Expect(dnsServer).NotTo(BeNil())
			Expect(dnsServer.mcpServer).NotTo(BeNil())
			Expect(dnsServer.fqdnReader).NotTo(BeNil())
			Expect(dnsServer.portalReader).NotTo(BeNil())
		})
	})

	Describe("handleSearchFQDNs", func() {
		Context("without filters", func() {
			It("should return all FQDNs when no filters are applied", func() {
				store := seedDNSStore()
				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("search_fqdns", map[string]any{})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())

				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 4 FQDN(s)"))
				Expect(text).To(ContainSubstring(fqdnAPI))
				Expect(text).To(ContainSubstring("web.example.com"))
				Expect(text).To(ContainSubstring("internal.example.com"))
				Expect(text).To(ContainSubstring("prod-api.example.com"))
			})
		})

		Context("with query filter", func() {
			It("should filter FQDNs by name substring", func() {
				store := seedDNSStore()
				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("search_fqdns", map[string]any{
					keyQuery: keyAPI,
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())

				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 2 FQDN(s)"))
				Expect(text).To(ContainSubstring(fqdnAPI))
				Expect(text).To(ContainSubstring("prod-api.example.com"))
				Expect(text).NotTo(ContainSubstring("web.example.com"))
				Expect(text).NotTo(ContainSubstring("internal.example.com"))
			})

			It("should be case-insensitive", func() {
				store := seedDNSStore()
				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("search_fqdns", map[string]any{
					keyQuery: "API",
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring(fqdnAPI))
			})
		})

		Context("with source filter", func() {
			It("should filter by manual source", func() {
				store := seedDNSStore()
				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("search_fqdns", map[string]any{
					keySource: "manual",
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 1 FQDN(s)"))
				Expect(text).To(ContainSubstring("internal.example.com"))
				Expect(text).NotTo(ContainSubstring(fqdnAPI))
			})

			It("should filter by external-dns source", func() {
				store := seedDNSStore()
				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("search_fqdns", map[string]any{
					keySource: "external-dns",
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				// 3 external-dns FQDNs: api, web (from dns1) + prod-api (from dns2)
				Expect(text).To(ContainSubstring("Found 3 FQDN(s)"))
				Expect(text).To(ContainSubstring(fqdnAPI))
				Expect(text).To(ContainSubstring("web.example.com"))
				Expect(text).NotTo(ContainSubstring("internal.example.com"))
			})
		})

		Context("with group filter", func() {
			It("should filter by exact group name", func() {
				store := seedDNSStore()
				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("search_fqdns", map[string]any{
					"group": fqdnWeb,
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("Found 2 FQDN(s)"))
				Expect(text).To(ContainSubstring(fqdnAPI))
				Expect(text).To(ContainSubstring("web.example.com"))
			})
		})

		Context("with namespace filter", func() {
			It("should filter by namespace", func() {
				store := seedDNSStore()
				server := NewDNSServer(store, emptyPortalStore())
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
				store := seedDNSStore()
				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("search_fqdns", map[string]any{
					keyQuery:  "example",
					keySource: "manual",
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
				store := seedDNSStore()
				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("search_fqdns", map[string]any{
					keyQuery: "nonexistent",
				})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(Equal("No FQDNs found matching the search criteria."))
			})

			It("should return appropriate message when store is empty", func() {
				store := dnsstore.NewFQDNStore(nil)
				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("search_fqdns", map[string]any{})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(Equal("No FQDNs found matching the search criteria."))
			})
		})

		Context("with sync status", func() {
			It("should include sync_status in results", func() {
				store := dnsstore.NewFQDNStore(nil)
				_ = store.Replace(ctx, "default/test-dns-sync", []domaindns.FQDNView{
					{
						Name: "synced.example.com", Source: domaindns.SourceExternalDNS,
						Groups: []string{fqdnWeb}, RecordType: "A",
						Targets: []string{ip10dot1}, SyncStatus: "sync",
						PortalName: portalMain, Namespace: nsDefault,
					},
					{
						Name: "drifted.example.com", Source: domaindns.SourceExternalDNS,
						Groups: []string{fqdnWeb}, RecordType: "A",
						Targets: []string{"10.0.0.2"}, SyncStatus: "notsync",
						PortalName: portalMain, Namespace: nsDefault,
					},
				})

				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("search_fqdns", map[string]any{})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)

				jsonStart := strings.Index(text, "[")
				Expect(jsonStart).To(BeNumerically(">", 0))
				var results []FQDNResult
				Expect(json.Unmarshal([]byte(text[jsonStart:]), &results)).To(Succeed())

				Expect(results).To(HaveLen(2))
				// Sorted by name: drifted < synced
				Expect(results[0].SyncStatus).To(Equal("notsync"))
				Expect(results[1].SyncStatus).To(Equal("sync"))
			})
		})

		Context("with duplicate FQDNs across resources", func() {
			It("should deduplicate FQDNs by name", func() {
				store := dnsstore.NewFQDNStore(nil)
				views := []domaindns.FQDNView{
					{
						Name: fqdnAPI, Source: domaindns.SourceExternalDNS,
						Groups: []string{fqdnWeb}, RecordType: "A",
						Targets:    []string{ip192dot1},
						PortalName: portalMain, Namespace: nsDefault,
					},
					{
						Name: "web.example.com", Source: domaindns.SourceExternalDNS,
						Groups: []string{fqdnWeb}, RecordType: "A",
						Targets:    []string{ip192dot2},
						PortalName: portalMain, Namespace: nsDefault,
					},
					{
						Name: "internal.example.com", Source: domaindns.SourceManual,
						Groups: []string{"internal"}, RecordType: "A",
						Targets:    []string{ip10dot1},
						PortalName: portalMain, Namespace: nsDefault,
					},
				}
				// Same FQDNs in two different resources
				_ = store.Replace(ctx, "default/test-dns-1", views)
				_ = store.Replace(ctx, "default/test-dns-copy", views)

				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("search_fqdns", map[string]any{})

				result, err := server.handleSearchFQDNs(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				// handleSearchFQDNs deduplicates by name, so only 3 unique FQDNs
				Expect(text).To(ContainSubstring("Found 3 FQDN(s)"))
			})
		})
	})

	Describe("handleListPortals", func() {
		Context("with existing portals", func() {
			It("should list all portals with their details", func() {
				pStore := portalstore.NewPortalStore()
				_ = pStore.Replace(ctx, "sreportal-system/main", domainportal.PortalView{
					Name: portalMain, Namespace: nsSystem,
					Title: "Main Portal", Main: true, Ready: true,
				})
				_ = pStore.Replace(ctx, "sreportal-system/dev", domainportal.PortalView{
					Name: nsDev, Namespace: nsSystem,
					Title: "Dev Portal", SubPath: nsDev, Ready: false,
				})

				store := dnsstore.NewFQDNStore(nil)
				server := NewDNSServer(store, pStore)
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
				pStore := portalstore.NewPortalStore()
				_ = pStore.Replace(ctx, "sreportal-system/remote", domainportal.PortalView{
					Name: "remote", Namespace: nsSystem,
					Title: "Remote Portal", IsRemote: true, Ready: true,
					URL: "https://remote.example.com",
				})

				store := dnsstore.NewFQDNStore(nil)
				server := NewDNSServer(store, pStore)
				request := newCallToolRequest("list_portals", map[string]any{})

				result, err := server.handleListPortals(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("https://remote.example.com"))
			})

			It("should include remoteSync with lastSyncError when remote sync failed", func() {
				pStore := portalstore.NewPortalStore()
				_ = pStore.Replace(ctx, "sreportal-system/remote", domainportal.PortalView{
					Name: "remote", Namespace: nsSystem,
					Title: "Remote Portal", IsRemote: true, Ready: true,
					URL: "https://remote.example.com",
					RemoteSync: &domainportal.RemoteSyncView{
						LastSyncError: "remote unreachable: dial tcp: connection refused",
					},
				})

				store := dnsstore.NewFQDNStore(nil)
				server := NewDNSServer(store, pStore)
				request := newCallToolRequest("list_portals", map[string]any{})

				result, err := server.handleListPortals(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring(`"lastSyncError": "remote unreachable: dial tcp: connection refused"`))
				Expect(text).To(ContainSubstring(`"remoteSync"`))
			})
		})

		Context("with no portals", func() {
			It("should return appropriate message", func() {
				store := dnsstore.NewFQDNStore(nil)
				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("list_portals", map[string]any{})

				result, err := server.handleListPortals(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(Equal("No portals found."))
			})
		})
	})

	Describe("handleGetFQDNDetails", func() {
		Context("with existing FQDN", func() {
			It("should return full details for the FQDN", func() {
				store := dnsstore.NewFQDNStore(nil)
				_ = store.Replace(ctx, "default/test-dns", []domaindns.FQDNView{
					{
						Name: fqdnAPI, Source: domaindns.SourceExternalDNS,
						Groups: []string{keyAPI}, Description: "Main API endpoint",
						RecordType: "A", Targets: []string{ip192dot1, ip192dot2},
						LastSeen:   time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
						PortalName: portalMain, Namespace: nsDefault,
					},
					{
						Name: "api-v2.example.com", Source: domaindns.SourceExternalDNS,
						Groups: []string{keyAPI}, RecordType: "CNAME",
						Targets:    []string{fqdnAPI},
						PortalName: portalMain, Namespace: nsDefault,
					},
				})

				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("get_fqdn_details", map[string]any{
					keyFqdn: fqdnAPI,
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
				Expect(text).To(ContainSubstring(ip192dot1))
				Expect(text).To(ContainSubstring(ip192dot2))
				Expect(text).To(ContainSubstring(`"portal": "main"`))
				Expect(text).To(ContainSubstring(`"dns_resource": "default/main"`))
				Expect(text).To(ContainSubstring("2026-01-15"))
			})

			It("should be case-insensitive", func() {
				store := dnsstore.NewFQDNStore(nil)
				_ = store.Replace(ctx, "default/test-dns", []domaindns.FQDNView{
					{
						Name: fqdnAPI, Source: domaindns.SourceExternalDNS,
						Groups: []string{keyAPI}, RecordType: "A",
						Targets:    []string{ip192dot1},
						PortalName: portalMain, Namespace: nsDefault,
					},
				})

				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("get_fqdn_details", map[string]any{
					keyFqdn: "API.EXAMPLE.COM",
				})

				result, err := server.handleGetFQDNDetails(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring(fqdnAPI))
			})

			It("should handle trailing dot in FQDN", func() {
				store := dnsstore.NewFQDNStore(nil)
				_ = store.Replace(ctx, "default/test-dns", []domaindns.FQDNView{
					{
						Name: fqdnAPI, Source: domaindns.SourceExternalDNS,
						Groups: []string{keyAPI}, RecordType: "A",
						Targets:    []string{ip192dot1},
						PortalName: portalMain, Namespace: nsDefault,
					},
				})

				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("get_fqdn_details", map[string]any{
					keyFqdn: "api.example.com.",
				})

				result, err := server.handleGetFQDNDetails(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring(fqdnAPI))
			})
		})

		Context("with sync status", func() {
			It("should include sync_status in details", func() {
				store := dnsstore.NewFQDNStore(nil)
				_ = store.Replace(ctx, "default/test-dns", []domaindns.FQDNView{
					{
						Name: fqdnAPI, Source: domaindns.SourceExternalDNS,
						Groups: []string{keyAPI}, RecordType: "A",
						Targets: []string{ip192dot1}, SyncStatus: "sync",
						PortalName: portalMain, Namespace: nsDefault,
					},
				})

				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("get_fqdn_details", map[string]any{
					keyFqdn: fqdnAPI,
				})

				result, err := server.handleGetFQDNDetails(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())

				text := extractTextContent(result)
				jsonStart := strings.Index(text, "{")
				Expect(jsonStart).To(BeNumerically(">", 0))
				var details FQDNDetails
				Expect(json.Unmarshal([]byte(text[jsonStart:]), &details)).To(Succeed())
				Expect(details.SyncStatus).To(Equal("sync"))
			})
		})

		Context("with non-existing FQDN", func() {
			It("should return not found message", func() {
				store := seedDNSStore()
				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("get_fqdn_details", map[string]any{
					keyFqdn: "nonexistent.example.com",
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
				store := dnsstore.NewFQDNStore(nil)
				server := NewDNSServer(store, emptyPortalStore())
				request := newCallToolRequest("get_fqdn_details", map[string]any{})

				result, err := server.handleGetFQDNDetails(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeTrue())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("fqdn parameter is required"))
			})
		})
	})

	Describe("JSON output format", func() {
		It("should produce valid JSON in search results", func() {
			store := dnsstore.NewFQDNStore(nil)
			_ = store.Replace(ctx, "default/test-dns", []domaindns.FQDNView{
				{
					Name: fqdnAPI, Source: domaindns.SourceExternalDNS,
					Groups: []string{fqdnWeb}, RecordType: "A",
					Targets:    []string{ip192dot1},
					PortalName: portalMain, Namespace: nsDefault,
				},
			})

			server := NewDNSServer(store, emptyPortalStore())
			request := newCallToolRequest("search_fqdns", map[string]any{})

			result, err := server.handleSearchFQDNs(ctx, request)

			Expect(err).NotTo(HaveOccurred())
			text := extractTextContent(result)

			jsonStart := strings.Index(text, "[")
			Expect(jsonStart).To(BeNumerically(">", 0))
			jsonStr := text[jsonStart:]

			var results []FQDNResult
			err = json.Unmarshal([]byte(jsonStr), &results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].Name).To(Equal(fqdnAPI))
		})

		It("should produce valid JSON in portal list results", func() {
			pStore := portalstore.NewPortalStore()
			_ = pStore.Replace(ctx, "sreportal-system/main", domainportal.PortalView{
				Name: portalMain, Namespace: nsSystem,
				Title: "Main Portal", Main: true, Ready: true,
			})

			store := dnsstore.NewFQDNStore(nil)
			server := NewDNSServer(store, pStore)
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
			store := dnsstore.NewFQDNStore(nil)
			_ = store.Replace(ctx, "default/test-dns", []domaindns.FQDNView{
				{
					Name: fqdnAPI, Source: domaindns.SourceExternalDNS,
					Groups: []string{keyAPI}, RecordType: "A",
					Targets:    []string{ip192dot1},
					PortalName: portalMain, Namespace: nsDefault,
				},
			})

			server := NewDNSServer(store, emptyPortalStore())
			request := newCallToolRequest("get_fqdn_details", map[string]any{
				keyFqdn: fqdnAPI,
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
			Expect(details.Name).To(Equal(fqdnAPI))
			Expect(details.RecordType).To(Equal("A"))
		})
	})

	Describe("MetricsServer", func() {
		Describe("creation", func() {
			It("should create server with list_metrics tool registered", func() {
				reg := prometheus.NewRegistry()
				server := NewMetricsServer(reg)
				Expect(server).NotTo(BeNil())
				Expect(server.mcpServer).NotTo(BeNil())
				Expect(server.gatherer).NotTo(BeNil())
			})
		})

		Describe("handleListMetrics", func() {
			It("should return all sreportal metrics", func() {
				reg := prometheus.NewRegistry()
				gauge := prometheus.NewGauge(prometheus.GaugeOpts{
					Namespace: mNsSreport,
					Subsystem: mSubDNS,
					Name:      mFqdnTotal,
					Help:      mFqdnHelp,
				})
				reg.MustRegister(gauge)
				gauge.Set(42)

				server := NewMetricsServer(reg)
				request := newCallToolRequest("list_metrics", map[string]any{})

				result, err := server.handleListMetrics(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("1 metric family"))

				var families []domainmetrics.MetricFamily
				jsonStr := text[strings.Index(text, "["):]
				Expect(json.Unmarshal([]byte(jsonStr), &families)).To(Succeed())
				Expect(families).To(HaveLen(1))
				Expect(families[0].Name).To(Equal("sreportal_dns_fqdns_total"))
				Expect(families[0].Metrics[0].Value).To(Equal(42.0))
			})

			It("should filter by subsystem", func() {
				reg := prometheus.NewRegistry()
				dnsGauge := prometheus.NewGauge(prometheus.GaugeOpts{
					Namespace: mNsSreport,
					Subsystem: mSubDNS,
					Name:      mFqdnTotal,
					Help:      mFqdnHelp,
				})
				httpCounter := prometheus.NewCounter(prometheus.CounterOpts{
					Namespace: mNsSreport,
					Subsystem: "http",
					Name:      "requests_total",
					Help:      "Total HTTP requests",
				})
				reg.MustRegister(dnsGauge, httpCounter)

				server := NewMetricsServer(reg)
				request := newCallToolRequest("list_metrics", map[string]any{
					"subsystem": mSubDNS,
				})

				result, err := server.handleListMetrics(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("sreportal_dns_fqdns_total"))
				Expect(text).NotTo(ContainSubstring("sreportal_http_requests_total"))
			})

			It("should filter by search", func() {
				reg := prometheus.NewRegistry()
				fqdnGauge := prometheus.NewGauge(prometheus.GaugeOpts{
					Namespace: mNsSreport,
					Subsystem: mSubDNS,
					Name:      mFqdnTotal,
					Help:      mFqdnHelp,
				})
				groupGauge := prometheus.NewGauge(prometheus.GaugeOpts{
					Namespace: mNsSreport,
					Subsystem: mSubDNS,
					Name:      "groups_total",
					Help:      "Total groups",
				})
				reg.MustRegister(fqdnGauge, groupGauge)

				server := NewMetricsServer(reg)
				request := newCallToolRequest("list_metrics", map[string]any{
					"search": keyFqdn,
				})

				result, err := server.handleListMetrics(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("sreportal_dns_fqdns_total"))
				Expect(text).NotTo(ContainSubstring("sreportal_dns_groups_total"))
			})

			It("should return message when no metrics match", func() {
				reg := prometheus.NewRegistry()
				server := NewMetricsServer(reg)
				request := newCallToolRequest("list_metrics", map[string]any{})

				result, err := server.handleListMetrics(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("No metrics found"))
			})
		})
	})

	Describe("ReleasesServer", func() {
		var store *releasestore.ReleaseStore

		BeforeEach(func() {
			store = releasestore.NewReleaseStore()
		})

		Describe("creation", func() {
			It("should create server with tools registered", func() {
				server := NewReleasesServer(store)
				Expect(server).NotTo(BeNil())
				Expect(server.mcpServer).NotTo(BeNil())
				Expect(server.reader).NotTo(BeNil())
			})
		})

		Describe("handleListReleases", func() {
			It("should list releases for a day", func() {
				_ = store.Replace(ctx, "default/release-2026-03-21", []domainrelease.EntryView{
					{
						PortalRef: portalMain,
						Day:       "2026-03-21",
						Type:      "deployment",
						Version:   "v1.0.0",
						Origin:    "ci/cd",
						Date:      time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
						Author:    "alice",
						Message:   "ship",
						Link:      "https://example.com/release",
					},
				})
				server := NewReleasesServer(store)

				listReq := newCallToolRequest("list_releases", map[string]any{
					"day": "2026-03-21",
				})
				result, err := server.handleListReleases(ctx, listReq)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("2026-03-21"))
				Expect(text).To(ContainSubstring("deployment"))
				Expect(text).To(ContainSubstring("v1.0.0"))
			})

			It("should return no releases message when empty", func() {
				server := NewReleasesServer(store)
				request := newCallToolRequest("list_releases", map[string]any{})

				result, err := server.handleListReleases(ctx, request)

				Expect(err).NotTo(HaveOccurred())
				Expect(isErrorResult(result)).To(BeFalse())
				text := extractTextContent(result)
				Expect(text).To(ContainSubstring("No releases found"))
			})
		})
	})

	Describe("ImageServer", func() {
		var store *imagestore.Store

		BeforeEach(func() {
			store = imagestore.NewStore()
		})

		Describe("creation", func() {
			It("should create server with list_images tool registered", func() {
				s := NewImageServer(store)
				Expect(s).NotTo(BeNil())
				Expect(s.mcpServer).NotTo(BeNil())
				Expect(s.reader).NotTo(BeNil())
			})
		})

		Describe("handleListImages", func() {
			Context("with images in the store", func() {
				BeforeEach(func() {
					_ = store.ReplaceAll(ctx, portalMain, map[domainimage.WorkloadKey][]domainimage.ImageView{
						{Kind: kindDeploy, Namespace: nsDefault, Name: keyAPI}: {
							{
								PortalRef:  portalMain,
								Registry:   "docker.io",
								Repository: "myorg/api",
								Tag:        "v1.2.3",
								TagType:    domainimage.TagTypeSemver,
								Workloads: []domainimage.WorkloadRef{
									{Kind: kindDeploy, Namespace: nsDefault, Name: keyAPI, Container: keyAPI},
								},
							},
						},
						{Kind: kindDeploy, Namespace: nsDefault, Name: nameWorker}: {
							{
								PortalRef:  portalMain,
								Registry:   "ghcr.io",
								Repository: "myorg/worker",
								Tag:        "abc1234",
								TagType:    domainimage.TagTypeCommit,
								Workloads: []domainimage.WorkloadRef{
									{Kind: kindDeploy, Namespace: nsDefault, Name: nameWorker, Container: nameWorker},
								},
							},
						},
					})
				})

				It("should return all images when no filters", func() {
					s := NewImageServer(store)
					req := newCallToolRequest("list_images", map[string]any{})

					result, err := s.handleListImages(ctx, req)

					Expect(err).NotTo(HaveOccurred())
					Expect(isErrorResult(result)).To(BeFalse())
					text := extractTextContent(result)
					Expect(text).To(ContainSubstring("Found 2 image(s)"))
					Expect(text).To(ContainSubstring("myorg/api"))
					Expect(text).To(ContainSubstring("myorg/worker"))
				})

				It("should filter by portal", func() {
					_ = store.ReplaceAll(ctx, nsDev, map[domainimage.WorkloadKey][]domainimage.ImageView{
						{Kind: kindDeploy, Namespace: nsDev, Name: nameSvc}: {
							{
								PortalRef:  nsDev,
								Registry:   "docker.io",
								Repository: "myorg/svc",
								Tag:        "latest",
								TagType:    domainimage.TagTypeLatest,
								Workloads:  []domainimage.WorkloadRef{{Kind: kindDeploy, Namespace: nsDev, Name: nameSvc, Container: nameSvc}},
							},
						},
					})
					s := NewImageServer(store)
					req := newCallToolRequest("list_images", map[string]any{"portal": nsDev})

					result, err := s.handleListImages(ctx, req)

					Expect(err).NotTo(HaveOccurred())
					text := extractTextContent(result)
					Expect(text).To(ContainSubstring("Found 1 image(s)"))
					Expect(text).To(ContainSubstring("myorg/svc"))
					Expect(text).NotTo(ContainSubstring("myorg/api"))
				})

				It("should filter by registry", func() {
					s := NewImageServer(store)
					req := newCallToolRequest("list_images", map[string]any{"registry": "ghcr.io"})

					result, err := s.handleListImages(ctx, req)

					Expect(err).NotTo(HaveOccurred())
					text := extractTextContent(result)
					Expect(text).To(ContainSubstring("Found 1 image(s)"))
					Expect(text).To(ContainSubstring("myorg/worker"))
					Expect(text).NotTo(ContainSubstring("myorg/api"))
				})

				It("should filter by tag_type", func() {
					s := NewImageServer(store)
					req := newCallToolRequest("list_images", map[string]any{"tag_type": "semver"})

					result, err := s.handleListImages(ctx, req)

					Expect(err).NotTo(HaveOccurred())
					text := extractTextContent(result)
					Expect(text).To(ContainSubstring("Found 1 image(s)"))
					Expect(text).To(ContainSubstring("myorg/api"))
				})

				It("should filter by search substring", func() {
					s := NewImageServer(store)
					req := newCallToolRequest("list_images", map[string]any{"search": nameWorker})

					result, err := s.handleListImages(ctx, req)

					Expect(err).NotTo(HaveOccurred())
					text := extractTextContent(result)
					Expect(text).To(ContainSubstring("Found 1 image(s)"))
					Expect(text).To(ContainSubstring("myorg/worker"))
				})
			})

			Context("with empty store", func() {
				It("should return no images message", func() {
					s := NewImageServer(store)
					req := newCallToolRequest("list_images", map[string]any{})

					result, err := s.handleListImages(ctx, req)

					Expect(err).NotTo(HaveOccurred())
					text := extractTextContent(result)
					Expect(text).To(Equal("No images found matching the criteria."))
				})
			})
		})
	})
})
