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

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// DeleteOrphanedHandler deletes DNSRecord resources that are no longer needed.
type DeleteOrphanedHandler struct {
	client        client.Client
	enabledLister EnabledSourcesLister
}

// NewDeleteOrphanedHandler creates a new DeleteOrphanedHandler.
func NewDeleteOrphanedHandler(c client.Client, lister EnabledSourcesLister) *DeleteOrphanedHandler {
	return &DeleteOrphanedHandler{client: c, enabledLister: lister}
}

// Handle implements reconciler.Handler.
func (h *DeleteOrphanedHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[struct{}, ChainData]) error {
	if rc.Data.Index == nil {
		return nil
	}

	logger := log.FromContext(ctx).WithName("delete-orphaned")

	for _, portal := range rc.Data.Index.Local {
		if err := h.deleteOrphaned(ctx, portal, rc.Data.EndpointsByPortalSource); err != nil {
			logger.Error(err, "failed to delete orphaned DNSRecords", "portal", portal.Name)
		}
	}

	return nil
}

func (h *DeleteOrphanedHandler) deleteOrphaned(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	activeKeys map[PortalSourceKey][]*endpoint.Endpoint,
) error {
	logger := log.FromContext(ctx).WithName("delete-orphaned")

	enabledTypes := h.enabledLister.EnabledSourceTypes()
	enabledSet := make(map[registry.SourceType]bool)
	for _, t := range enabledTypes {
		enabledSet[t] = true
	}

	var dnsRecordList sreportalv1alpha1.DNSRecordList
	if err := h.client.List(ctx, &dnsRecordList,
		client.InNamespace(portal.Namespace),
		client.MatchingFields{FieldIndexPortalRef: portal.Name},
	); err != nil {
		return err
	}

	for i := range dnsRecordList.Items {
		rec := &dnsRecordList.Items[i]
		sourceType := registry.SourceType(rec.Spec.SourceType)
		key := PortalSourceKey{PortalName: portal.Name, SourceType: sourceType}

		if !enabledSet[sourceType] || activeKeys[key] == nil {
			logger.Info("deleting orphaned DNSRecord",
				"name", rec.Name,
				"sourceType", rec.Spec.SourceType,
				"portal", portal.Name)

			if err := h.client.Delete(ctx, rec); err != nil {
				logger.Error(err, "failed to delete orphaned DNSRecord", "name", rec.Name)
				return err
			}
		}
	}

	return nil
}
