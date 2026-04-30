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

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ImageServer wraps the MCP server for image inventory.
// Mount at /mcp/image for Streamable HTTP.
type ImageServer struct {
	mcpServer *server.MCPServer
	reader    domainimage.ImageReader
}

// NewImageServer creates a new MCP server instance for image inventory.
func NewImageServer(reader domainimage.ImageReader) *ImageServer {
	s := &ImageServer{reader: reader}

	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-image")
		logger.Info("client session registered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("image").Inc()
	})
	hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
		logger := log.FromContext(ctx).WithName("mcp-image")
		logger.Info("client session unregistered", "sessionID", session.SessionID())
		metrics.MCPSessionsActive.WithLabelValues("image").Dec()
	})
	hooks.AddAfterInitialize(func(ctx context.Context, _ any, message *mcp.InitializeRequest, _ *mcp.InitializeResult) {
		logger := log.FromContext(ctx).WithName("mcp-image")
		logger.Info("client initialized",
			"clientName", message.Params.ClientInfo.Name,
			"clientVersion", message.Params.ClientInfo.Version,
			"protocolVersion", message.Params.ProtocolVersion,
		)
	})

	s.mcpServer = server.NewMCPServer(
		"sreportal-image",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithHooks(hooks),
	)

	s.registerImageTools()

	return s
}

func (s *ImageServer) registerImageTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("list_images",
			mcp.WithDescription("List container images discovered by ImageInventory resources in the SRE Portal. "+
				"Returns images with their tag type (semver, commit, digest, latest), registry, repository, and the workloads using them."),
			mcp.WithString("portal",
				mcp.Description("Filter by portal name (portalRef)"),
			),
			mcp.WithString("search",
				mcp.Description("Search in repository name (substring match)"),
			),
			mcp.WithString("registry",
				mcp.Description("Filter by registry hostname (e.g. docker.io, ghcr.io)"),
			),
			mcp.WithString("tag_type",
				mcp.Description("Filter by tag type: semver, commit, digest, or latest"),
			),
		),
		withToolMetrics("image", "list_images", s.handleListImages),
	)
}

// ImageWorkloadResult represents a workload that uses an image.
type ImageWorkloadResult struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Container string `json:"container"`
}

// ImageResult represents a single image entry in the list results.
type ImageResult struct {
	Registry   string                `json:"registry"`
	Repository string                `json:"repository"`
	Tag        string                `json:"tag"`
	TagType    string                `json:"tag_type"`
	Workloads  []ImageWorkloadResult `json:"workloads"`
}

func (s *ImageServer) handleListImages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	views, err := s.reader.List(ctx, domainimage.ImageFilters{
		Portal:   request.GetString("portal", ""),
		Search:   request.GetString("search", ""),
		Registry: request.GetString("registry", ""),
		TagType:  request.GetString("tag_type", ""),
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list images: %v", err)), nil
	}

	if len(views) == 0 {
		return mcp.NewToolResultText("No images found matching the criteria."), nil
	}

	results := make([]ImageResult, 0, len(views))
	for _, v := range views {
		workloads := make([]ImageWorkloadResult, 0, len(v.Workloads))
		for _, w := range v.Workloads {
			workloads = append(workloads, ImageWorkloadResult{
				Kind:      w.Kind,
				Namespace: w.Namespace,
				Name:      w.Name,
				Container: w.Container,
			})
		}
		results = append(results, ImageResult{
			Registry:   v.Registry,
			Repository: v.Repository,
			Tag:        v.Tag,
			TagType:    string(v.TagType),
			Workloads:  workloads,
		})
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d image(s):\n\n%s", len(results), string(jsonBytes))), nil
}

// Handler returns an http.Handler for the MCP Streamable HTTP transport.
// Mount at /mcp/image.
func (s *ImageServer) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}
