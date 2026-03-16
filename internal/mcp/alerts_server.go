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

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AlertsServer wraps the MCP server for Alertmanager alerts.
// Mount at /mcp/alerts for Streamable HTTP.
type AlertsServer struct {
	mcpServer *server.MCPServer
	client    client.Client
}

// NewAlertsServer creates a new MCP server instance for alerts.
func NewAlertsServer(k8sClient client.Client) *AlertsServer {
	s := &AlertsServer{client: k8sClient}

	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-alerts")
		logger.Info("client session registered", "sessionID", session.SessionID())
	})
	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-alerts")
		logger.Info("client session unregistered", "sessionID", session.SessionID())
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
		s.handleListAlerts,
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

// handleListAlerts handles the list_alerts tool call.
func (s *AlertsServer) handleListAlerts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	portal := request.GetString("portal", "")
	namespace := request.GetString("namespace", "")
	search := request.GetString("search", "")
	stateFilter := request.GetString("state", "")

	var amList sreportalv1alpha1.AlertmanagerList
	listOpts := []client.ListOption{}

	if namespace != "" {
		listOpts = append(listOpts, client.InNamespace(namespace))
	}

	if err := s.client.List(ctx, &amList, listOpts...); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list Alertmanager resources: %v", err)), nil
	}

	results := make([]AlertmanagerResult, 0, len(amList.Items))
	searchLower := strings.ToLower(search)

	for _, am := range amList.Items {
		if portal != "" && am.Spec.PortalRef != portal {
			continue
		}

		alerts := make([]AlertResult, 0, len(am.Status.ActiveAlerts))
		for _, a := range am.Status.ActiveAlerts {
			if stateFilter != "" && a.State != stateFilter {
				continue
			}
			if search != "" && !matchesAlertSearch(a, searchLower) {
				continue
			}

			startsAt := ""
			if !a.StartsAt.IsZero() {
				startsAt = a.StartsAt.Format("2006-01-02T15:04:05Z07:00")
			}
			endsAt := ""
			if a.EndsAt != nil && !a.EndsAt.IsZero() {
				endsAt = a.EndsAt.Format("2006-01-02T15:04:05Z07:00")
			}
			updatedAt := ""
			if !a.UpdatedAt.IsZero() {
				updatedAt = a.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
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
		if am.Status.LastReconcileTime != nil && !am.Status.LastReconcileTime.IsZero() {
			lastReconcile = am.Status.LastReconcileTime.Format("2006-01-02T15:04:05Z07:00")
		}

		silences := make([]SilenceResult, 0, len(am.Status.Silences))
		for _, s := range am.Status.Silences {
			startsAt := s.StartsAt.Format("2006-01-02T15:04:05Z07:00")
			endsAt := s.EndsAt.Format("2006-01-02T15:04:05Z07:00")
			updatedAt := s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")

			matchers := make([]MatcherResult, 0, len(s.Matchers))
			for _, m := range s.Matchers {
				matchers = append(matchers, MatcherResult{
					Name:    m.Name,
					Value:   m.Value,
					IsRegex: m.IsRegex,
				})
			}

			silences = append(silences, SilenceResult{
				ID:        s.ID,
				Matchers:  matchers,
				StartsAt:  startsAt,
				EndsAt:    endsAt,
				Status:    s.Status,
				CreatedBy: s.CreatedBy,
				Comment:   s.Comment,
				UpdatedAt: updatedAt,
			})
		}

		results = append(results, AlertmanagerResult{
			Name:              am.Name,
			Namespace:         am.Namespace,
			PortalRef:         am.Spec.PortalRef,
			LocalURL:          am.Spec.URL.Local,
			RemoteURL:         am.Spec.URL.Remote,
			Ready:             isAlertmanagerReady(am),
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

func isAlertmanagerReady(am sreportalv1alpha1.Alertmanager) bool {
	for _, c := range am.Status.Conditions {
		if c.Type == "Ready" && c.Status == "True" {
			return true
		}
	}
	return false
}

func matchesAlertSearch(a sreportalv1alpha1.AlertStatus, searchLower string) bool {
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
