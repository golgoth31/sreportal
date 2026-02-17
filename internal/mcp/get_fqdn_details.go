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

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
)

// FQDNDetails represents detailed information about a specific FQDN
type FQDNDetails struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	Group       string   `json:"group"`
	Description string   `json:"description,omitempty"`
	RecordType  string   `json:"record_type"`
	Targets     []string `json:"targets"`
	Portal      string   `json:"portal,omitempty"`
	Namespace   string   `json:"namespace,omitempty"`
	LastSeen    string   `json:"last_seen,omitempty"`
	DNSResource string   `json:"dns_resource,omitempty"`
}

// handleGetFQDNDetails handles the get_fqdn_details tool call
func (s *Server) handleGetFQDNDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract required parameter
	fqdn, err := request.RequireString("fqdn")
	if err != nil {
		return mcp.NewToolResultError("fqdn parameter is required"), nil
	}

	// Normalize the FQDN for comparison
	fqdnLower := strings.ToLower(strings.TrimSuffix(fqdn, "."))

	// Search in all DNS resources
	var dnsList sreportalv1alpha1.DNSList
	if err := s.client.List(ctx, &dnsList); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list DNS resources: %v", err)), nil
	}

	// Look for the FQDN in DNS status
	for _, dns := range dnsList.Items {
		for _, grp := range dns.Status.Groups {
			for _, fqdnStatus := range grp.FQDNs {
				fqdnStatusLower := strings.ToLower(strings.TrimSuffix(fqdnStatus.FQDN, "."))
				if fqdnStatusLower == fqdnLower {
					details := FQDNDetails{
						Name:        fqdnStatus.FQDN,
						Source:      grp.Source,
						Group:       grp.Name,
						Description: fqdnStatus.Description,
						RecordType:  fqdnStatus.RecordType,
						Targets:     fqdnStatus.Targets,
						Portal:      dns.Spec.PortalRef,
						Namespace:   dns.Namespace,
						DNSResource: fmt.Sprintf("%s/%s", dns.Namespace, dns.Name),
					}
					if !fqdnStatus.LastSeen.IsZero() {
						details.LastSeen = fqdnStatus.LastSeen.Format("2006-01-02T15:04:05Z07:00")
					}

					jsonBytes, err := json.MarshalIndent(details, "", "  ")
					if err != nil {
						return mcp.NewToolResultError(fmt.Sprintf("failed to marshal details: %v", err)), nil
					}

					return mcp.NewToolResultText(fmt.Sprintf("FQDN details for '%s':\n\n%s", fqdn, string(jsonBytes))), nil
				}
			}
		}
	}

	// Also check DNSRecords directly
	var dnsRecordList sreportalv1alpha1.DNSRecordList
	if err := s.client.List(ctx, &dnsRecordList); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list DNSRecord resources: %v", err)), nil
	}

	var allEndpoints []sreportalv1alpha1.EndpointStatus
	for _, rec := range dnsRecordList.Items {
		allEndpoints = append(allEndpoints, rec.Status.Endpoints...)
	}

	if len(allEndpoints) > 0 {
		groups := adapter.EndpointStatusToGroups(allEndpoints, s.groupMapping)
		for _, grp := range groups {
			for _, fqdnStatus := range grp.FQDNs {
				fqdnStatusLower := strings.ToLower(strings.TrimSuffix(fqdnStatus.FQDN, "."))
				if fqdnStatusLower == fqdnLower {
					details := FQDNDetails{
						Name:       fqdnStatus.FQDN,
						Source:     grp.Source,
						Group:      grp.Name,
						RecordType: fqdnStatus.RecordType,
						Targets:    fqdnStatus.Targets,
					}
					if !fqdnStatus.LastSeen.IsZero() {
						details.LastSeen = fqdnStatus.LastSeen.Format("2006-01-02T15:04:05Z07:00")
					}

					jsonBytes, err := json.MarshalIndent(details, "", "  ")
					if err != nil {
						return mcp.NewToolResultError(fmt.Sprintf("failed to marshal details: %v", err)), nil
					}

					return mcp.NewToolResultText(fmt.Sprintf("FQDN details for '%s':\n\n%s", fqdn, string(jsonBytes))), nil
				}
			}
		}
	}

	return mcp.NewToolResultText(fmt.Sprintf("FQDN '%s' not found.", fqdn)), nil
}
