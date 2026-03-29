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

package chain

import (
	"context"

	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// maxSourceConsecutiveFailures is the threshold after which a persistently
	// failing source is surfaced as a NotReady condition on its DNSRecord.
	maxSourceConsecutiveFailures = 5
)

// CollectEndpointsHandler gathers endpoints from every configured source and
// routes each endpoint to the appropriate (portalName, sourceType) bucket.
type CollectEndpointsHandler struct {
	enricher       EndpointEnricher
	failureTracker SourceFailureTracker
}

// NewCollectEndpointsHandler creates a new CollectEndpointsHandler.
func NewCollectEndpointsHandler(
	enricher EndpointEnricher,
	failureTracker SourceFailureTracker,
) *CollectEndpointsHandler {
	return &CollectEndpointsHandler{
		enricher:       enricher,
		failureTracker: failureTracker,
	}
}

// Handle implements reconciler.Handler.
func (h *CollectEndpointsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[struct{}, ChainData]) error {
	if rc.Data.Index == nil {
		return nil // no portals to reconcile
	}

	logger := log.FromContext(ctx).WithName("collect-endpoints")
	idx := rc.Data.Index
	result := make(map[PortalSourceKey][]*endpoint.Endpoint)

	// Track component requests by (portalName, displayName) for dedup.
	type compKey struct{ portal, name string }
	seenComponents := make(map[compKey]struct{})

	for _, ts := range rc.Data.TypedSources {
		endpoints, err := ts.Source.Endpoints(ctx)
		if err != nil {
			count := h.failureTracker.RecordFailure(ts.Type)
			logger.Error(err, "failed to get endpoints from source",
				"sourceType", ts.Type, "consecutiveFailures", count)
			metrics.SourceErrorsTotal.WithLabelValues(string(ts.Type)).Inc()
			if count >= maxSourceConsecutiveFailures {
				h.failureTracker.MarkDegraded(ctx, idx.Main, ts.Type, err, count)
			}
			continue
		}

		if prev := h.failureTracker.RecordRecovery(ts.Type); prev > 0 {
			logger.Info("source recovered after consecutive failures",
				"sourceType", ts.Type, "previousFailures", prev)
		}

		metrics.SourceEndpointsCollected.WithLabelValues(string(ts.Type)).Set(float64(len(endpoints)))
		logger.V(1).Info("collected endpoints from source", "sourceType", ts.Type, "count", len(endpoints))

		h.enricher.EnrichEndpoints(ctx, ts.Type, endpoints)

		for _, ep := range endpoints {
			portalName, target := resolveEndpointPortal(ctx, ep, idx)
			if target == nil {
				continue
			}
			key := PortalSourceKey{PortalName: portalName, SourceType: ts.Type}
			result[key] = append(result[key], ep)

			// Collect component request if annotated (first-seen wins).
			if compSpec := adapter.ResolveComponentSpec(ep); compSpec != nil {
				ck := compKey{portal: portalName, name: compSpec.DisplayName}
				if _, seen := seenComponents[ck]; !seen {
					seenComponents[ck] = struct{}{}
					rc.Data.ComponentRequests = append(rc.Data.ComponentRequests, ComponentRequest{
						PortalName:  portalName,
						DisplayName: compSpec.DisplayName,
						Group:       compSpec.Group,
						Description: compSpec.Description,
						Link:        compSpec.Link,
						Status:      compSpec.Status,
					})
				}
			}
		}
	}

	rc.Data.EndpointsByPortalSource = result
	return nil
}

// resolveEndpointPortal maps an endpoint to its target local portal.
func resolveEndpointPortal(
	ctx context.Context,
	ep *endpoint.Endpoint,
	idx *PortalIndex,
) (string, *sreportalv1alpha1.Portal) {
	logger := log.FromContext(ctx).WithName("source")

	portalName := adapter.ResolvePortal(ep)
	var target *sreportalv1alpha1.Portal

	if portalName == "" {
		// No annotation → route to main portal (if available)
		if idx.Main == nil {
			logger.Info("discarding endpoint: no annotation and no main portal available",
				"endpoint", ep.DNSName)
			return "", nil
		}
		portalName = idx.Main.Name
		target = idx.Main
	} else if p := idx.ByName[portalName]; p == nil {
		logger.Info("discarding endpoint: annotated portal not found",
			"annotatedPortal", portalName, "endpoint", ep.DNSName)
		return "", nil
	} else if p.Spec.Remote != nil {
		logger.Info("discarding endpoint: annotated portal is remote",
			"annotatedPortal", portalName, "endpoint", ep.DNSName)
		return "", nil
	} else if !p.Spec.Features.IsDNSEnabled() {
		logger.Info("discarding endpoint: annotated portal has DNS feature disabled",
			"annotatedPortal", portalName, "endpoint", ep.DNSName)
		return "", nil
	} else {
		target = p
	}

	return portalName, target
}
