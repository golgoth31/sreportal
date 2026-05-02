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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// CleanupDisabledFeaturesHandler purges data from read stores and deletes
// child resources when feature toggles are disabled on the portal.
type CleanupDisabledFeaturesHandler struct {
	client client.Client
}

// NewCleanupDisabledFeaturesHandler creates a new CleanupDisabledFeaturesHandler.
func NewCleanupDisabledFeaturesHandler(c client.Client) *CleanupDisabledFeaturesHandler {
	return &CleanupDisabledFeaturesHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *CleanupDisabledFeaturesHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	logger := log.FromContext(ctx).WithName("cleanup-features")
	portal := rc.Resource

	if !portal.Spec.Features.IsDNSEnabled() {
		if err := h.cleanupDNSData(ctx, portal, &rc.Data); err != nil {
			logger.Error(err, "failed to clean up DNS data for disabled feature")
		}
	}

	if !portal.Spec.Features.IsReleasesEnabled() {
		if err := h.cleanupReleaseData(ctx, portal, &rc.Data); err != nil {
			logger.Error(err, "failed to clean up release data for disabled feature")
		}
	}

	if !portal.Spec.Features.IsNetworkPolicyEnabled() {
		if err := h.cleanupNetworkFlowData(ctx, portal, &rc.Data); err != nil {
			logger.Error(err, "failed to clean up network flow data for disabled feature")
		}
	}

	// Clean up the remote ImageInventory shadow CR when (a) the portal is no
	// longer remote, or (b) the portal is remote but the imageInventory feature
	// is disabled. The Get quickly returns NotFound for portals that never had
	// a shadow CR, so the no-op cost is a single API call per reconcile.
	shouldCleanupRemoteImageInventory := portal.Spec.Remote == nil ||
		!portal.Spec.Features.IsImageInventoryEnabled()
	if shouldCleanupRemoteImageInventory {
		if err := h.cleanupRemoteImageInventory(ctx, portal); err != nil {
			logger.Error(err, "failed to clean up remote ImageInventory")
		}
	}

	return nil
}

// cleanupDNSData removes all DNS-related data for a portal from the FQDN read store
// and deletes DNSRecord K8s resources. DNS CRs are preserved (they hold spec data
// needed for recovery).
func (h *CleanupDisabledFeaturesHandler) cleanupDNSData(ctx context.Context, portal *sreportalv1alpha1.Portal, data *ChainData) error {
	logger := log.FromContext(ctx)

	if data.FQDNWriter == nil {
		return nil
	}

	var dnsList sreportalv1alpha1.DNSList
	if err := h.client.List(ctx, &dnsList,
		client.InNamespace(portal.Namespace),
		client.MatchingFields{portalfeatures.FieldIndexPortalRef: portal.Name},
	); err != nil {
		return fmt.Errorf("list DNS resources: %w", err)
	}

	for i := range dnsList.Items {
		resourceKey := dnsList.Items[i].Namespace + "/" + dnsList.Items[i].Name
		if err := data.FQDNWriter.Delete(ctx, resourceKey); err != nil {
			logger.Error(err, "failed to delete DNS FQDN views from read store", "key", resourceKey)
		}
	}

	var dnsRecordList sreportalv1alpha1.DNSRecordList
	if err := h.client.List(ctx, &dnsRecordList,
		client.InNamespace(portal.Namespace),
		client.MatchingFields{portalfeatures.FieldIndexPortalRef: portal.Name},
	); err != nil {
		return fmt.Errorf("list DNSRecord resources: %w", err)
	}

	for i := range dnsRecordList.Items {
		rec := &dnsRecordList.Items[i]
		resourceKey := rec.Namespace + "/" + rec.Name
		if err := data.FQDNWriter.Delete(ctx, resourceKey); err != nil {
			logger.Error(err, "failed to delete DNSRecord FQDN views from read store", "key", resourceKey)
		}
		if err := h.client.Delete(ctx, rec); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "failed to delete DNSRecord", "name", rec.Name)
		} else if err == nil {
			logger.Info("deleted DNSRecord (DNS feature disabled)", "name", rec.Name)
		}
	}

	logger.Info("cleaned up DNS data for disabled feature",
		"portal", portal.Name,
		"dnsCount", len(dnsList.Items),
		"dnsRecordCount", len(dnsRecordList.Items))

	return nil
}

