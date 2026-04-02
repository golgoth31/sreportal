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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	dnsctrl "github.com/golgoth31/sreportal/internal/controller/dns"
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
	dns := &sreportalv1alpha1.DNS{}

	err := h.client.Get(ctx, types.NamespacedName{Name: dnsName, Namespace: portal.Namespace}, dns)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get DNS: %w", err)
	}

	isNew := errors.IsNotFound(err)
	if isNew {
		dns = &sreportalv1alpha1.DNS{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dnsName,
				Namespace: portal.Namespace,
			},
			Spec: sreportalv1alpha1.DNSSpec{
				PortalRef: portal.Name,
				IsRemote:  true,
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

	// Update status with fetched groups
	dnsBase := dns.DeepCopy()
	now := metav1.Now()
	dns.Status.Groups = result.Groups
	dns.Status.LastReconcileTime = &now

	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "RemoteSyncSuccess",
		Message:            fmt.Sprintf("Successfully synced %d FQDNs from remote portal", result.FQDNCount),
		LastTransitionTime: now,
	}
	meta.SetStatusCondition(&dns.Status.Conditions, readyCondition)

	if err := h.client.Status().Patch(ctx, dns, client.MergeFrom(dnsBase)); err != nil {
		return fmt.Errorf("patch DNS status: %w", err)
	}

	// Project remote FQDNs into FQDN read store
	if data.FQDNWriter != nil {
		resourceKey := dns.Namespace + "/" + dns.Name
		views := dnsctrl.GroupsToFQDNViews(dns)
		if err := data.FQDNWriter.Replace(ctx, resourceKey, views); err != nil {
			logger.Error(err, "failed to update FQDN read store for remote DNS")
		}
	}

	logger.Info("updated DNS status with remote FQDNs",
		"dns", dnsName,
		"portal", portal.Name,
		"fqdnCount", result.FQDNCount,
		"groupCount", len(result.Groups))

	return nil
}

// RemoteDNSName returns the name of the DNS CR for a remote portal.
func RemoteDNSName(portalName string) string {
	return fmt.Sprintf("remote-%s", portalName)
}
