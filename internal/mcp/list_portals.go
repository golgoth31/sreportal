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

	"github.com/mark3labs/mcp-go/mcp"

	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
)

// RemoteSyncResult mirrors Portal status.remoteSync for MCP JSON (aligned with Connect ListPortals).
type RemoteSyncResult struct {
	LastSyncTime  string `json:"lastSyncTime,omitempty"`
	LastSyncError string `json:"lastSyncError,omitempty"`
	RemoteTitle   string `json:"remoteTitle,omitempty"`
	FQDNCount     int    `json:"fqdnCount,omitempty"`
}

// PortalResult represents a portal in the list results
type PortalResult struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Title      string            `json:"title"`
	Main       bool              `json:"main"`
	SubPath    string            `json:"subPath,omitempty"`
	RemoteURL  string            `json:"remoteUrl,omitempty"`
	Ready      bool              `json:"ready"`
	RemoteSync *RemoteSyncResult `json:"remoteSync,omitempty"`
}

// handleListPortals handles the list_portals tool call
func (s *DNSServer) handleListPortals(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	views, err := s.portalReader.List(ctx, domainportal.PortalFilters{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list portals: %v", err)), nil
	}

	if len(views) == 0 {
		return mcp.NewToolResultText("No portals found."), nil
	}

	results := make([]PortalResult, 0, len(views))
	for _, v := range views {
		result := PortalResult{
			Name:      v.Name,
			Namespace: v.Namespace,
			Title:     v.Title,
			Main:      v.Main,
			SubPath:   v.SubPath,
			Ready:     v.Ready,
			RemoteURL: v.URL,
		}
		if v.RemoteSync != nil {
			result.RemoteSync = &RemoteSyncResult{
				LastSyncTime:  v.RemoteSync.LastSyncTime,
				LastSyncError: v.RemoteSync.LastSyncError,
				RemoteTitle:   v.RemoteSync.RemoteTitle,
				FQDNCount:     v.RemoteSync.FQDNCount,
			}
		}
		results = append(results, result)
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d portal(s):\n\n%s", len(results), string(jsonBytes))), nil
}
