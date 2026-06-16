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
	"fmt"
	"slices"
	"sort"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/adapter"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ProjectStoreHandler converts a DNSRecord's endpoints into FQDN views and pushes
// them into the FQDN read store. A nil writer is a no-op.
type ProjectStoreHandler struct {
	fqdnWriter domaindns.FQDNWriter
}

// NewProjectStoreHandler creates a new ProjectStoreHandler.
func NewProjectStoreHandler(w domaindns.FQDNWriter) *ProjectStoreHandler {
	return &ProjectStoreHandler{fqdnWriter: w}
}

// Handle implements reconciler.Handler.
func (h *ProjectStoreHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNSRecord, ChainData]) error {
	if h.fqdnWriter == nil {
		return nil
	}
	views := DNSRecordToFQDNViews(rc.Resource, rc.Data.GroupMapping)
	if err := h.fqdnWriter.Replace(ctx, rc.Data.ResourceKey, rc.Resource.Spec.PortalRef, views); err != nil {
		return fmt.Errorf("project store: %w", err)
	}
	// Annotate the read store with the owning DNS CR so per-DNS conflict
	// reporting can scope events to a specific owner. Skip if the owner is
	// unknown (defensive — should not happen for v1alpha2 DNSRecords created
	// by the DNS controller).
	if rc.Data.OwnerDNSName != "" {
		if w, ok := h.fqdnWriter.(interface {
			AnnotateOwner(recordKey, dnsNS, dnsName string)
		}); ok {
			w.AnnotateOwner(rc.Data.ResourceKey, rc.Resource.Namespace, rc.Data.OwnerDNSName)
		}
	}
	return nil
}

// DNSRecordToFQDNViews converts a v1alpha2.DNSRecord's status endpoints into a
// deduplicated slice of FQDNViews suitable for the read store. It reuses the
// adapter layer for group mapping and sets PortalName from spec.PortalRef.
// Source is set to SourceManual when spec.Origin is "manual", otherwise SourceExternalDNS.
func DNSRecordToFQDNViews(
	record *v1alpha2.DNSRecord,
	groupMapping *v1alpha2.GroupMappingSpec,
) []domaindns.FQDNView {
	if len(record.Status.Endpoints) == 0 {
		return nil
	}

	source := domaindns.SourceExternalDNS
	if record.Spec.Origin == v1alpha2.DNSRecordOriginManual {
		source = domaindns.SourceManual
	}

	groups := adapter.EndpointStatusToGroupsV2(record.Status.Endpoints, groupMapping)

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
					Source:      source,
					SourceType:  string(record.Spec.SourceType),
					Groups:      []string{group.Name},
					Description: fqdn.Description,
					RecordType:  fqdn.RecordType,
					Targets:     fqdn.Targets,
					LastSeen:    fqdn.LastSeen.Time,
					Portals:     []string{record.Spec.PortalRef},
					Namespace:   record.Namespace,
					SyncStatus:  string(fqdn.SyncStatus),
				}
				if fqdn.OriginRef != nil {
					raw := fqdn.OriginRef.Kind + "/" + fqdn.OriginRef.Namespace + "/" + fqdn.OriginRef.Name
					if ref, err := domaindns.ParseResourceRef(raw); err == nil {
						view.OriginRef = &ref
					}
				}
				seen[key] = &view
			}
		}
	}

	views := make([]domaindns.FQDNView, 0, len(seen))
	for _, v := range seen {
		views = append(views, *v)
	}
	sort.Slice(views, func(i, j int) bool {
		return views[i].Name < views[j].Name
	})
	return views
}
