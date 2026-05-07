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
	"sort"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
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
				"Returns images with their tag type (semver, commit, digest, latest, other), registry, repository, "+
				"latest available version (if registry lookup ran), change type (none/mutated/injected), and the workloads using them. "+
				"Each workload entry has a `source` field: \"spec\" when the container is declared in the workload template, "+
				"or \"pod\" when it was only observed in the running pod (e.g. injected/mutated by a MutatingWebhook)."),
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
				mcp.Description("Filter by tag type: semver, commit, digest, latest, or other"),
			),
		),
		withToolMetrics("image", "list_images", s.handleListImages),
	)

	s.mcpServer.AddTool(
		mcp.NewTool("list_upgrades",
			mcp.WithDescription("List images for which a newer semver version is available on the origin registry. "+
				"Each result has upgrade_available=true and a non-empty latest_version that is strictly greater than the current tag."),
			mcp.WithString("portal",
				mcp.Description("Filter by portal name (portalRef)"),
			),
			mcp.WithString("host",
				mcp.Description("Filter by registry hostname (e.g. docker.io, ghcr.io)"),
			),
		),
		withToolMetrics("image", "list_upgrades", s.handleListUpgrades),
	)

	s.mcpServer.AddTool(
		mcp.NewTool("list_mutations",
			mcp.WithDescription("List images whose runtime form differs from the workload template — either rewritten by a "+
				"MutatingWebhook (change_type=mutated) or injected as a sidecar (change_type=injected)."),
			mcp.WithString("portal",
				mcp.Description("Filter by portal name (portalRef)"),
			),
			mcp.WithString("host",
				mcp.Description("Filter by registry hostname"),
			),
			mcp.WithString("change_type",
				mcp.Description("Filter by change_type: mutated or injected (default: both)"),
			),
		),
		withToolMetrics("image", "list_mutations", s.handleListMutations),
	)

	s.mcpServer.AddTool(
		mcp.NewTool("summary",
			mcp.WithDescription("Aggregated counts (images, upgrades, mutated, injected) per registry host for a portal — "+
				"the JSON equivalent of the group-by-host UI view."),
			mcp.WithString("portal",
				mcp.Description("Portal name (portalRef) to summarize"),
			),
		),
		withToolMetrics("image", "summary", s.handleSummary),
	)
}

// ImageWorkloadResult represents a workload that uses an image.
type ImageWorkloadResult struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Container string `json:"container"`
	// Source is "spec" when the container is declared in the workload template,
	// or "pod" when it was only observed in the running pod (typically because
	// a MutatingWebhook injected or mutated it).
	Source string `json:"source"`
}

// ImageResult represents a single image entry in the list results.
type ImageResult struct {
	Registry         string                `json:"registry"`
	Repository       string                `json:"repository"`
	Tag              string                `json:"tag"`
	TagType          string                `json:"tag_type"`
	OriginalImage    string                `json:"original_image,omitempty"`
	MutatedImage     string                `json:"mutated_image,omitempty"`
	ChangeType       string                `json:"change_type,omitempty"`
	LatestVersion    string                `json:"latest_version,omitempty"`
	LatestCheckedAt  string                `json:"latest_checked_at,omitempty"`
	LatestError      string                `json:"latest_error,omitempty"`
	UpgradeAvailable bool                  `json:"upgrade_available,omitempty"`
	Workloads        []ImageWorkloadResult `json:"workloads"`
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
				Source:    string(w.Source),
			})
		}
		results = append(results, toImageResult(v, workloads))
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d image(s):\n\n%s", len(results), string(jsonBytes))), nil
}

