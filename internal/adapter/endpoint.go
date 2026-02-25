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
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

const (
	// SourceExternalDNS indicates an FQDN discovered from external-dns sources.
	SourceExternalDNS = "external-dns"

	// PortalAnnotationKey is the label key used on external-dns endpoints
	// to route them to a specific portal.
	PortalAnnotationKey = "sreportal.io/portal"

	// GroupsAnnotationKey is the label key used on external-dns endpoints
	// to assign them to one or more groups (comma-separated). Takes highest
	// priority over other grouping rules (labelKey, namespace mapping, default group).
	GroupsAnnotationKey = "sreportal.io/groups"

	// IgnoreAnnotationKey is the label key used on external-dns endpoints
	// to exclude them from DNS discovery. When set to "true", the endpoint
	// is silently dropped during group conversion.
	IgnoreAnnotationKey = "sreportal.io/ignore"
)

// SreportalAnnotations lists the annotation keys that should be propagated
// from K8s resource annotations to endpoint labels.
var SreportalAnnotations = []string{PortalAnnotationKey, GroupsAnnotationKey, IgnoreAnnotationKey}

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

// IsIgnored returns true when the endpoint has the sreportal.io/ignore label set to "true".
func IsIgnored(ep *endpoint.Endpoint) bool {
	if ep == nil {
		return false
	}
	return ep.Labels[IgnoreAnnotationKey] == "true"
}

// IsEndpointStatusIgnored returns true when the EndpointStatus has the sreportal.io/ignore label set to "true".
func IsEndpointStatusIgnored(ep *sreportalv1alpha1.EndpointStatus) bool {
	if ep == nil {
		return false
	}
	return ep.Labels[IgnoreAnnotationKey] == "true"
}

// strategyFromConfig builds a GroupMappingStrategy from the provided config.
// A nil mapping yields a strategy with the "Services" default group.
func strategyFromConfig(mapping *config.GroupMappingConfig) domaindns.GroupMappingStrategy {
	if mapping == nil {
		return domaindns.GroupMappingStrategy{DefaultGroup: "Services"}
	}
	return domaindns.GroupMappingStrategy{
		DefaultGroup: mapping.DefaultGroup,
		LabelKey:     mapping.LabelKey,
		ByNamespace:  mapping.ByNamespace,
	}
}

// EndpointsToGroups converts external-dns endpoints to DNS CR status groups.
// It groups endpoints based on the provided mapping configuration.
func EndpointsToGroups(endpoints []*endpoint.Endpoint, mapping *config.GroupMappingConfig) []sreportalv1alpha1.FQDNGroupStatus {
	strategy := strategyFromConfig(mapping)

	// Group endpoints by mapping rules
	groups := make(map[string]*sreportalv1alpha1.FQDNGroupStatus)
	now := metav1.Now()

	for _, ep := range endpoints {
		if IsIgnored(ep) {
			continue
		}

		ns := extractNamespace(ep.Labels[endpoint.ResourceLabelKey])
		groupNames := strategy.Resolve(ep.Labels, ns)

		fqdn := sreportalv1alpha1.FQDNStatus{
			FQDN:       ep.DNSName,
			RecordType: ep.RecordType,
			Targets:    ep.Targets,
			LastSeen:   now,
		}

		for _, groupName := range groupNames {
			if _, exists := groups[groupName]; !exists {
				groups[groupName] = &sreportalv1alpha1.FQDNGroupStatus{
					Name:   groupName,
					Source: SourceExternalDNS,
					FQDNs:  []sreportalv1alpha1.FQDNStatus{},
				}
			}

			groups[groupName].FQDNs = append(groups[groupName].FQDNs, fqdn)
		}
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

// fqdnKey uniquely identifies a FQDN within a group for deduplication.
type fqdnKey struct {
	groupName  string
	dnsName    string
	recordType string
}

// EndpointStatusToGroups converts EndpointStatus slice to FQDNGroupStatus slice.
// This is used when aggregating endpoints from DNSRecord resources into DNS status.
// Duplicate FQDNs (same DNSName + RecordType) within the same group are merged,
// combining their targets.
func EndpointStatusToGroups(endpoints []sreportalv1alpha1.EndpointStatus, mapping *config.GroupMappingConfig) []sreportalv1alpha1.FQDNGroupStatus {
	strategy := strategyFromConfig(mapping)

	// Group endpoints by mapping rules
	groups := make(map[string]*sreportalv1alpha1.FQDNGroupStatus)
	// Track seen FQDNs per group to deduplicate and merge targets
	seen := make(map[fqdnKey]int) // key → index in group.FQDNs

	for _, ep := range endpoints {
		if IsEndpointStatusIgnored(&ep) {
			continue
		}

		ns := extractNamespace(ep.Labels[endpoint.ResourceLabelKey])
		groupNames := strategy.Resolve(ep.Labels, ns)

		for _, groupName := range groupNames {
			if _, exists := groups[groupName]; !exists {
				groups[groupName] = &sreportalv1alpha1.FQDNGroupStatus{
					Name:   groupName,
					Source: SourceExternalDNS,
					FQDNs:  []sreportalv1alpha1.FQDNStatus{},
				}
			}

			key := fqdnKey{groupName: groupName, dnsName: ep.DNSName, recordType: ep.RecordType}
			if idx, dup := seen[key]; dup {
				// Merge targets from duplicate endpoint
				existing := &groups[groupName].FQDNs[idx]
				existing.Targets = mergeTargets(existing.Targets, ep.Targets)
				if ep.LastSeen.After(existing.LastSeen.Time) {
					existing.LastSeen = ep.LastSeen
				}
			} else {
				seen[key] = len(groups[groupName].FQDNs)
				groups[groupName].FQDNs = append(groups[groupName].FQDNs, sreportalv1alpha1.FQDNStatus{
					FQDN:       ep.DNSName,
					RecordType: ep.RecordType,
					Targets:    ep.Targets,
					LastSeen:   ep.LastSeen,
				})
			}
		}
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

// mergeTargets merges two target slices, deduplicating entries.
func mergeTargets(existing, additional []string) []string {
	set := make(map[string]struct{}, len(existing))
	for _, t := range existing {
		set[t] = struct{}{}
	}
	for _, t := range additional {
		if _, exists := set[t]; !exists {
			existing = append(existing, t)
			set[t] = struct{}{}
		}
	}
	sort.Strings(existing)
	return existing
}
