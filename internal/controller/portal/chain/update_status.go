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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// UpdateStatusHandler updates the portal status with Ready condition.
// Handles both local portals (simple ready) and remote portals (sync status + requeue).
type UpdateStatusHandler struct {
	client client.Client
}

// NewUpdateStatusHandler creates a new UpdateStatusHandler.
func NewUpdateStatusHandler(c client.Client) *UpdateStatusHandler {
	return &UpdateStatusHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	portal := rc.Resource

	if portal.Spec.Remote != nil {
		return h.handleRemote(ctx, rc)
	}
	return h.handleLocal(ctx, portal)
}

func (h *UpdateStatusHandler) handleLocal(ctx context.Context, portal *sreportalv1alpha1.Portal) error {
	logger := log.FromContext(ctx).WithName("update-status")

	base := portal.DeepCopy()

	portal.Status.Ready = true
	portal.Status.RemoteSync = nil

	meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "PortalConfigured",
		Message:            "Portal is fully configured",
		LastTransitionTime: metav1.Now(),
	})

	if err := h.client.Status().Patch(ctx, portal, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch Portal status: %w", err)
	}

	logger.V(1).Info("updated local portal status")
	return nil
}

func (h *UpdateStatusHandler) handleRemote(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	portal := rc.Resource
	result := rc.Data.FetchResult
	remoteLog := log.Default().WithName("portal").WithName("remote")

	base := portal.DeepCopy()
	now := metav1.Now()

	portal.Status.Ready = true
	if portal.Status.RemoteSync == nil {
		portal.Status.RemoteSync = &sreportalv1alpha1.RemoteSyncStatus{}
	}
	portal.Status.RemoteSync.LastSyncTime = &now
	portal.Status.RemoteSync.LastSyncError = ""
	portal.Status.RemoteSync.RemoteTitle = result.RemoteTitle
	portal.Status.RemoteSync.FQDNCount = result.FQDNCount
	portal.Status.RemoteSync.Features = result.RemoteFeatures

	meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "RemoteSyncSuccess",
		Message:            "Successfully synced with remote portal",
		LastTransitionTime: metav1.Now(),
	})

	if err := h.client.Status().Patch(ctx, portal, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch Portal status: %w", err)
	}

	metrics.PortalRemoteFQDNsSynced.WithLabelValues(portal.Name).Set(float64(result.FQDNCount))

	remoteLog.Info("remote portal sync successful",
		"url", portal.Spec.Remote.URL,
		"remotePortal", portal.Spec.Remote.Portal,
		"fqdnCount", result.FQDNCount,
		"groupCount", len(result.Groups),
		"remoteTitle", result.RemoteTitle)

	rc.Result = ctrl.Result{RequeueAfter: DefaultRemoteSyncInterval}
	return nil
}
