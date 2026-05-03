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

// FetchRemoteDataHandler fetches FQDNs and portal info from the remote portal.
// No-op for local portals.
type FetchRemoteDataHandler struct {
	client client.Client
}

// NewFetchRemoteDataHandler creates a new FetchRemoteDataHandler.
func NewFetchRemoteDataHandler(c client.Client) *FetchRemoteDataHandler {
	return &FetchRemoteDataHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *FetchRemoteDataHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	portal := rc.Resource
	if portal.Spec.Remote == nil {
		return nil
	}

	remoteLog := log.Default().WithName("portal").WithName("remote")
	remote := portal.Spec.Remote

	result, err := rc.Data.RemoteClient.FetchFQDNs(ctx, remote.URL, remote.Portal)
	if err != nil {
		metrics.PortalRemoteSyncErrorsTotal.WithLabelValues(portal.Name).Inc()
		remoteLog.Warn("failed to fetch FQDNs from remote portal", "name", portal.Name, "namespace", portal.Namespace, "url", remote.URL, "remotePortal", remote.Portal, "error", err.Error())

		base := portal.DeepCopy()
		portal.Status.Ready = false
		if portal.Status.RemoteSync == nil {
			portal.Status.RemoteSync = &sreportalv1alpha1.RemoteSyncStatus{}
		}
		portal.Status.RemoteSync.LastSyncError = err.Error()

		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               conditionTypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             "RemoteFetchFailed",
			Message:            "Failed to fetch data from remote portal: " + err.Error(),
			LastTransitionTime: metav1.Now(),
		})

		if patchErr := h.client.Status().Patch(ctx, portal, client.MergeFrom(base)); patchErr != nil {
			return fmt.Errorf("patch Portal status: %w", patchErr)
		}

		rc.Result = ctrl.Result{RequeueAfter: DefaultRemoteSyncInterval}
		return nil
	}

	rc.Data.FetchResult = result
	return nil
}
