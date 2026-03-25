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
	"net/http"
	"time"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DNSServer wraps the MCP server with SRE Portal DNS/portal functionality.
// Mount at /mcp/dns for Streamable HTTP.
type DNSServer struct {
	mcpServer    *server.MCPServer
	fqdnReader   domaindns.FQDNReader
	portalReader domainportal.PortalReader
}

// NewDNSServer creates a new MCP server instance for DNS and portals.
func NewDNSServer(fqdnReader domaindns.FQDNReader, portalReader domainportal.PortalReader) *DNSServer {
	s := &DNSServer{
		fqdnReader:   fqdnReader,
		portalReader: portalReader,
	}

	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-dns")
		logger.Info("client session registered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("dns").Inc()
	})
	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-dns")
		logger.Info("client session unregistered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("dns").Dec()
	})
	hooks.AddAfterInitialize(func(ctx context.Context, _ any, message *mcp.InitializeRequest, _ *mcp.InitializeResult) {
		logger := log.FromContext(ctx).WithName("mcp-dns")
		logger.Info("client initialized",
			"clientName", message.Params.ClientInfo.Name,
			"clientVersion", message.Params.ClientInfo.Version,
			"protocolVersion", message.Params.ProtocolVersion,
		)
	})

	s.mcpServer = server.NewMCPServer(
		"sreportal-dns",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithHooks(hooks),
	)

	s.registerDNSTools()

	return s
}

// registerDNSTools registers DNS and portal MCP tools.
func (s *DNSServer) registerDNSTools() {
	// Register search_fqdns tool
	s.mcpServer.AddTool(
		mcp.NewTool("search_fqdns",
			mcp.WithDescription("Search for FQDNs (Fully Qualified Domain Names) in the SRE Portal. "+
				"Returns a list of DNS entries matching the search criteria."),
			mcp.WithString("query",
				mcp.Description("Search query to filter FQDNs by name (substring match)"),
			),
			mcp.WithString("source",
				mcp.Description("Filter by source: 'manual' or 'external-dns'"),
			),
			mcp.WithString("group",
				mcp.Description("Filter by group name"),
			),
			mcp.WithString("portal",
				mcp.Description("Filter by portal name"),
			),
			mcp.WithString("namespace",
				mcp.Description("Filter by Kubernetes namespace"),
			),
		),
		withToolMetrics("dns", "search_fqdns", s.handleSearchFQDNs),
	)

	// Register list_portals tool
	s.mcpServer.AddTool(
		mcp.NewTool("list_portals",
			mcp.WithDescription("List all available portals in the SRE Portal. "+
				"Portals are entry points that group DNS entries together. "+
				"For remote portals, includes remoteSync (lastSyncTime, lastSyncError, remoteTitle, fqdnCount) when status is available."),
		),
		withToolMetrics("dns", "list_portals", s.handleListPortals),
	)

	// Register get_fqdn_details tool
	s.mcpServer.AddTool(
		mcp.NewTool("get_fqdn_details",
			mcp.WithDescription("Get detailed information about a specific FQDN. "+
				"Returns the full DNS record details including targets, record type, and metadata."),
			mcp.WithString("fqdn",
				mcp.Required(),
				mcp.Description("The exact FQDN to look up (e.g., 'api.example.com')"),
			),
		),
		withToolMetrics("dns", "get_fqdn_details", s.handleGetFQDNDetails),
	)
}

// withToolMetrics wraps an MCP tool handler with Prometheus instrumentation.
func withToolMetrics(serverName, toolName string, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		metrics.MCPToolCallsTotal.WithLabelValues(serverName, toolName).Inc()

		result, err := handler(ctx, request)

		metrics.MCPToolCallDuration.WithLabelValues(serverName, toolName).Observe(time.Since(start).Seconds())
		if err != nil || (result != nil && result.IsError) {
			metrics.MCPToolCallErrorsTotal.WithLabelValues(serverName, toolName).Inc()
		}

		return result, err
	}
}

// ServeStdio starts the MCP server using stdio transport.
func (s *DNSServer) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}

// Handler returns an http.Handler for the MCP Streamable HTTP transport.
// Mount at /mcp/dns.
func (s *DNSServer) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}
