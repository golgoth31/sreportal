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
	"strings"

	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanagerreadmodel"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// AlertsServer wraps the MCP server for Alertmanager alerts.
// Mount at /mcp/alerts for Streamable HTTP.
type AlertsServer struct {
	mcpServer *server.MCPServer
	reader    domainalertmanager.AlertmanagerReader
}

// NewAlertsServer creates a new MCP server instance for alerts.
func NewAlertsServer(reader domainalertmanager.AlertmanagerReader) *AlertsServer {
	s := &AlertsServer{reader: reader}

	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-alerts")
		logger.Info("client session registered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("alerts").Inc()
	})
	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-alerts")
		logger.Info("client session unregistered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("alerts").Dec()
	})
	hooks.AddAfterInitialize(func(ctx context.Context, _ any, message *mcp.InitializeRequest, _ *mcp.InitializeResult) {
		logger := log.FromContext(ctx).WithName("mcp-alerts")
		logger.Info("client initialized",
			"clientName", message.Params.ClientInfo.Name,
			"clientVersion", message.Params.ClientInfo.Version,
			"protocolVersion", message.Params.ProtocolVersion,
		)
	})

	s.mcpServer = server.NewMCPServer(
		"sreportal-alerts",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithHooks(hooks),
	)

	s.registerAlertTools()

	return s
}

// registerAlertTools registers alert-related MCP tools.
func (s *AlertsServer) registerAlertTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("list_alerts",
			mcp.WithDescription("List active alerts from Alertmanager resources in the SRE Portal. "+
				"Returns Alertmanager resources with their active alerts and labels."),
			mcp.WithString("portal",
				mcp.Description("Filter by portal name (portalRef)"),
			),
			mcp.WithString("namespace",
				mcp.Description("Filter by Kubernetes namespace"),
			),
			mcp.WithString("search",
				mcp.Description("Search in label or annotation values (substring match)"),
			),
			mcp.WithString("state",
				mcp.Description("Filter by alert state: active, suppressed, or unprocessed"),
			),
		),
		withToolMetrics("alerts", "list_alerts", s.handleListAlerts),
	)
}

// AlertResult represents a single alert in the list results.
type AlertResult struct {
	Fingerprint string            `json:"fingerprint"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations,omitempty"`
	State       string            `json:"state"`
	StartsAt    string            `json:"starts_at"`
	EndsAt      string            `json:"ends_at,omitempty"`
	UpdatedAt   string            `json:"updated_at"`
	AlertName   string            `json:"alertname,omitempty"`
	Receivers   []string          `json:"receivers,omitempty"`
	SilencedBy  []string          `json:"silenced_by,omitempty"`
}

// MatcherResult represents a label matcher within a silence.
type MatcherResult struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"is_regex"`
}

// SilenceResult represents a silence from Alertmanager (for identifying silenced alerts).
type SilenceResult struct {
	ID        string          `json:"id"`
	Matchers  []MatcherResult `json:"matchers"`
	StartsAt  string          `json:"starts_at"`
	EndsAt    string          `json:"ends_at"`
	Status    string          `json:"status"`
	CreatedBy string          `json:"created_by"`
	Comment   string          `json:"comment"`
	UpdatedAt string          `json:"updated_at"`
}

// AlertmanagerResult represents an Alertmanager resource with its alerts.
type AlertmanagerResult struct {
	Name              string          `json:"name"`
	Namespace         string          `json:"namespace"`
	PortalRef         string          `json:"portal_ref"`
	LocalURL          string          `json:"local_url"`
	RemoteURL         string          `json:"remote_url,omitempty"`
	Ready             bool            `json:"ready"`
	LastReconcileTime string          `json:"last_reconcile_time,omitempty"`
	Alerts            []AlertResult   `json:"alerts"`
	Silences          []SilenceResult `json:"silences,omitempty"`
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

// handleListAlerts handles the list_alerts tool call.
func (s *AlertsServer) handleListAlerts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	portal := request.GetString("portal", "")
	namespace := request.GetString("namespace", "")
	search := request.GetString("search", "")
	stateFilter := request.GetString("state", "")

	filters := domainalertmanager.AlertmanagerFilters{
		Portal:    portal,
		Namespace: namespace,
	}

	views, err := s.reader.List(ctx, filters)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list Alertmanager resources: %v", err)), nil
	}

	searchLower := strings.ToLower(search)
	results := make([]AlertmanagerResult, 0, len(views))

	for _, v := range views {
		alerts := make([]AlertResult, 0, len(v.Alerts))
		for _, a := range v.Alerts {
			if stateFilter != "" && a.State != stateFilter {
				continue
			}
			if search != "" && !matchesAlertSearch(a, searchLower) {
				continue
			}

			startsAt := ""
			if !a.StartsAt.IsZero() {
				startsAt = a.StartsAt.Format(timeFormat)
			}
			endsAt := ""
			if a.EndsAt != nil && !a.EndsAt.IsZero() {
				endsAt = a.EndsAt.Format(timeFormat)
			}
			updatedAt := ""
			if !a.UpdatedAt.IsZero() {
				updatedAt = a.UpdatedAt.Format(timeFormat)
			}

			alerts = append(alerts, AlertResult{
				Fingerprint: a.Fingerprint,
				Labels:      a.Labels,
				Annotations: a.Annotations,
				State:       a.State,
				StartsAt:    startsAt,
				EndsAt:      endsAt,
				UpdatedAt:   updatedAt,
				AlertName:   a.Labels["alertname"],
				Receivers:   a.Receivers,
				SilencedBy:  a.SilencedBy,
			})
		}

		lastReconcile := ""
		if v.LastReconcileTime != nil && !v.LastReconcileTime.IsZero() {
			lastReconcile = v.LastReconcileTime.Format(timeFormat)
		}

		silences := make([]SilenceResult, 0, len(v.Silences))
		for _, sil := range v.Silences {
			matchers := make([]MatcherResult, 0, len(sil.Matchers))
			for _, m := range sil.Matchers {
				matchers = append(matchers, MatcherResult{
					Name:    m.Name,
					Value:   m.Value,
					IsRegex: m.IsRegex,
				})
			}

			silences = append(silences, SilenceResult{
				ID:        sil.ID,
				Matchers:  matchers,
				StartsAt:  sil.StartsAt.Format(timeFormat),
				EndsAt:    sil.EndsAt.Format(timeFormat),
				Status:    sil.Status,
				CreatedBy: sil.CreatedBy,
				Comment:   sil.Comment,
				UpdatedAt: sil.UpdatedAt.Format(timeFormat),
			})
		}

		results = append(results, AlertmanagerResult{
			Name:              v.Name,
			Namespace:         v.Namespace,
			PortalRef:         v.PortalRef,
			LocalURL:          v.LocalURL,
			RemoteURL:         v.RemoteURL,
			Ready:             v.Ready,
			LastReconcileTime: lastReconcile,
			Alerts:            alerts,
			Silences:          silences,
		})
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No Alertmanager resources or alerts found matching the criteria."), nil
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d Alertmanager resource(s):\n\n%s", len(results), string(jsonBytes))), nil
}

func matchesAlertSearch(a domainalertmanager.AlertView, searchLower string) bool {
	for _, v := range a.Labels {
		if strings.Contains(strings.ToLower(v), searchLower) {
			return true
		}
	}
	for _, v := range a.Annotations {
		if strings.Contains(strings.ToLower(v), searchLower) {
			return true
		}
	}
	return false
}

// Handler returns an http.Handler for the MCP Streamable HTTP transport.
// Mount at /mcp/alerts.
func (s *AlertsServer) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}
