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

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// PortalResult represents a portal in the list results
type PortalResult struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Title     string `json:"title"`
	Main      bool   `json:"main"`
	SubPath   string `json:"subPath,omitempty"`
	RemoteURL string `json:"remoteUrl,omitempty"`
	Ready     bool   `json:"ready"`
}

// handleListPortals handles the list_portals tool call
func (s *Server) handleListPortals(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// List all Portal resources
	var portalList sreportalv1alpha1.PortalList
	if err := s.client.List(ctx, &portalList); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list Portal resources: %v", err)), nil
	}

	if len(portalList.Items) == 0 {
		return mcp.NewToolResultText("No portals found."), nil
	}

	// Convert to results
	results := make([]PortalResult, 0, len(portalList.Items))
	for _, portal := range portalList.Items {
		result := PortalResult{
			Name:      portal.Name,
			Namespace: portal.Namespace,
			Title:     portal.Spec.Title,
			Main:      portal.Spec.Main,
			SubPath:   portal.Spec.SubPath,
			Ready:     portal.Status.Ready,
		}
		if portal.Spec.Remote != nil {
			result.RemoteURL = portal.Spec.Remote.URL
		}
		results = append(results, result)
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d portal(s):\n\n%s", len(results), string(jsonBytes))), nil
}
