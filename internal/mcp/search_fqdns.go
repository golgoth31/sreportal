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
	"slices"

	"github.com/mark3labs/mcp-go/mcp"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

// FQDNResult represents a single FQDN in the search results
type FQDNResult struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	Group       string   `json:"group"`
	Description string   `json:"description,omitempty"`
	RecordType  string   `json:"record_type"`
	Targets     []string `json:"targets"`
	SyncStatus  string   `json:"sync_status,omitempty"`
	Portal      string   `json:"portal,omitempty"`
	Namespace   string   `json:"namespace,omitempty"`
}

// handleSearchFQDNs handles the search_fqdns tool call
func (s *DNSServer) handleSearchFQDNs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := request.GetString("query", "")
	source := request.GetString("source", "")
	portal := request.GetString("portal", "")
	namespace := request.GetString("namespace", "")

	filters := domaindns.FQDNFilters{
		Search:    query,
		Source:    source,
		Portal:    portal,
		Namespace: namespace,
	}

	views, err := s.fqdnReader.List(ctx, filters)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list FQDNs: %v", err)), nil
	}

	// Apply group filter (not part of FQDNFilters since it's MCP-specific post-filter)
	group := request.GetString("group", "")

	var results []FQDNResult
	seen := make(map[string]bool)

	for _, v := range views {
		if seen[v.Name] {
			continue
		}

		if group != "" && !slices.Contains(v.Groups, group) {
			continue
		}

		seen[v.Name] = true

		groupName := ""
		if len(v.Groups) > 0 {
			groupName = v.Groups[0]
		}

		results = append(results, FQDNResult{
			Name:        v.Name,
			Source:      string(v.Source),
			Group:       groupName,
			Description: v.Description,
			RecordType:  v.RecordType,
			Targets:     v.Targets,
			SyncStatus:  v.SyncStatus,
			Portal:      v.PortalName,
			Namespace:   v.Namespace,
		})
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No FQDNs found matching the search criteria."), nil
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d FQDN(s):\n\n%s", len(results), string(jsonBytes))), nil
}
