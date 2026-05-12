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
	"slices"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/config"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ProjectStoreHandler converts a DNSRecord's endpoints into FQDN views and pushes
// them into the FQDN read store. A nil writer is a no-op.
type ProjectStoreHandler struct {
	fqdnWriter   domaindns.FQDNWriter
	groupMapping *config.GroupMappingConfig
}

// NewProjectStoreHandler creates a new ProjectStoreHandler.
func NewProjectStoreHandler(w domaindns.FQDNWriter, groupMapping *config.GroupMappingConfig) *ProjectStoreHandler {
	return &ProjectStoreHandler{fqdnWriter: w, groupMapping: groupMapping}
}

// Handle implements reconciler.Handler.
func (h *ProjectStoreHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DNSRecord, ChainData]) error {
	if h.fqdnWriter == nil {
		return nil
	}

	views := DNSRecordToFQDNViews(rc.Resource, h.groupMapping)
	if err := h.fqdnWriter.Replace(ctx, rc.Data.ResourceKey, views); err != nil {
		log.FromContext(ctx).Error(err, "failed to update FQDN read store")
	}
	return nil
}

// DNSRecordToFQDNViews converts a DNSRecord resource's status endpoints into a
// deduplicated slice of FQDNViews suitable for the read store. It reuses the
// adapter layer for group mapping and sets PortalName from spec.PortalRef.
func DNSRecordToFQDNViews(
	record *sreportalv1alpha1.DNSRecord,
	groupMapping *config.GroupMappingConfig,
) []domaindns.FQDNView {
	if len(record.Status.Endpoints) == 0 {
		return nil
	}

	groups := adapter.EndpointStatusToGroups(record.Status.Endpoints, groupMapping)

	seen := make(map[string]*domaindns.FQDNView)

	for _, group := range groups {
		for _, fqdn := range group.FQDNs {
			key := fqdn.FQDN + "/" + fqdn.RecordType
			if existing, ok := seen[key]; ok {
				if !slices.Contains(existing.Groups, group.Name) {
					existing.Groups = append(existing.Groups, group.Name)
				}
			} else {
				view := domaindns.FQDNView{
					Name:        fqdn.FQDN,
					Source:      domaindns.SourceExternalDNS,
					SourceType:  record.Spec.SourceType,
					Groups:      []string{group.Name},
					Description: fqdn.Description,
					RecordType:  fqdn.RecordType,
					Targets:     fqdn.Targets,
					LastSeen:    fqdn.LastSeen.Time,
					PortalName:  record.Spec.PortalRef,
					Namespace:   record.Namespace,
					SyncStatus:  fqdn.SyncStatus,
				}
				if fqdn.OriginRef != nil {
					ref, _ := domaindns.ParseResourceRef(
						fqdn.OriginRef.Kind + "/" + fqdn.OriginRef.Namespace + "/" + fqdn.OriginRef.Name,
					)
					view.OriginRef = &ref
				}
				seen[key] = &view
			}
		}
	}

	views := make([]domaindns.FQDNView, 0, len(seen))
	for _, v := range seen {
		views = append(views, *v)
	}

	return views
}
