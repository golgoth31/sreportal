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

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golgoth31/sreportal/internal/config"
)

// Server wraps the MCP server with SRE Portal functionality
type Server struct {
	mcpServer    *server.MCPServer
	httpServer   *server.StreamableHTTPServer
	client       client.Client
	groupMapping *config.GroupMappingConfig
}

// New creates a new MCP server instance
func New(k8sClient client.Client, groupMapping *config.GroupMappingConfig) *Server {
	s := &Server{
		client:       k8sClient,
		groupMapping: groupMapping,
	}

	// Create the MCP server
	s.mcpServer = server.NewMCPServer(
		"sreportal",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register tools
	s.registerTools()

	return s
}

// registerTools registers all MCP tools
func (s *Server) registerTools() {
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
		s.handleSearchFQDNs,
	)

	// Register list_portals tool
	s.mcpServer.AddTool(
		mcp.NewTool("list_portals",
			mcp.WithDescription("List all available portals in the SRE Portal. "+
				"Portals are entry points that group DNS entries together."),
		),
		s.handleListPortals,
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
		s.handleGetFQDNDetails,
	)
}

// ServeStdio starts the MCP server using stdio transport
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}

// ServeStreamableHTTP starts the MCP server using Streamable HTTP transport (MCP spec 2025-03-26).
// It exposes a single /mcp endpoint that accepts POST, GET, and DELETE methods.
func (s *Server) ServeStreamableHTTP(address string) error {
	s.httpServer = server.NewStreamableHTTPServer(s.mcpServer,
		server.WithEndpointPath("/mcp"),
	)
	return s.httpServer.Start(address)
}

// Shutdown gracefully stops the MCP HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
