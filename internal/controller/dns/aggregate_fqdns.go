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

package dns

import (
	"context"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// DataKeyAggregatedGroups is the key for storing aggregated groups in context
	DataKeyAggregatedGroups = "aggregatedGroups"

	// SourceManual indicates a manually configured FQDN
	SourceManual = "manual"
	// SourceExternalDNS indicates an FQDN discovered from external-dns
	SourceExternalDNS = "external-dns"
)

// AggregateFQDNsHandler aggregates FQDNs from all sources into groups
type AggregateFQDNsHandler struct{}

// NewAggregateFQDNsHandler creates a new AggregateFQDNsHandler
func NewAggregateFQDNsHandler() *AggregateFQDNsHandler {
	return &AggregateFQDNsHandler{}
}

// Handle implements reconciler.Handler
func (h *AggregateFQDNsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DNS]) error {
	log := logf.FromContext(ctx).WithName("aggregate-fqdns")
	now := metav1.Now()

	// Map to store groups by name
	groupMap := make(map[string]*sreportalv1alpha1.FQDNGroupStatus)

	// Add external-dns groups from DNSRecords (set by AggregateDNSRecordsHandler)
	if externalGroups, ok := rc.Data[DataKeyExternalGroups].([]sreportalv1alpha1.FQDNGroupStatus); ok {
		for _, group := range externalGroups {
			groupCopy := group
			groupMap[group.Name] = &groupCopy
		}
		log.V(1).Info("added external groups from DNSRecords", "count", len(externalGroups))
	}

	// Process manual groups — merge FQDNs with any external group of the same name.
	if groups, ok := rc.Data[DataKeyManualGroups].([]sreportalv1alpha1.DNSGroup); ok {
		for _, group := range groups {
			manualFQDNs := make([]sreportalv1alpha1.FQDNStatus, 0, len(group.Entries))
			for _, entry := range group.Entries {
				manualFQDNs = append(manualFQDNs, sreportalv1alpha1.FQDNStatus{
					FQDN:        entry.FQDN,
					Description: entry.Description,
					LastSeen:    now,
				})
			}

			if existing, ok := groupMap[group.Name]; ok {
				// Same name: append manual FQDNs to the external group and keep both.
				existing.FQDNs = append(existing.FQDNs, manualFQDNs...)
				if group.Description != "" {
					existing.Description = group.Description
				}
				sort.Slice(existing.FQDNs, func(i, j int) bool {
					return existing.FQDNs[i].FQDN < existing.FQDNs[j].FQDN
				})
			} else {
				// No external group with this name: create a pure manual group.
				sort.Slice(manualFQDNs, func(i, j int) bool {
					return manualFQDNs[i].FQDN < manualFQDNs[j].FQDN
				})
				groupMap[group.Name] = &sreportalv1alpha1.FQDNGroupStatus{
					Name:        group.Name,
					Description: group.Description,
					Source:      SourceManual,
					FQDNs:       manualFQDNs,
				}
			}
		}
	}

	// Convert map to sorted slice
	groups := make([]sreportalv1alpha1.FQDNGroupStatus, 0, len(groupMap))
	for _, group := range groupMap {
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	log.V(1).Info("aggregated groups", "count", len(groups))
	rc.Data[DataKeyAggregatedGroups] = groups
	return nil
}
