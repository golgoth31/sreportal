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
	"fmt"
	"net/http"
	"time"

	portalctrl "github.com/golgoth31/sreportal/internal/controller/portal"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ReleasesServer wraps the MCP server for release tracking.
// Mount at /mcp/releases for Streamable HTTP.
type ReleasesServer struct {
	mcpServer *server.MCPServer
	reader    domainrelease.ReleaseReader
}

// NewReleasesServer creates a new MCP server instance for releases.
func NewReleasesServer(reader domainrelease.ReleaseReader) *ReleasesServer {
	s := &ReleasesServer{reader: reader}

	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-releases")
		logger.Info("client session registered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("releases").Inc()
	})
	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-releases")
		logger.Info("client session unregistered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("releases").Dec()
	})
	hooks.AddAfterInitialize(func(ctx context.Context, _ any, message *mcp.InitializeRequest, _ *mcp.InitializeResult) {
		logger := log.FromContext(ctx).WithName("mcp-releases")
		logger.Info("client initialized",
			"clientName", message.Params.ClientInfo.Name,
			"clientVersion", message.Params.ClientInfo.Version,
			"protocolVersion", message.Params.ProtocolVersion,
		)
	})

	s.mcpServer = server.NewMCPServer(
		"sreportal-releases",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithHooks(hooks),
	)

	s.registerTools()

	return s
}

// ReleaseEntryResult represents a release entry in MCP responses.
type ReleaseEntryResult struct {
	Type    string `json:"type"`
	Version string `json:"version,omitempty"`
	Origin  string `json:"origin"`
	Date    string `json:"date"`
	Author  string `json:"author,omitempty"`
	Message string `json:"message,omitempty"`
	Link    string `json:"link,omitempty"`
}

// ListReleasesResult represents the list_releases response.
type ListReleasesResult struct {
	Day         string               `json:"day"`
	Entries     []ReleaseEntryResult `json:"entries"`
	PreviousDay string               `json:"previous_day,omitempty"`
	NextDay     string               `json:"next_day,omitempty"`
}

func (s *ReleasesServer) registerTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("list_releases",
			mcp.WithDescription("List release entries from the SRE Portal release tracker. "+
				"Returns entries for a specific day with navigation to adjacent days."),
			mcp.WithString("day",
				mcp.Description("Day to list releases for (YYYY-MM-DD format, defaults to latest)"),
			),
			mcp.WithString("portal",
				mcp.Description("Portal metadata.name (defaults to main)"),
			),
		),
		withToolMetrics("releases", "list_releases", s.handleListReleases),
	)
}

func (s *ReleasesServer) handleListReleases(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	day := request.GetString("day", "")
	portal := request.GetString("portal", "")
	if portal == "" {
		portal = portalctrl.MainPortalName
	}

	// If no day specified, use the latest
	if day == "" {
		days, err := s.reader.ListDays(ctx, portal)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list days: %v", err)), nil
		}
		if len(days) == 0 {
			return mcp.NewToolResultText("No releases found."), nil
		}
		day = days[len(days)-1]
	}

	entries, err := s.reader.ListEntries(ctx, day, portal)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list releases for day %s: %v", day, err)), nil
	}

	// Get day navigation
	days, err := s.reader.ListDays(ctx, portal)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list days: %v", err)), nil
	}

	var prevDay, nextDay string
	for i, d := range days {
		if d == day {
			if i > 0 {
				prevDay = days[i-1]
			}
			if i < len(days)-1 {
				nextDay = days[i+1]
			}
			break
		}
	}

	results := make([]ReleaseEntryResult, 0, len(entries))
	for _, e := range entries {
		results = append(results, ReleaseEntryResult{
			Type:    e.Type,
			Version: e.Version,
			Origin:  e.Origin,
			Date:    e.Date.Format(time.RFC3339),
			Author:  e.Author,
			Message: e.Message,
			Link:    e.Link,
		})
	}

	resp := ListReleasesResult{
		Day:         day,
		Entries:     results,
		PreviousDay: prevDay,
		NextDay:     nextDay,
	}

	jsonBytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Releases for %s (%d entries):\n\n%s", day, len(results), string(jsonBytes))), nil
}

// Handler returns an http.Handler for the MCP Streamable HTTP transport.
// Mount at /mcp/releases.
func (s *ReleasesServer) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}
