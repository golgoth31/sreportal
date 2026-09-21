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
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// SyncRemoteDeployStatusHandler creates or updates a single shadow
// DeployStatus CR (`remote-<portal>`) with IsRemote=true for each remote
// portal, so the DeployStatus controller can fetch the remote portal's deploy
// status data and project it into the local readstore.
// No-op for local portals or when the deployStatus feature is disabled.
type SyncRemoteDeployStatusHandler struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewSyncRemoteDeployStatusHandler creates a new SyncRemoteDeployStatusHandler.
func NewSyncRemoteDeployStatusHandler(c client.Client, scheme *runtime.Scheme) *SyncRemoteDeployStatusHandler {
	return &SyncRemoteDeployStatusHandler{client: c, scheme: scheme}
}

// Handle implements reconciler.Handler.
func (h *SyncRemoteDeployStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	portal := rc.Resource
	if portal.Spec.Remote == nil {
		return nil
	}

	remoteLog := log.Default().WithName("portal").WithName("remote")

	if !portal.Spec.Features.IsDeployStatusEnabled() {
		remoteLog.V(1).Info("deployStatus feature disabled, skipping remote deploy status sync", "portal", portal.Name)
		return nil
	}

	if err := h.reconcileRemoteDeployStatus(ctx, portal); err != nil {
		remoteLog.Warn("failed to reconcile DeployStatus for remote portal", "name", portal.Name, "namespace", portal.Namespace, "error", err.Error())
		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "DeployStatusSynced",
			Status:             metav1.ConditionFalse,
			Reason:             "DeployStatusSyncFailed",
			Message:            fmt.Sprintf("Failed to sync DeployStatus resources for remote portal: %v", err),
			LastTransitionTime: metav1.Now(),
		})
	} else {
		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "DeployStatusSynced",
			Status:             metav1.ConditionTrue,
			Reason:             "DeployStatusSyncSuccess",
			Message:            "DeployStatus resources synced for remote portal",
			LastTransitionTime: metav1.Now(),
		})
	}

	return nil
}

func (h *SyncRemoteDeployStatusHandler) reconcileRemoteDeployStatus(ctx context.Context, portal *sreportalv1alpha1.Portal) error {
	logger := log.FromContext(ctx)

	dsName := RemoteDeployStatusName(portal.Name)
	ds := &sreportalv1alpha1.DeployStatus{}

	err := h.client.Get(ctx, types.NamespacedName{Name: dsName, Namespace: portal.Namespace}, ds)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get DeployStatus: %w", err)
	}

	isNew := errors.IsNotFound(err)
	if isNew {
		ds = &sreportalv1alpha1.DeployStatus{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dsName,
				Namespace: portal.Namespace,
			},
			Spec: sreportalv1alpha1.DeployStatusSpec{
				PortalRef: portal.Name,
				Namespace: RemoteDeployStatusNamespace(portal.Name),
				IsRemote:  true,
			},
		}
	}

	if err := controllerutil.SetControllerReference(portal, ds, h.scheme); err != nil {
		return fmt.Errorf("set controller reference: %w", err)
	}

	if isNew {
		if err := h.client.Create(ctx, ds); err != nil {
			return fmt.Errorf("create DeployStatus: %w", err)
		}
		logger.Info("created DeployStatus CR for remote portal", "deployStatus", dsName)
	} else {
		ds.Spec.PortalRef = portal.Name
		ds.Spec.Namespace = RemoteDeployStatusNamespace(portal.Name)
		ds.Spec.IsRemote = true

		if err := h.client.Update(ctx, ds); err != nil {
			return fmt.Errorf("update DeployStatus: %w", err)
		}
		logger.V(1).Info("updated DeployStatus CR for remote portal", "deployStatus", dsName)
	}

	return nil
}

// RemoteDeployStatusName returns the name of the DeployStatus CR used to
// shadow a remote portal's deploy status.
func RemoteDeployStatusName(portalName string) string {
	return fmt.Sprintf("remote-%s", portalName)
}

// RemoteDeployStatusNamespace returns the sentinel readstore namespace bucket
// under which a remote portal's aggregated deploy-status entries are stored.
//
// Unlike the local path (one CR per observed namespace), the shadow CR
// aggregates all of a remote portal's namespaces into a single bucket. This
// keeps the readstore contribution under one (portalRef, namespace) scope so
// the DeployStatus finalizer's single RemoveForNamespace call cleans it up.
func RemoteDeployStatusNamespace(portalName string) string {
	return fmt.Sprintf("remote-%s", portalName)
}