// cleanupReleaseData removes release read-store projections for this portal without
// deleting Release CRs.
func (h *CleanupDisabledFeaturesHandler) cleanupReleaseData(ctx context.Context, portal *sreportalv1alpha1.Portal, data *ChainData) error {
	if data.ReleaseWriter == nil {
		return nil
	}
	logger := log.FromContext(ctx)

	var releaseList sreportalv1alpha1.ReleaseList
	if err := h.client.List(ctx, &releaseList,
		client.InNamespace(portal.Namespace),
		client.MatchingFields{portalfeatures.FieldIndexPortalRef: portal.Name},
	); err != nil {
		return fmt.Errorf("list Release resources: %w", err)
	}

	for i := range releaseList.Items {
		resourceKey := releaseList.Items[i].Namespace + "/" + releaseList.Items[i].Name
		if err := data.ReleaseWriter.Delete(ctx, resourceKey); err != nil {
			logger.Error(err, "failed to delete Release views from read store", "key", resourceKey)
		}
	}

	logger.Info("cleaned up release data for disabled feature",
		"portal", portal.Name, "releaseCount", len(releaseList.Items))
	return nil
}

// cleanupRemoteImageInventory deletes the shadow ImageInventory CR (named
// "remote-<portal>") when the imageInventory feature is disabled on a remote
// portal, or when the portal is no longer remote. The ImageInventory finalizer
// purges the read-store projection on its own as part of CR deletion.
func (h *CleanupDisabledFeaturesHandler) cleanupRemoteImageInventory(ctx context.Context, portal *sreportalv1alpha1.Portal) error {
	logger := log.FromContext(ctx)

	invName := RemoteImageInventoryName(portal.Name)
	inv := &sreportalv1alpha1.ImageInventory{}
	if err := h.client.Get(ctx, types.NamespacedName{Name: invName, Namespace: portal.Namespace}, inv); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get remote ImageInventory: %w", err)
	}

	owned := false
	for _, ref := range inv.OwnerReferences {
		if ref.UID == portal.UID {
			owned = true
			break
		}
	}
	if !owned {
		return nil
	}

	if err := h.client.Delete(ctx, inv); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("delete remote ImageInventory: %w", err)
	}
	logger.Info("deleted remote ImageInventory (feature disabled or portal no longer remote)", "name", invName)
	return nil
}

// cleanupNetworkFlowData removes network flow read-store projections for this portal
// and deletes NetworkFlowDiscovery K8s resources.
func (h *CleanupDisabledFeaturesHandler) cleanupNetworkFlowData(ctx context.Context, portal *sreportalv1alpha1.Portal, data *ChainData) error {
	logger := log.FromContext(ctx)

	var nfdList sreportalv1alpha1.NetworkFlowDiscoveryList
	if err := h.client.List(ctx, &nfdList,
		client.InNamespace(portal.Namespace),
		client.MatchingFields{portalfeatures.FieldIndexPortalRef: portal.Name},
	); err != nil {
		return fmt.Errorf("list NetworkFlowDiscovery resources: %w", err)
	}

	for i := range nfdList.Items {
		nfd := &nfdList.Items[i]
		if data.FlowGraphWriter != nil {
			if err := data.FlowGraphWriter.Delete(ctx, nfd.Name); err != nil {
				logger.Error(err, "failed to delete flow graph from read store", "key", nfd.Name)
			}
		}
		if err := h.client.Delete(ctx, nfd); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "failed to delete NetworkFlowDiscovery", "name", nfd.Name)
		} else if err == nil {
			logger.Info("deleted NetworkFlowDiscovery (networkPolicy feature disabled)", "name", nfd.Name)
		}
	}

	logger.Info("cleaned up network flow data for disabled feature",
		"portal", portal.Name, "nfdCount", len(nfdList.Items))
	return nil
}
