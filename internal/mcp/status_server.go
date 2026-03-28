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

	domaincomponent "github.com/golgoth31/sreportal/internal/domain/component"
	domainincident "github.com/golgoth31/sreportal/internal/domain/incident"
	domainmaint "github.com/golgoth31/sreportal/internal/domain/maintenance"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// StatusServer wraps the MCP server for status page data.
// Mount at /mcp/status for Streamable HTTP.
type StatusServer struct {
	mcpServer         *server.MCPServer
	componentReader   domaincomponent.ComponentReader
	maintenanceReader domainmaint.MaintenanceReader
	incidentReader    domainincident.IncidentReader
}

// NewStatusServer creates a new MCP server instance for status page.
func NewStatusServer(
	componentReader domaincomponent.ComponentReader,
	maintenanceReader domainmaint.MaintenanceReader,
	incidentReader domainincident.IncidentReader,
) *StatusServer {
	s := &StatusServer{
		componentReader:   componentReader,
		maintenanceReader: maintenanceReader,
		incidentReader:    incidentReader,
	}

	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-status")
		logger.Info("client session registered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("status").Inc()
	})
	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-status")
		logger.Info("client session unregistered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("status").Dec()
	})
	hooks.AddAfterInitialize(func(ctx context.Context, _ any, message *mcp.InitializeRequest, _ *mcp.InitializeResult) {
		logger := log.FromContext(ctx).WithName("mcp-status")
		logger.Info("client initialized",
			"clientName", message.Params.ClientInfo.Name,
			"clientVersion", message.Params.ClientInfo.Version,
			"protocolVersion", message.Params.ProtocolVersion,
		)
	})

	s.mcpServer = server.NewMCPServer(
		"sreportal-status",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithHooks(hooks),
	)

	s.registerTools()

	return s
}

func (s *StatusServer) registerTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("list_components",
			mcp.WithDescription("List platform components with their operational status from the SRE Portal status page."),
			mcp.WithString("portal", mcp.Description("Filter by portal name (portalRef)")),
			mcp.WithString("group", mcp.Description("Filter by component group name")),
		),
		withToolMetrics("status", "list_components", s.handleListComponents),
	)

	s.mcpServer.AddTool(
		mcp.NewTool("list_maintenances",
			mcp.WithDescription("List scheduled maintenance windows from the SRE Portal status page."),
			mcp.WithString("portal", mcp.Description("Filter by portal name (portalRef)")),
			mcp.WithString("phase", mcp.Description("Filter by phase: upcoming, in_progress, or completed")),
		),
		withToolMetrics("status", "list_maintenances", s.handleListMaintenances),
	)

	s.mcpServer.AddTool(
		mcp.NewTool("list_incidents",
			mcp.WithDescription("List declared incidents from the SRE Portal status page."),
			mcp.WithString("portal", mcp.Description("Filter by portal name (portalRef)")),
			mcp.WithString("phase", mcp.Description("Filter by phase: investigating, identified, monitoring, or resolved")),
		),
		withToolMetrics("status", "list_incidents", s.handleListIncidents),
	)

	s.mcpServer.AddTool(
		mcp.NewTool("get_platform_status",
			mcp.WithDescription("Get the overall platform status (aggregated from all components) for a portal."),
			mcp.WithString("portal", mcp.Description("Portal name to check (defaults to all portals)")),
		),
		withToolMetrics("status", "get_platform_status", s.handleGetPlatformStatus),
	)
}

// --- MCP Result types (stable JSON contract, decoupled from domain) ---

// ComponentResult is the MCP JSON representation of a platform component.
type ComponentResult struct {
	Name            string `json:"name"`
	DisplayName     string `json:"display_name"`
	Description     string `json:"description,omitempty"`
	Group           string `json:"group"`
	Link            string `json:"link,omitempty"`
	PortalRef       string `json:"portal_ref"`
	DeclaredStatus  string `json:"declared_status"`
	ComputedStatus  string `json:"computed_status"`
	ActiveIncidents int    `json:"active_incidents"`
}

// MaintenanceResult is the MCP JSON representation of a maintenance window.
type MaintenanceResult struct {
	Name           string   `json:"name"`
	Title          string   `json:"title"`
	Description    string   `json:"description,omitempty"`
	PortalRef      string   `json:"portal_ref"`
	Components     []string `json:"components,omitempty"`
	ScheduledStart string   `json:"scheduled_start"`
	ScheduledEnd   string   `json:"scheduled_end"`
	AffectedStatus string   `json:"affected_status"`
	Phase          string   `json:"phase"`
}

// IncidentUpdateResult is the MCP JSON representation of a single incident timeline entry.
type IncidentUpdateResult struct {
	Timestamp string `json:"timestamp"`
	Phase     string `json:"phase"`
	Message   string `json:"message"`
}

// IncidentResult is the MCP JSON representation of an incident.
type IncidentResult struct {
	Name            string                 `json:"name"`
	Title           string                 `json:"title"`
	PortalRef       string                 `json:"portal_ref"`
	Components      []string               `json:"components,omitempty"`
	Severity        string                 `json:"severity"`
	CurrentPhase    string                 `json:"current_phase"`
	Updates         []IncidentUpdateResult `json:"updates,omitempty"`
	StartedAt       string                 `json:"started_at,omitempty"`
	ResolvedAt      string                 `json:"resolved_at,omitempty"`
	DurationMinutes int                    `json:"duration_minutes,omitempty"`
}

