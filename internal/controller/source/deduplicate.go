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

package source

import (
	"context"

	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// DeduplicateHandler removes duplicate FQDNs across source types using the
// configured priority ordering.
type DeduplicateHandler struct {
	priority []string
}

// NewDeduplicateHandler creates a new DeduplicateHandler.
func NewDeduplicateHandler(priority []string) *DeduplicateHandler {
	return &DeduplicateHandler{priority: priority}
}

// Handle implements reconciler.Handler.
func (h *DeduplicateHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[struct{}, ChainData]) error {
	if rc.Data.Index == nil {
		return nil
	}
	rc.Data.EndpointsByPortalSource = DeduplicateByPriority(rc.Data.EndpointsByPortalSource, h.priority)
	return nil
}

// DeduplicateByPriority removes duplicate FQDNs across source types within each
// portal, keeping only the endpoints from the highest-priority source for each
// FQDN name. Deduplication is at the FQDN-name level: the winning source keeps
// all its record types, losing sources drop all records for that hostname.
//
// When priority is nil or empty, no deduplication is performed.
func DeduplicateByPriority(
	endpointsByPortalSource map[PortalSourceKey][]*endpoint.Endpoint,
	priority []string,
) map[PortalSourceKey][]*endpoint.Endpoint {
	if len(priority) == 0 {
		return endpointsByPortalSource
	}

	rank := make(map[string]int, len(priority))
	for i, src := range priority {
		rank[src] = i
	}
	unlistedRank := len(priority)

	sourceRank := func(st registry.SourceType) int {
		if r, ok := rank[string(st)]; ok {
			return r
		}
		return unlistedRank
	}

	// Phase 1: per portal, elect the winning source type for each FQDN name.
	type fqdnWinner struct {
		sourceType registry.SourceType
		rank       int
	}
	winnerByPortalFQDN := make(map[string]map[string]fqdnWinner)

	for key, eps := range endpointsByPortalSource {
		winners, ok := winnerByPortalFQDN[key.PortalName]
		if !ok {
			winners = make(map[string]fqdnWinner)
			winnerByPortalFQDN[key.PortalName] = winners
		}

		srcRank := sourceRank(key.SourceType)
		for _, ep := range eps {
			existing, exists := winners[ep.DNSName]
			if !exists || srcRank < existing.rank {
				winners[ep.DNSName] = fqdnWinner{sourceType: key.SourceType, rank: srcRank}
			}
		}
	}

	// Phase 2: filter endpoints — only keep those whose FQDN name was won by this source.
	result := make(map[PortalSourceKey][]*endpoint.Endpoint, len(endpointsByPortalSource))

	for key, eps := range endpointsByPortalSource {
		winners := winnerByPortalFQDN[key.PortalName]
		var kept []*endpoint.Endpoint

		for _, ep := range eps {
			w := winners[ep.DNSName]
			if w.sourceType == key.SourceType {
				kept = append(kept, ep)
			}
		}

		if len(kept) > 0 {
			result[key] = kept
		}
	}

	return result
}
