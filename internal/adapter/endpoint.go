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

package adapter

import (
	"maps"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
)

const (
	// SourceExternalDNS indicates an FQDN discovered from external-dns sources.
	SourceExternalDNS = "external-dns"

	// PortalAnnotationKey is the label key used on external-dns endpoints
	// to route them to a specific portal.
	PortalAnnotationKey = "sreportal.io/portal"

	// GroupAnnotationKey is the label key used on external-dns endpoints
	// to assign them to a specific group. Takes highest priority over other
	// grouping rules (labelKey, namespace mapping, default group).
	GroupAnnotationKey = "sreportal.io/group"
)

// SreportalAnnotations lists the annotation keys that should be propagated
// from K8s resource annotations to endpoint labels.
var SreportalAnnotations = []string{PortalAnnotationKey, GroupAnnotationKey}

// EnrichEndpointLabels copies sreportal annotations from K8s resource annotations
// to the endpoint's labels. Existing endpoint labels are not overwritten.
func EnrichEndpointLabels(ep *endpoint.Endpoint, annotations map[string]string) {
	if ep == nil || len(annotations) == 0 {
		return
	}
	if ep.Labels == nil {
		ep.Labels = make(map[string]string)
	}
	for _, key := range SreportalAnnotations {
		if val, ok := annotations[key]; ok && val != "" {
			if _, exists := ep.Labels[key]; !exists {
				ep.Labels[key] = val
			}
		}
	}
}

// ResolvePortal extracts the portal name from an endpoint's labels.
// Returns the portal name or empty string if not annotated.
func ResolvePortal(ep *endpoint.Endpoint) string {
	if ep == nil {
		return ""
	}
	if val, ok := ep.Labels[PortalAnnotationKey]; ok && val != "" {
		return val
	}
	return ""
}

// EndpointsToGroups converts external-dns endpoints to DNS CR status groups.
// It groups endpoints based on the provided mapping configuration.
func EndpointsToGroups(endpoints []*endpoint.Endpoint, mapping *config.GroupMappingConfig) []sreportalv1alpha1.FQDNGroupStatus {
	if mapping == nil {
		mapping = &config.GroupMappingConfig{
			DefaultGroup: "Services",
		}
	}

	// Group endpoints by mapping rules
	groups := make(map[string]*sreportalv1alpha1.FQDNGroupStatus)
	now := metav1.Now()

	for _, ep := range endpoints {
		groupName := resolveGroup(ep, mapping)

		if _, exists := groups[groupName]; !exists {
			groups[groupName] = &sreportalv1alpha1.FQDNGroupStatus{
				Name:   groupName,
				Source: SourceExternalDNS,
				FQDNs:  []sreportalv1alpha1.FQDNStatus{},
			}
		}

		fqdn := sreportalv1alpha1.FQDNStatus{
			FQDN:       ep.DNSName,
			RecordType: ep.RecordType,
			Targets:    ep.Targets,
			LastSeen:   now,
		}

		// Add TTL if present
		if ep.RecordTTL > 0 {
			// TTL is stored in the endpoint as a TTL type (int64)
			fqdn.RecordType = ep.RecordType
		}

		groups[groupName].FQDNs = append(groups[groupName].FQDNs, fqdn)
	}

	// Convert map to sorted slice
	result := make([]sreportalv1alpha1.FQDNGroupStatus, 0, len(groups))
	for _, group := range groups {
		// Sort FQDNs within each group
		sort.Slice(group.FQDNs, func(i, j int) bool {
			return group.FQDNs[i].FQDN < group.FQDNs[j].FQDN
		})
		result = append(result, *group)
	}

	// Sort groups by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// resolveGroup determines the group name for an endpoint based on mapping rules.
func resolveGroup(ep *endpoint.Endpoint, mapping *config.GroupMappingConfig) string {
	// 1. Check sreportal.io/group annotation (highest priority)
	if val, ok := ep.Labels[GroupAnnotationKey]; ok && val != "" {
		return val
	}

	// 2. Check endpoint label if labelKey is configured
	if mapping.LabelKey != "" {
		if val, ok := ep.Labels[mapping.LabelKey]; ok && val != "" {
			return val
		}
	}

	// 3. Check resource label for namespace mapping
	if resource, ok := ep.Labels[endpoint.ResourceLabelKey]; ok {
		ns := extractNamespace(resource)
		if ns != "" && mapping.ByNamespace != nil {
			if groupName, ok := mapping.ByNamespace[ns]; ok {
				return groupName
			}
		}
	}

	// 4. Fall back to default group
	if mapping.DefaultGroup != "" {
		return mapping.DefaultGroup
	}

	return "Services"
}

// extractNamespace extracts the namespace from a resource label.
// The resource label format from external-dns is "kind/namespace/name" (e.g., "service/default/my-svc").
func extractNamespace(resource string) string {
	parts := strings.Split(resource, "/")
	if len(parts) == 3 {
		return parts[1]
	}
	return ""
}

// MergeGroups merges existing groups with new external groups.
// Manual groups (source != external-dns) are preserved, while external groups are replaced.
func MergeGroups(existing, external []sreportalv1alpha1.FQDNGroupStatus) []sreportalv1alpha1.FQDNGroupStatus {
	// Start with manual groups from existing
	result := make([]sreportalv1alpha1.FQDNGroupStatus, 0, len(existing)+len(external))
	for _, g := range existing {
		if g.Source != SourceExternalDNS {
			result = append(result, g)
		}
	}

	// Add all external groups
	result = append(result, external...)

	// Sort by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// ToEndpointStatus converts external-dns endpoints to EndpointStatus slice.
// This is used when storing endpoints in DNSRecord status.
func ToEndpointStatus(endpoints []*endpoint.Endpoint) []sreportalv1alpha1.EndpointStatus {
	now := metav1.Now()
	result := make([]sreportalv1alpha1.EndpointStatus, 0, len(endpoints))

	for _, ep := range endpoints {
		status := sreportalv1alpha1.EndpointStatus{
			DNSName:    ep.DNSName,
			RecordType: ep.RecordType,
			Targets:    ep.Targets,
			TTL:        int64(ep.RecordTTL),
			LastSeen:   now,
		}

		// Copy labels if present
		if len(ep.Labels) > 0 {
			status.Labels = make(map[string]string, len(ep.Labels))
			maps.Copy(status.Labels, ep.Labels)
		}

		result = append(result, status)
	}

	// Sort by DNS name for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		if result[i].DNSName == result[j].DNSName {
			return result[i].RecordType < result[j].RecordType
		}
		return result[i].DNSName < result[j].DNSName
	})

	return result
}

