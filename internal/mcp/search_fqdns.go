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
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// FQDNResult represents a single FQDN in the search results
type FQDNResult struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	Group       string   `json:"group"`
	Description string   `json:"description,omitempty"`
	RecordType  string   `json:"record_type"`
	Targets     []string `json:"targets"`
	Portal      string   `json:"portal,omitempty"`
	Namespace   string   `json:"namespace,omitempty"`
}

// handleSearchFQDNs handles the search_fqdns tool call
func (s *Server) handleSearchFQDNs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters using GetString helper methods
	query := request.GetString("query", "")
	source := request.GetString("source", "")
	group := request.GetString("group", "")
	portal := request.GetString("portal", "")
	namespace := request.GetString("namespace", "")

	// List all DNS resources
	var dnsList sreportalv1alpha1.DNSList
	listOpts := []client.ListOption{}

	if namespace != "" {
		listOpts = append(listOpts, client.InNamespace(namespace))
	}

	if portal != "" {
		listOpts = append(listOpts, client.MatchingFields{"spec.portalRef": portal})
	}

	if err := s.client.List(ctx, &dnsList, listOpts...); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list DNS resources: %v", err)), nil
	}

	// Collect FQDNs from DNS resources
	var results []FQDNResult
	seen := make(map[string]bool)

	for _, dns := range dnsList.Items {
		for _, grp := range dns.Status.Groups {
			// Apply source filter
			if source != "" && grp.Source != source {
				continue
			}

			// Apply group filter
			if group != "" && !strings.EqualFold(grp.Name, group) {
				continue
			}

			for _, fqdnStatus := range grp.FQDNs {
				// Apply search query filter
				if query != "" && !strings.Contains(
					strings.ToLower(fqdnStatus.FQDN),
					strings.ToLower(query),
				) {
					continue
				}

				if seen[fqdnStatus.FQDN] {
					continue
				}
				seen[fqdnStatus.FQDN] = true

				results = append(results, FQDNResult{
					Name:        fqdnStatus.FQDN,
					Source:      grp.Source,
					Group:       grp.Name,
					Description: fqdnStatus.Description,
					RecordType:  fqdnStatus.RecordType,
					Targets:     fqdnStatus.Targets,
					Portal:      dns.Spec.PortalRef,
					Namespace:   dns.Namespace,
				})
			}
		}
	}

	// Format response
	if len(results) == 0 {
		return mcp.NewToolResultText("No FQDNs found matching the search criteria."), nil
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d FQDN(s):\n\n%s", len(results), string(jsonBytes))), nil
}
