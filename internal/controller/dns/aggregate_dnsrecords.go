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

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// DataKeyExternalGroups is the key for storing external-dns groups in context
	DataKeyExternalGroups = "externalGroups"

	// IndexFieldPortalRef is the index field name for looking up DNSRecords by portal
	IndexFieldPortalRef = "spec.portalRef"
)

// AggregateDNSRecordsHandler aggregates endpoints from DNSRecord resources
// belonging to the same portal as the DNS resource being reconciled.
type AggregateDNSRecordsHandler struct {
	client         client.Client
	config         *config.GroupMappingConfig
	sourcePriority []string
}

// NewAggregateDNSRecordsHandler creates a new AggregateDNSRecordsHandler.
// sourcePriority defines the order in which source types are preferred when the
// same FQDN+RecordType appears in multiple sources. Pass nil to merge all sources
// (backward-compatible default).
func NewAggregateDNSRecordsHandler(c client.Client, cfg *config.GroupMappingConfig, sourcePriority []string) *AggregateDNSRecordsHandler {
	return &AggregateDNSRecordsHandler{
		client:         c,
		config:         cfg,
		sourcePriority: sourcePriority,
	}
}

// Handle implements reconciler.Handler
func (h *AggregateDNSRecordsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DNS]) error {
	log := logf.FromContext(ctx).WithName("aggregate-dnsrecords")

	// List DNSRecords belonging to the same portal as this DNS resource
	var dnsRecordList sreportalv1alpha1.DNSRecordList
	if err := h.client.List(ctx, &dnsRecordList,
		client.InNamespace(rc.Resource.Namespace),
		client.MatchingFields{IndexFieldPortalRef: rc.Resource.Spec.PortalRef},
	); err != nil {
		log.Error(err, "failed to list DNSRecords")
		return err
	}

	log.V(1).Info("found DNSRecords", "count", len(dnsRecordList.Items), "portalRef", rc.Resource.Spec.PortalRef)

	// Group endpoints by source type for priority-based deduplication
	endpointsBySource := make(map[string][]sreportalv1alpha1.EndpointStatus)
	for _, rec := range dnsRecordList.Items {
		log.V(2).Info("processing DNSRecord",
			"name", rec.Name,
			"sourceType", rec.Spec.SourceType,
			"endpointCount", len(rec.Status.Endpoints))
		endpointsBySource[rec.Spec.SourceType] = append(endpointsBySource[rec.Spec.SourceType], rec.Status.Endpoints...)
	}

	// Apply source priority (or flatten without deduplication when priority is not configured)
	allEndpoints := adapter.ApplySourcePriority(endpointsBySource, h.sourcePriority)

	log.V(1).Info("aggregated endpoints", "totalCount", len(allEndpoints))

	// Convert endpoints to groups using mapping configuration
	groups := adapter.EndpointStatusToGroups(allEndpoints, h.config)

	log.V(1).Info("converted to groups", "groupCount", len(groups))

	// Store in context for downstream handlers
	rc.Data[DataKeyExternalGroups] = groups

	return nil
}