// toImageResult lifts an ImageView + workloads slice into the JSON-friendly
// MCP result, including the registry-version-lookup fields.
func toImageResult(v domainimage.ImageView, workloads []ImageWorkloadResult) ImageResult {
	r := ImageResult{
		Registry:         v.Registry,
		Repository:       v.Repository,
		Tag:              v.Tag,
		TagType:          string(v.TagType),
		OriginalImage:    v.OriginalImage,
		MutatedImage:     v.MutatedImage,
		ChangeType:       v.ChangeType,
		LatestVersion:    v.LatestVersion,
		LatestError:      v.LatestError,
		UpgradeAvailable: v.UpgradeAvailable,
		Workloads:        workloads,
	}
	if v.LatestCheckedAt != nil {
		r.LatestCheckedAt = v.LatestCheckedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return r
}

func (s *ImageServer) handleListUpgrades(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	views, err := s.reader.List(ctx, domainimage.ImageFilters{
		Portal:   request.GetString("portal", ""),
		Registry: request.GetString("host", ""),
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list images: %v", err)), nil
	}

	results := make([]ImageResult, 0)
	for _, v := range views {
		if !v.UpgradeAvailable {
			continue
		}
		workloads := mcpWorkloads(v.Workloads)
		results = append(results, toImageResult(v, workloads))
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No upgrades available."), nil
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Found %d upgrade(s):\n\n%s", len(results), string(jsonBytes))), nil
}

func (s *ImageServer) handleListMutations(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	wantedType := request.GetString("change_type", "")
	mutated := string(domainimageregistry.ChangeTypeMutated)
	injected := string(domainimageregistry.ChangeTypeInjected)
	if wantedType != "" && wantedType != mutated && wantedType != injected {
		return mcp.NewToolResultError(
			fmt.Sprintf("invalid change_type %q: must be %q or %q", wantedType, mutated, injected),
		), nil
	}

	views, err := s.reader.List(ctx, domainimage.ImageFilters{
		Portal:   request.GetString("portal", ""),
		Registry: request.GetString("host", ""),
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list images: %v", err)), nil
	}

	results := make([]ImageResult, 0)
	for _, v := range views {
		if v.ChangeType != mutated && v.ChangeType != injected {
			continue
		}
		if wantedType != "" && v.ChangeType != wantedType {
			continue
		}
		workloads := mcpWorkloads(v.Workloads)
		results = append(results, toImageResult(v, workloads))
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No mutations found."), nil
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Found %d mutation(s):\n\n%s", len(results), string(jsonBytes))), nil
}

// HostSummary aggregates per-host counts for the summary tool.
type HostSummary struct {
	Host     string `json:"host"`
	Images   int    `json:"images"`
	Upgrades int    `json:"upgrades"`
	Mutated  int    `json:"mutated"`
	Injected int    `json:"injected"`
}

func (s *ImageServer) handleSummary(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	views, err := s.reader.List(ctx, domainimage.ImageFilters{
		Portal: request.GetString("portal", ""),
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list images: %v", err)), nil
	}

	byHost := make(map[string]*HostSummary)
	for _, v := range views {
		h := byHost[v.Registry]
		if h == nil {
			h = &HostSummary{Host: v.Registry}
			byHost[v.Registry] = h
		}
		h.Images++
		if v.UpgradeAvailable {
			h.Upgrades++
		}
		switch v.ChangeType {
		case string(domainimageregistry.ChangeTypeMutated):
			h.Mutated++
		case string(domainimageregistry.ChangeTypeInjected):
			h.Injected++
		}
	}

	out := make([]HostSummary, 0, len(byHost))
	for _, h := range byHost {
		out = append(out, *h)
	}
	// Deterministic order for diff-friendly tool output.
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })

	jsonBytes, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Summary across %d host(s):\n\n%s", len(out), string(jsonBytes))), nil
}

// mcpWorkloads converts a domain WorkloadRef slice into the MCP wire format.
func mcpWorkloads(refs []domainimage.WorkloadRef) []ImageWorkloadResult {
	out := make([]ImageWorkloadResult, 0, len(refs))
	for _, w := range refs {
		out = append(out, ImageWorkloadResult{
			Kind:      w.Kind,
			Namespace: w.Namespace,
			Name:      w.Name,
			Container: w.Container,
			Source:    string(w.Source),
		})
	}
	return out
}

// Handler returns an http.Handler for the MCP Streamable HTTP transport.
// Mount at /mcp/image.
func (s *ImageServer) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}