func (s *StatusServer) handleListComponents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	opts := domaincomponent.ListOptions{
		PortalRef: request.GetString("portal", ""),
		Group:     request.GetString("group", ""),
	}

	views, err := s.componentReader.List(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list components: %v", err)), nil
	}

	if len(views) == 0 {
		return mcp.NewToolResultText("No components found."), nil
	}

	results := make([]ComponentResult, 0, len(views))
	for _, v := range views {
		results = append(results, ComponentResult{
			Name:            v.Name,
			DisplayName:     v.DisplayName,
			Description:     v.Description,
			Group:           v.Group,
			Link:            v.Link,
			PortalRef:       v.PortalRef,
			DeclaredStatus:  string(v.DeclaredStatus),
			ComputedStatus:  string(v.ComputedStatus),
			ActiveIncidents: v.ActiveIncidents,
		})
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d component(s):\n\n%s", len(results), string(jsonBytes))), nil
}

func (s *StatusServer) handleListMaintenances(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	opts := domainmaint.ListOptions{
		PortalRef: request.GetString("portal", ""),
		Phase:     domainmaint.MaintenancePhase(request.GetString("phase", "")),
	}

	views, err := s.maintenanceReader.List(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list maintenances: %v", err)), nil
	}

	if len(views) == 0 {
		return mcp.NewToolResultText("No maintenances found."), nil
	}

	results := make([]MaintenanceResult, 0, len(views))
	for _, v := range views {
		results = append(results, MaintenanceResult{
			Name:           v.Name,
			Title:          v.Title,
			Description:    v.Description,
			PortalRef:      v.PortalRef,
			Components:     v.Components,
			ScheduledStart: v.ScheduledStart.Format(timeFormat),
			ScheduledEnd:   v.ScheduledEnd.Format(timeFormat),
			AffectedStatus: v.AffectedStatus,
			Phase:          string(v.Phase),
		})
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d maintenance(s):\n\n%s", len(results), string(jsonBytes))), nil
}

func (s *StatusServer) handleListIncidents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	opts := domainincident.ListOptions{
		PortalRef: request.GetString("portal", ""),
		Phase:     domainincident.IncidentPhase(request.GetString("phase", "")),
	}

	views, err := s.incidentReader.List(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list incidents: %v", err)), nil
	}

	if len(views) == 0 {
		return mcp.NewToolResultText("No incidents found."), nil
	}

	results := make([]IncidentResult, 0, len(views))
	for _, v := range views {
		updates := make([]IncidentUpdateResult, 0, len(v.Updates))
		for _, u := range v.Updates {
			updates = append(updates, IncidentUpdateResult{
				Timestamp: u.Timestamp.Format(timeFormat),
				Phase:     string(u.Phase),
				Message:   u.Message,
			})
		}
		startedAt := ""
		if !v.StartedAt.IsZero() {
			startedAt = v.StartedAt.Format(timeFormat)
		}
		resolvedAt := ""
		if !v.ResolvedAt.IsZero() {
			resolvedAt = v.ResolvedAt.Format(timeFormat)
		}
		results = append(results, IncidentResult{
			Name:            v.Name,
			Title:           v.Title,
			PortalRef:       v.PortalRef,
			Components:      v.Components,
			Severity:        string(v.Severity),
			CurrentPhase:    string(v.CurrentPhase),
			Updates:         updates,
			StartedAt:       startedAt,
			ResolvedAt:      resolvedAt,
			DurationMinutes: v.DurationMinutes,
		})
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d incident(s):\n\n%s", len(results), string(jsonBytes))), nil
}

func (s *StatusServer) handleGetPlatformStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	portal := request.GetString("portal", "")

	views, err := s.componentReader.List(ctx, domaincomponent.ListOptions{PortalRef: portal})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list components: %v", err)), nil
	}

	if len(views) == 0 {
		return mcp.NewToolResultText("No components found. Platform status unknown."), nil
	}

	statusCounts := make(map[string]int)
	for _, v := range views {
		status := string(v.ComputedStatus)
		if status == "" {
			status = string(v.DeclaredStatus)
		}
		statusCounts[status]++
	}

	type statusSummary struct {
		TotalComponents int            `json:"total_components"`
		StatusCounts    map[string]int `json:"status_counts"`
		GlobalStatus    string         `json:"global_status"`
	}

	global := "operational"
	severityOrder := map[string]int{
		"major_outage":   6,
		"partial_outage": 5,
		"degraded":       4,
		"maintenance":    3,
		"unknown":        2,
		"operational":    1,
	}
	for status := range statusCounts {
		if severityOrder[status] > severityOrder[global] {
			global = status
		}
	}

	summary := statusSummary{
		TotalComponents: len(views),
		StatusCounts:    statusCounts,
		GlobalStatus:    global,
	}

	jsonBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// Handler returns an http.Handler for the MCP Streamable HTTP transport.
func (s *StatusServer) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}
