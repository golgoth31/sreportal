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

	domaindeploystatus "github.com/golgoth31/sreportal/internal/domain/deploystatus"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DeployStatusServer wraps the MCP server for deploy-status inventory.
// Mount at /mcp/deploystatus for Streamable HTTP.
type DeployStatusServer struct {
	mcpServer *server.MCPServer
	reader    domaindeploystatus.Reader
}

// NewDeployStatusServer creates a new MCP server instance for deploy-status inventory.
func NewDeployStatusServer(reader domaindeploystatus.Reader) *DeployStatusServer {
	s := &DeployStatusServer{reader: reader}

	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-deploystatus")
		logger.Info("client session registered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("deploystatus").Inc()
	})
	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-deploystatus")
		logger.Info("client session unregistered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("deploystatus").Dec()
	})
	hooks.AddAfterInitialize(func(ctx context.Context, _ any, message *mcp.InitializeRequest, _ *mcp.InitializeResult) {
		logger := log.FromContext(ctx).WithName("mcp-deploystatus")
		logger.Info("client initialized",
			"clientName", message.Params.ClientInfo.Name,
			"clientVersion", message.Params.ClientInfo.Version,
			"protocolVersion", message.Params.ProtocolVersion,
		)
	})

	s.mcpServer = server.NewMCPServer(
		"sreportal-deploystatus",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithHooks(hooks),
	)

	s.registerDeployStatusTools()

	return s
}

func (s *DeployStatusServer) registerDeployStatusTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("list_deploy_status",
			mcp.WithDescription("List deploy-status entries for workloads tracked by DeployStatus resources in the SRE Portal. "+
				"Returns per-workload deploy information including the deployed git ref, how many commits are ahead of the "+
				"deployed ref on the default branch, pending commits, and the overall state (ok|behind|unresolved|error)."),
			mcp.WithString("portal",
				mcp.Description("Portal name (portalRef) to query. Defaults to \"main\" when omitted."),
			),
			mcp.WithString("state",
				mcp.Description("Filter by deploy state: ok, behind, unresolved, or error. Omit to return all states."),
			),
		),
		withToolMetrics("deploystatus", "list_deploy_status", s.handleListDeployStatus),
	)
}

// DeployStatusWorkloadResult represents the workload owning a deploy-status entry.
type DeployStatusWorkloadResult struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Container string `json:"container"`
}

// DeployStatusCommitResult represents a single pending commit.
type DeployStatusCommitResult struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	URL     string `json:"url,omitempty"`
}

// DeployStatusResult is the JSON-serialisable form of a single deploy-status entry.
type DeployStatusResult struct {
	Key              string                     `json:"key"`
	Workload         DeployStatusWorkloadResult `json:"workload"`
	Image            string                     `json:"image"`
	SourceRepo       string                     `json:"source_repo"`
	DeployedRef      string                     `json:"deployed_ref"`
	DefaultBranch    string                     `json:"default_branch"`
	AheadBy          int                        `json:"ahead_by"`
	PendingCommits   []DeployStatusCommitResult `json:"pending_commits,omitempty"`
	PendingTruncated bool                       `json:"pending_truncated,omitempty"`
	DeployedAt       string                     `json:"deployed_at,omitempty"`
	DeployRunURL     string                     `json:"deploy_run_url,omitempty"`
	State            string                     `json:"state"`
	Error            string                     `json:"error,omitempty"`
	LastCheckedAt    string                     `json:"last_checked_at,omitempty"`
}

func (s *DeployStatusServer) handleListDeployStatus(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	portal := request.GetString("portal", "main")
	stateFilter := request.GetString("state", "")

	validStates := map[string]bool{"ok": true, "behind": true, "unresolved": true, "error": true}
	if stateFilter != "" && !validStates[stateFilter] {
		return mcp.NewToolResultError(
			fmt.Sprintf("invalid state %q: must be one of ok, behind, unresolved, error", stateFilter),
		), nil
	}

	entries := s.reader.List(portal)

	results := make([]DeployStatusResult, 0, len(entries))
	for _, e := range entries {
		if stateFilter != "" && e.State != stateFilter {
			continue
		}
		results = append(results, toDeployStatusResult(e))
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No deploy-status entries found matching the criteria."), nil
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d deploy-status entry(ies):\n\n%s", len(results), string(jsonBytes))), nil
}

// toDeployStatusResult converts a domain Entry into the JSON-friendly MCP result.
func toDeployStatusResult(e domaindeploystatus.Entry) DeployStatusResult {
	r := DeployStatusResult{
		Key: e.Key,
		Workload: DeployStatusWorkloadResult{
			Kind:      e.Workload.Kind,
			Namespace: e.Workload.Namespace,
			Name:      e.Workload.Name,
			Container: e.Workload.Container,
		},
		Image:            e.Image,
		SourceRepo:       e.SourceRepo,
		DeployedRef:      e.DeployedRef,
		DefaultBranch:    e.DefaultBranch,
		AheadBy:          e.AheadBy,
		PendingTruncated: e.PendingTruncated,
		DeployRunURL:     e.DeployRunURL,
		State:            e.State,
		Error:            e.Error,
	}

	if len(e.PendingCommits) > 0 {
		r.PendingCommits = make([]DeployStatusCommitResult, len(e.PendingCommits))
		for i, c := range e.PendingCommits {
			r.PendingCommits[i] = DeployStatusCommitResult{
				SHA:     c.Sha,
				Message: c.Message,
				Author:  c.Author,
				Date:    c.Date.UTC().Format("2006-01-02T15:04:05Z"),
				URL:     c.URL,
			}
		}
	}

	if !e.DeployedAt.IsZero() {
		r.DeployedAt = e.DeployedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if !e.LastCheckedAt.IsZero() {
		r.LastCheckedAt = e.LastCheckedAt.UTC().Format("2006-01-02T15:04:05Z")
	}

	return r
}

// Handler returns an http.Handler for the MCP Streamable HTTP transport.
// Mount at /mcp/deploystatus.
func (s *DeployStatusServer) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}
