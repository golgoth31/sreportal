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
	alertmanagerctrl "github.com/golgoth31/sreportal/internal/controller/alertmanager/chain"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
)

// SyncRemoteAlertmanagerHandler discovers alertmanagers on the remote portal and creates
// one local Alertmanager CR per remote alertmanager.
// No-op for local portals or when alerts feature is disabled.
type SyncRemoteAlertmanagerHandler struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewSyncRemoteAlertmanagerHandler creates a new SyncRemoteAlertmanagerHandler.
func NewSyncRemoteAlertmanagerHandler(c client.Client, scheme *runtime.Scheme) *SyncRemoteAlertmanagerHandler {
	return &SyncRemoteAlertmanagerHandler{client: c, scheme: scheme}
}

// Handle implements reconciler.Handler.
func (h *SyncRemoteAlertmanagerHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	portal := rc.Resource
	if portal.Spec.Remote == nil {
		return nil
	}

	remoteLog := log.Default().WithName("portal").WithName("remote")

	if !portal.Spec.Features.IsAlertsEnabled() {
		remoteLog.V(1).Info("alerts feature disabled, skipping remote alertmanager sync", "portal", portal.Name)
		return nil
	}

	if err := h.reconcileRemoteAlertmanager(ctx, portal, rc.Data.RemoteClient); err != nil {
		remoteLog.Error(err, "failed to reconcile Alertmanager for remote portal")
		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "AlertsSynced",
			Status:             metav1.ConditionFalse,
			Reason:             "AlertsSyncFailed",
			Message:            fmt.Sprintf("Failed to sync Alertmanager resources for remote portal: %v", err),
			LastTransitionTime: metav1.Now(),
		})
	} else {
		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "AlertsSynced",
			Status:             metav1.ConditionTrue,
			Reason:             "AlertsSyncSuccess",
			Message:            "Alertmanager resources synced for remote portal",
			LastTransitionTime: metav1.Now(),
		})
	}

	return nil
}

func (h *SyncRemoteAlertmanagerHandler) reconcileRemoteAlertmanager(ctx context.Context, portal *sreportalv1alpha1.Portal, remoteClient *remoteclient.Client) error {
	logger := log.FromContext(ctx)

	remoteAMs, err := remoteClient.DiscoverAlertmanagers(ctx, portal.Spec.Remote.URL, portal.Spec.Remote.Portal)
	if err != nil {
		return fmt.Errorf("discover remote alertmanagers: %w", err)
	}

	logger.V(1).Info("discovered remote alertmanagers", "count", len(remoteAMs))

	expectedNames := make(map[string]struct{}, len(remoteAMs))

	for _, remoteAM := range remoteAMs {
		amName := RemoteAlertmanagerName(portal.Name, remoteAM.Name)
		expectedNames[amName] = struct{}{}

		if err := h.ensureAlertmanagerCR(ctx, portal, amName, remoteAM); err != nil {
			return fmt.Errorf("ensure alertmanager CR %s: %w", amName, err)
		}
	}

	if err := h.cleanupOrphanedAlertmanagers(ctx, portal, expectedNames); err != nil {
		return fmt.Errorf("cleanup orphaned alertmanagers: %w", err)
	}

	return nil
}

func (h *SyncRemoteAlertmanagerHandler) ensureAlertmanagerCR(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	amName string,
	remoteAM remoteclient.RemoteAlertmanagerInfo,
) error {
	logger := log.FromContext(ctx)

	am := &sreportalv1alpha1.Alertmanager{}
	err := h.client.Get(ctx, types.NamespacedName{Name: amName, Namespace: portal.Namespace}, am)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get Alertmanager: %w", err)
	}

	isNew := errors.IsNotFound(err)
	if isNew {
		am = &sreportalv1alpha1.Alertmanager{
			ObjectMeta: metav1.ObjectMeta{
				Name:      amName,
				Namespace: portal.Namespace,
				Labels: map[string]string{
					alertmanagerctrl.LabelRemoteAlertmanagerName: remoteAM.Name,
				},
			},
			Spec: sreportalv1alpha1.AlertmanagerSpec{
				PortalRef: portal.Name,
				URL: sreportalv1alpha1.AlertmanagerURL{
					Local:  portal.Spec.Remote.URL,
					Remote: remoteAM.RemoteURL,
				},
				IsRemote: true,
			},
		}
	}

	if err := controllerutil.SetControllerReference(portal, am, h.scheme); err != nil {
		return fmt.Errorf("set controller reference: %w", err)
	}

	if isNew {
		if err := h.client.Create(ctx, am); err != nil {
			return fmt.Errorf("create Alertmanager: %w", err)
		}
		logger.Info("created Alertmanager CR for remote alertmanager", "alertmanager", amName, "remoteAM", remoteAM.Name)
		return nil
	}

	am.Spec.PortalRef = portal.Name
	am.Spec.URL.Local = portal.Spec.Remote.URL
	am.Spec.URL.Remote = remoteAM.RemoteURL
	am.Spec.IsRemote = true

	if am.Labels == nil {
		am.Labels = make(map[string]string)
	}
	am.Labels[alertmanagerctrl.LabelRemoteAlertmanagerName] = remoteAM.Name

	if err := h.client.Update(ctx, am); err != nil {
		return fmt.Errorf("update Alertmanager: %w", err)
	}
	logger.V(1).Info("updated Alertmanager CR for remote alertmanager", "alertmanager", amName, "remoteAM", remoteAM.Name)

	return nil
}

func (h *SyncRemoteAlertmanagerHandler) cleanupOrphanedAlertmanagers(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	expectedNames map[string]struct{},
) error {
	logger := log.FromContext(ctx)

	var amList sreportalv1alpha1.AlertmanagerList
	if err := h.client.List(ctx, &amList, client.InNamespace(portal.Namespace)); err != nil {
		return fmt.Errorf("list alertmanagers: %w", err)
	}

	for i := range amList.Items {
		am := &amList.Items[i]
		if !am.Spec.IsRemote || am.Spec.PortalRef != portal.Name {
			continue
		}

		if _, expected := expectedNames[am.Name]; expected {
			continue
		}

		owned := false
		for _, ref := range am.OwnerReferences {
			if ref.UID == portal.UID {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}

		logger.Info("deleting orphaned Alertmanager CR", "alertmanager", am.Name)
		if err := h.client.Delete(ctx, am); err != nil {
			return fmt.Errorf("delete orphaned Alertmanager %s: %w", am.Name, err)
		}
	}

	return nil
}

// RemoteAlertmanagerName returns the name of the local Alertmanager CR
// that mirrors a specific remote alertmanager.
func RemoteAlertmanagerName(portalName, remoteAMName string) string {
	return fmt.Sprintf("remote-%s-%s", portalName, remoteAMName)
}
