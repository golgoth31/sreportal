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

// SyncRemoteNetworkFlowsHandler creates or updates a NetworkFlowDiscovery CR with
// isRemote=true for remote portals.
// No-op for local portals or when networkPolicy feature is disabled.
type SyncRemoteNetworkFlowsHandler struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewSyncRemoteNetworkFlowsHandler creates a new SyncRemoteNetworkFlowsHandler.
func NewSyncRemoteNetworkFlowsHandler(c client.Client, scheme *runtime.Scheme) *SyncRemoteNetworkFlowsHandler {
	return &SyncRemoteNetworkFlowsHandler{client: c, scheme: scheme}
}

// Handle implements reconciler.Handler.
func (h *SyncRemoteNetworkFlowsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	portal := rc.Resource
	if portal.Spec.Remote == nil {
		return nil
	}

	remoteLog := log.Default().WithName("portal").WithName("remote")

	if !portal.Spec.Features.IsNetworkPolicyEnabled() {
		remoteLog.V(1).Info("networkPolicy feature disabled, skipping remote network flows sync", "portal", portal.Name)
		return nil
	}

	if err := h.reconcileRemoteNetworkFlows(ctx, portal); err != nil {
		remoteLog.Warn("failed to reconcile network flows for remote portal", "name", portal.Name, "namespace", portal.Namespace, "error", err.Error())
		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "NetworkFlowsSynced",
			Status:             metav1.ConditionFalse,
			Reason:             "NetworkFlowsSyncFailed",
			Message:            fmt.Sprintf("Failed to sync network flows from remote portal: %v", err),
			LastTransitionTime: metav1.Now(),
		})
	} else {
		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "NetworkFlowsSynced",
			Status:             metav1.ConditionTrue,
			Reason:             "NetworkFlowsSyncSuccess",
			Message:            "Network flows synced for remote portal",
			LastTransitionTime: metav1.Now(),
		})
	}

	return nil
}

func (h *SyncRemoteNetworkFlowsHandler) reconcileRemoteNetworkFlows(ctx context.Context, portal *sreportalv1alpha1.Portal) error {
	logger := log.FromContext(ctx)

	nfdName := RemoteNFDName(portal.Name)
	nfd := &sreportalv1alpha1.NetworkFlowDiscovery{}

	err := h.client.Get(ctx, types.NamespacedName{Name: nfdName, Namespace: portal.Namespace}, nfd)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get NetworkFlowDiscovery: %w", err)
	}

	isNew := errors.IsNotFound(err)
	if isNew {
		nfd = &sreportalv1alpha1.NetworkFlowDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nfdName,
				Namespace: portal.Namespace,
			},
			Spec: sreportalv1alpha1.NetworkFlowDiscoverySpec{
				PortalRef: portal.Name,
				IsRemote:  true,
				RemoteURL: portal.Spec.Remote.URL,
			},
		}
	}

	if err := controllerutil.SetControllerReference(portal, nfd, h.scheme); err != nil {
		return fmt.Errorf("set controller reference: %w", err)
	}

	if isNew {
		if err := h.client.Create(ctx, nfd); err != nil {
			return fmt.Errorf("create NetworkFlowDiscovery: %w", err)
		}
		logger.Info("created NetworkFlowDiscovery CR for remote portal", "nfd", nfdName)
	} else {
		nfd.Spec.PortalRef = portal.Name
		nfd.Spec.IsRemote = true
		nfd.Spec.RemoteURL = portal.Spec.Remote.URL
		if err := h.client.Update(ctx, nfd); err != nil {
			return fmt.Errorf("update NetworkFlowDiscovery: %w", err)
		}
	}

	return nil
}

// RemoteNFDName returns the name of the NetworkFlowDiscovery CR for a remote portal.
func RemoteNFDName(portalName string) string {
	return fmt.Sprintf("remote-%s", portalName)
}