// EndpointStatusToGroups converts EndpointStatus slice to FQDNGroupStatus slice.
// This is used when aggregating endpoints from DNSRecord resources into DNS status.
func EndpointStatusToGroups(endpoints []sreportalv1alpha1.EndpointStatus, mapping *config.GroupMappingConfig) []sreportalv1alpha1.FQDNGroupStatus {
	if mapping == nil {
		mapping = &config.GroupMappingConfig{
			DefaultGroup: "Services",
		}
	}

	// Group endpoints by mapping rules
	groups := make(map[string]*sreportalv1alpha1.FQDNGroupStatus)

	for _, ep := range endpoints {
		groupName := resolveGroupFromEndpointStatus(&ep, mapping)

		if _, exists := groups[groupName]; !exists {
			groups[groupName] = &sreportalv1alpha1.FQDNGroupStatus{
				Name:   groupName,
				Source: SourceExternalDNS,
				FQDNs:  []sreportalv1alpha1.FQDNStatus{},
			}
		}

		fqdn := sreportalv1alpha1.FQDNStatus{
			FQDN:       ep.DNSName,
			RecordType: ep.RecordType,
			Targets:    ep.Targets,
			LastSeen:   ep.LastSeen,
		}

		groups[groupName].FQDNs = append(groups[groupName].FQDNs, fqdn)
	}

	// Convert map to sorted slice
	result := make([]sreportalv1alpha1.FQDNGroupStatus, 0, len(groups))
	for _, group := range groups {
		// Sort FQDNs within each group
		sort.Slice(group.FQDNs, func(i, j int) bool {
			return group.FQDNs[i].FQDN < group.FQDNs[j].FQDN
		})
		result = append(result, *group)
	}

	// Sort groups by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// resolveGroupFromEndpointStatus determines the group name for an EndpointStatus based on mapping rules.
func resolveGroupFromEndpointStatus(ep *sreportalv1alpha1.EndpointStatus, mapping *config.GroupMappingConfig) string {
	// 1. Check sreportal.io/group annotation (highest priority)
	if val, ok := ep.Labels[GroupAnnotationKey]; ok && val != "" {
		return val
	}

	// 2. Check endpoint label if labelKey is configured
	if mapping.LabelKey != "" {
		if val, ok := ep.Labels[mapping.LabelKey]; ok && val != "" {
			return val
		}
	}

	// 3. Check resource label for namespace mapping
	if resource, ok := ep.Labels[endpoint.ResourceLabelKey]; ok {
		ns := extractNamespace(resource)
		if ns != "" && mapping.ByNamespace != nil {
			if groupName, ok := mapping.ByNamespace[ns]; ok {
				return groupName
			}
		}
	}

	// 4. Fall back to default group
	if mapping.DefaultGroup != "" {
		return mapping.DefaultGroup
	}

	return "Services"
}
