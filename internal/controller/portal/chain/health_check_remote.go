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

// HealthCheckRemoteHandler performs a health check on the remote portal.
// No-op for local portals.
type HealthCheckRemoteHandler struct {
	client client.Client
}

// NewHealthCheckRemoteHandler creates a new HealthCheckRemoteHandler.
func NewHealthCheckRemoteHandler(c client.Client) *HealthCheckRemoteHandler {
	return &HealthCheckRemoteHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *HealthCheckRemoteHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	portal := rc.Resource
	if portal.Spec.Remote == nil {
		return nil
	}

	remoteLog := log.Default().WithName("portal").WithName("remote")
	remote := portal.Spec.Remote

	err := rc.Data.RemoteClient.HealthCheck(ctx, remote.URL)
	if err != nil {
		metrics.PortalRemoteSyncErrorsTotal.WithLabelValues(portal.Name).Inc()
		remoteLog.Error(err, "remote portal health check failed", "name", portal.Name, "namespace", portal.Namespace, "url", remote.URL, "error", err.Error())

		base := portal.DeepCopy()
		portal.Status.Ready = false
		if portal.Status.RemoteSync == nil {
			portal.Status.RemoteSync = &sreportalv1alpha1.RemoteSyncStatus{}
		}
		portal.Status.RemoteSync.LastSyncError = err.Error()

		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               conditionTypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             "RemoteConnectionFailed",
			Message:            "Failed to connect to remote portal: " + err.Error(),
			LastTransitionTime: metav1.Now(),
		})

		if patchErr := h.client.Status().Patch(ctx, portal, client.MergeFrom(base)); patchErr != nil {
			return fmt.Errorf("patch Portal status: %w", patchErr)
		}

		rc.Result = ctrl.Result{RequeueAfter: DefaultRemoteSyncInterval}
		return nil
	}

	return nil
}
