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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
)

// SyncRemoteDNSHandler creates or updates a DNS CR with FQDNs fetched from a remote portal.
// No-op for local portals or when DNS feature is disabled.
type SyncRemoteDNSHandler struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewSyncRemoteDNSHandler creates a new SyncRemoteDNSHandler.
func NewSyncRemoteDNSHandler(c client.Client, scheme *runtime.Scheme) *SyncRemoteDNSHandler {
	return &SyncRemoteDNSHandler{client: c, scheme: scheme}
}

// Handle implements reconciler.Handler.
func (h *SyncRemoteDNSHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	portal := rc.Resource
	if portal.Spec.Remote == nil {
		return nil
	}

	remoteLog := log.Default().WithName("portal").WithName("remote")

	if !portal.Spec.Features.IsDNSEnabled() {
		remoteLog.V(1).Info("DNS feature disabled, skipping remote DNS sync", "portal", portal.Name)
		return nil
	}

	if err := h.reconcileRemoteDNS(ctx, portal, rc.Data.FetchResult, &rc.Data); err != nil {
		remoteLog.Warn("failed to reconcile DNS for remote portal", "name", portal.Name, "namespace", portal.Namespace, "error", err.Error())
		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "DNSSynced",
			Status:             metav1.ConditionFalse,
			Reason:             "DNSSyncFailed",
			Message:            fmt.Sprintf("Failed to sync DNS from remote portal: %v", err),
			LastTransitionTime: metav1.Now(),
		})
	} else {
		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "DNSSynced",
			Status:             metav1.ConditionTrue,
			Reason:             "DNSSyncSuccess",
			Message:            fmt.Sprintf("Synced %d FQDNs from remote portal", rc.Data.FetchResult.FQDNCount),
			LastTransitionTime: metav1.Now(),
		})
	}

	return nil
}

func (h *SyncRemoteDNSHandler) reconcileRemoteDNS(ctx context.Context, portal *sreportalv1alpha1.Portal, result *remoteclient.FetchResult, data *ChainData) error {
	logger := log.FromContext(ctx)

	dnsName := RemoteDNSName(portal.Name)
	dns := &sreportalv1alpha2.DNS{}

	err := h.client.Get(ctx, types.NamespacedName{Name: dnsName, Namespace: portal.Namespace}, dns)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get DNS: %w", err)
	}

	isNew := errors.IsNotFound(err)
	if isNew {
		dns = &sreportalv1alpha2.DNS{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dnsName,
				Namespace: portal.Namespace,
			},
			Spec: sreportalv1alpha2.DNSSpec{
				PortalRef:    portal.Name,
				IsRemote:     true,
				GroupMapping: sreportalv1alpha2.GroupMappingSpec{DefaultGroup: "Services"},
				Reconciliation: sreportalv1alpha2.ReconciliationSpec{
					Interval:     metav1.Duration{Duration: 5 * time.Minute},
					RetryOnError: metav1.Duration{Duration: 30 * time.Second},
				},
			},
		}
	}

	if err := controllerutil.SetControllerReference(portal, dns, h.scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	if isNew {
		if err := h.client.Create(ctx, dns); err != nil {
			return fmt.Errorf("failed to create DNS: %w", err)
		}
		logger.Info("created DNS CR for remote portal", "dns", dnsName, "portal", portal.Name)
	} else {
		dns.Spec.PortalRef = portal.Name
		dns.Spec.IsRemote = true
		if err := h.client.Update(ctx, dns); err != nil {
			return fmt.Errorf("failed to update DNS: %w", err)
		}
	}

	dnsBase := dns.DeepCopy()
	now := metav1.Now()
	dns.Status.LastReconcileTime = &now
	meta.SetStatusCondition(&dns.Status.Conditions, metav1.Condition{
		Type:               conditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "RemoteSyncSuccess",
		Message:            fmt.Sprintf("Successfully synced %d FQDNs from remote portal", result.FQDNCount),
		LastTransitionTime: now,
	})

	if err := h.client.Status().Patch(ctx, dns, client.MergeFrom(dnsBase)); err != nil {
		return fmt.Errorf("patch DNS status: %w", err)
	}

	// Project remote FQDNs directly into the FQDN read store. The read store
	// (in-memory) is the source of truth for the API/UI; the DNS CR no longer
	// materialises grouped FQDNs in its status.
	if data.FQDNWriter != nil {
		resourceKey := dns.Namespace + "/" + dns.Name
		views := fqdnViewsFromRemoteGroups(result.Groups, dns.Spec.PortalRef, dns.Namespace)
		if err := data.FQDNWriter.Replace(ctx, resourceKey, dns.Spec.PortalRef, views); err != nil {
			logger.Error(err, "failed to update FQDN read store for remote DNS")
		}
	}

	logger.Info("synced remote FQDNs",
		"dns", dnsName,
		"portal", portal.Name,
		"fqdnCount", result.FQDNCount,
		"groupCount", len(result.Groups))

	return nil
}

// fqdnViewsFromRemoteGroups builds a deduplicated slice of FQDNViews from the
// grouped FQDNs returned by a remote portal. Duplicate FQDN/recordType pairs
// (which can occur when a single FQDN appears in multiple groups) collapse
// into one view whose Groups slice carries every group it belongs to.
func fqdnViewsFromRemoteGroups(groups []sreportalv1alpha1.FQDNGroupStatus, portalRef, namespace string) []domaindns.FQDNView {
	seen := make(map[string]*domaindns.FQDNView)

	for _, group := range groups {
		for _, fqdn := range group.FQDNs {
			key := fqdn.FQDN + "/" + fqdn.RecordType
			if existing, ok := seen[key]; ok {
				if !slices.Contains(existing.Groups, group.Name) {
					existing.Groups = append(existing.Groups, group.Name)
				}
				continue
			}
			view := domaindns.FQDNView{
				Name:        fqdn.FQDN,
				Source:      domaindns.Source(group.Source),
				Groups:      []string{group.Name},
				Description: fqdn.Description,
				RecordType:  fqdn.RecordType,
				Targets:     fqdn.Targets,
				LastSeen:    fqdn.LastSeen.Time,
				Portals:     []string{portalRef},
				Namespace:   namespace,
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

	views := make([]domaindns.FQDNView, 0, len(seen))
	for _, v := range seen {
		views = append(views, *v)
	}
	return views
}

// RemoteDNSName returns the name of the DNS CR for a remote portal.
func RemoteDNSName(portalName string) string {
	return fmt.Sprintf("remote-%s", portalName)
}
