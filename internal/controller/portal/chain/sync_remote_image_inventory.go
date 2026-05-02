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

// SyncRemoteImageInventoryHandler creates or updates a single shadow
// ImageInventory CR (`remote-<portal>`) with IsRemote=true for each remote
// portal, so the ImageInventory controller can fetch the remote portal's
// image data and project it into the local readstore.
// No-op for local portals or when the imageInventory feature is disabled.
type SyncRemoteImageInventoryHandler struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewSyncRemoteImageInventoryHandler creates a new SyncRemoteImageInventoryHandler.
func NewSyncRemoteImageInventoryHandler(c client.Client, scheme *runtime.Scheme) *SyncRemoteImageInventoryHandler {
	return &SyncRemoteImageInventoryHandler{client: c, scheme: scheme}
}

// Handle implements reconciler.Handler.
func (h *SyncRemoteImageInventoryHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	portal := rc.Resource
	if portal.Spec.Remote == nil {
		return nil
	}

	remoteLog := log.Default().WithName("portal").WithName("remote")

	if !portal.Spec.Features.IsImageInventoryEnabled() {
		remoteLog.V(1).Info("imageInventory feature disabled, skipping remote image inventory sync", "portal", portal.Name)
		return nil
	}

	if err := h.reconcileRemoteImageInventory(ctx, portal); err != nil {
		remoteLog.Warn("failed to reconcile ImageInventory for remote portal", "name", portal.Name, "namespace", portal.Namespace, "error", err.Error())
		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "ImageInventorySynced",
			Status:             metav1.ConditionFalse,
			Reason:             "ImageInventorySyncFailed",
			Message:            fmt.Sprintf("Failed to sync ImageInventory resources for remote portal: %v", err),
			LastTransitionTime: metav1.Now(),
		})
	} else {
		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "ImageInventorySynced",
			Status:             metav1.ConditionTrue,
			Reason:             "ImageInventorySyncSuccess",
			Message:            "ImageInventory resources synced for remote portal",
			LastTransitionTime: metav1.Now(),
		})
	}

	return nil
}

func (h *SyncRemoteImageInventoryHandler) reconcileRemoteImageInventory(ctx context.Context, portal *sreportalv1alpha1.Portal) error {
	logger := log.FromContext(ctx)

	invName := RemoteImageInventoryName(portal.Name)
	inv := &sreportalv1alpha1.ImageInventory{}

	err := h.client.Get(ctx, types.NamespacedName{Name: invName, Namespace: portal.Namespace}, inv)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get ImageInventory: %w", err)
	}

	isNew := errors.IsNotFound(err)
	if isNew {
		inv = &sreportalv1alpha1.ImageInventory{
			ObjectMeta: metav1.ObjectMeta{
				Name:      invName,
				Namespace: portal.Namespace,
			},
			Spec: sreportalv1alpha1.ImageInventorySpec{
				PortalRef: portal.Name,
				IsRemote:  true,
				RemoteURL: portal.Spec.Remote.URL,
			},
		}
	}

	if err := controllerutil.SetControllerReference(portal, inv, h.scheme); err != nil {
		return fmt.Errorf("set controller reference: %w", err)
	}

	if isNew {
		if err := h.client.Create(ctx, inv); err != nil {
			return fmt.Errorf("create ImageInventory: %w", err)
		}
		logger.Info("created ImageInventory CR for remote portal", "imageInventory", invName)
		return nil
	}

	inv.Spec.PortalRef = portal.Name
	inv.Spec.IsRemote = true
	inv.Spec.RemoteURL = portal.Spec.Remote.URL

	if err := h.client.Update(ctx, inv); err != nil {
		return fmt.Errorf("update ImageInventory: %w", err)
	}
	logger.V(1).Info("updated ImageInventory CR for remote portal", "imageInventory", invName)
	return nil
}

// RemoteImageInventoryName returns the name of the ImageInventory CR used to
// shadow a remote portal's image inventory.
func RemoteImageInventoryName(portalName string) string {
	return fmt.Sprintf("remote-%s", portalName)
}
