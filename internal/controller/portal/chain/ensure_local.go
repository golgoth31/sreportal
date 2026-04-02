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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// EnsureLocalResourcesHandler creates child resources for local portals
// when features are enabled (e.g. NetworkFlowDiscovery for networkPolicy).
type EnsureLocalResourcesHandler struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewEnsureLocalResourcesHandler creates a new EnsureLocalResourcesHandler.
func NewEnsureLocalResourcesHandler(c client.Client, scheme *runtime.Scheme) *EnsureLocalResourcesHandler {
	return &EnsureLocalResourcesHandler{client: c, scheme: scheme}
}

// Handle implements reconciler.Handler.
func (h *EnsureLocalResourcesHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	portal := rc.Resource

	// Only applies to local portals with networkPolicy enabled
	if portal.Spec.Remote != nil || !portal.Spec.Features.IsNetworkPolicyEnabled() {
		return nil
	}

	return h.ensureNetworkFlowDiscovery(ctx, portal)
}

// ensureNetworkFlowDiscovery creates a NetworkFlowDiscovery resource for the portal
// if one does not already exist.
func (h *EnsureLocalResourcesHandler) ensureNetworkFlowDiscovery(ctx context.Context, portal *sreportalv1alpha1.Portal) error {
	logger := log.FromContext(ctx).WithName("ensure-local")

	nfdName := LocalNFDName(portal.Name)
	var existing sreportalv1alpha1.NetworkFlowDiscovery
	err := h.client.Get(ctx, types.NamespacedName{Name: nfdName, Namespace: portal.Namespace}, &existing)
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("get NetworkFlowDiscovery: %w", err)
	}

	nfd := &sreportalv1alpha1.NetworkFlowDiscovery{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nfdName,
			Namespace: portal.Namespace,
		},
		Spec: sreportalv1alpha1.NetworkFlowDiscoverySpec{
			PortalRef: portal.Name,
		},
	}
	if err := controllerutil.SetControllerReference(portal, nfd, h.scheme); err != nil {
		return fmt.Errorf("set controller reference: %w", err)
	}
	if err := h.client.Create(ctx, nfd); err != nil {
		return fmt.Errorf("create NetworkFlowDiscovery: %w", err)
	}

	logger.Info("created NetworkFlowDiscovery (networkPolicy feature re-enabled)", "name", nfdName)
	return nil
}

// LocalNFDName returns the name of the NetworkFlowDiscovery CR for a local portal.
func LocalNFDName(portalName string) string {
	return fmt.Sprintf("netflow-%s", portalName)
}
