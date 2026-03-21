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

	"github.com/prometheus/client_golang/prometheus"

	domainmetrics "github.com/golgoth31/sreportal/internal/domain/metrics"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MetricsServer wraps the MCP server for Prometheus metrics.
// Mount at /mcp/metrics for Streamable HTTP.
type MetricsServer struct {
	mcpServer *server.MCPServer
	gatherer  prometheus.Gatherer
}

// NewMetricsServer creates a new MCP server instance for metrics.
func NewMetricsServer(g prometheus.Gatherer) *MetricsServer {
	s := &MetricsServer{gatherer: g}

	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-metrics")
		logger.Info("client session registered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("metrics").Inc()
	})
	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-metrics")
		logger.Info("client session unregistered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("metrics").Dec()
	})
	hooks.AddAfterInitialize(func(ctx context.Context, _ any, message *mcp.InitializeRequest, _ *mcp.InitializeResult) {
		logger := log.FromContext(ctx).WithName("mcp-metrics")
		logger.Info("client initialized",
			"clientName", message.Params.ClientInfo.Name,
			"clientVersion", message.Params.ClientInfo.Version,
			"protocolVersion", message.Params.ProtocolVersion,
		)
	})

	s.mcpServer = server.NewMCPServer(
		"sreportal-metrics",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithHooks(hooks),
	)

	s.registerTools()

	return s
}

// registerTools registers metrics MCP tools.
func (s *MetricsServer) registerTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("list_metrics",
			mcp.WithDescription("List current values of SRE Portal Prometheus metrics. "+
				"Returns sreportal_* custom metrics with their current values, labels, and types."),
			mcp.WithString("subsystem",
				mcp.Description("Filter by metric subsystem: controller, dns, source, alertmanager, portal, http, mcp"),
			),
			mcp.WithString("search",
				mcp.Description("Filter by metric name substring match"),
			),
		),
		withToolMetrics("metrics", "list_metrics", s.handleListMetrics),
	)
}

// handleListMetrics handles the list_metrics tool call.
func (s *MetricsServer) handleListMetrics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	subsystem := request.GetString("subsystem", "")
	search := request.GetString("search", "")

	families, err := domainmetrics.Gather(s.gatherer, subsystem, search)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to gather metrics: %v", err)), nil
	}

	if len(families) == 0 {
		return mcp.NewToolResultText("No metrics found matching the criteria."), nil
	}

	jsonBytes, err := json.MarshalIndent(families, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal metrics: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d metric family(ies):\n\n%s", len(families), string(jsonBytes))), nil
}

// Handler returns an http.Handler for the MCP Streamable HTTP transport.
// Mount at /mcp/metrics.
func (s *MetricsServer) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}
