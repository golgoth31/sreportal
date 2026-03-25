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
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

// FQDNDetails represents detailed information about a specific FQDN
type FQDNDetails struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	Group       string   `json:"group"`
	Description string   `json:"description,omitempty"`
	RecordType  string   `json:"record_type"`
	Targets     []string `json:"targets"`
	SyncStatus  string   `json:"sync_status,omitempty"`
	Portal      string   `json:"portal,omitempty"`
	Namespace   string   `json:"namespace,omitempty"`
	LastSeen    string   `json:"last_seen,omitempty"`
	DNSResource string   `json:"dns_resource,omitempty"`
}

// handleGetFQDNDetails handles the get_fqdn_details tool call
func (s *DNSServer) handleGetFQDNDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	fqdn, err := request.RequireString("fqdn")
	if err != nil {
		return mcp.NewToolResultError("fqdn parameter is required"), nil
	}

	// Normalize the FQDN for lookup
	fqdnNormalized := strings.ToLower(strings.TrimSuffix(fqdn, "."))

	// Try to find via the reader (empty recordType matches first hit)
	view, err := s.fqdnReader.Get(ctx, fqdnNormalized, "")
	if err != nil {
		if errors.Is(err, domaindns.ErrFQDNNotFound) {
			return mcp.NewToolResultText(fmt.Sprintf("FQDN '%s' not found.", fqdn)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to get FQDN details: %v", err)), nil
	}

	groupName := ""
	if len(view.Groups) > 0 {
		groupName = view.Groups[0]
	}

	details := FQDNDetails{
		Name:        view.Name,
		Source:      string(view.Source),
		Group:       groupName,
		Description: view.Description,
		RecordType:  view.RecordType,
		Targets:     view.Targets,
		SyncStatus:  view.SyncStatus,
		Portal:      view.PortalName,
		Namespace:   view.Namespace,
		DNSResource: fmt.Sprintf("%s/%s", view.Namespace, view.PortalName),
	}
	if !view.LastSeen.IsZero() {
		details.LastSeen = view.LastSeen.Format("2006-01-02T15:04:05Z07:00")
	}

	jsonBytes, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal details: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("FQDN details for '%s':\n\n%s", fqdn, string(jsonBytes))), nil
}
