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

package networkflowdiscovery

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"k8s.io/apimachinery/pkg/api/errors"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	nfdchain "github.com/golgoth31/sreportal/internal/controller/networkflowdiscovery/chain"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol" // used by SetFlowGraphWriter
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
)

const (
	networkFlowDiscoveryRequeueAfter = 1 * time.Minute
)

// NetworkFlowDiscoveryReconciler reconciles a NetworkFlowDiscovery object using a chain of handlers.
type NetworkFlowDiscoveryReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	chain           *reconciler.Chain[*sreportalv1alpha1.NetworkFlowDiscovery, nfdchain.ChainData]
	updateStatus    *nfdchain.UpdateStatusHandler
	flowGraphWriter domainnetpol.FlowGraphWriter
}

// NewNetworkFlowDiscoveryReconciler creates a new reconciler with the handler chain.
func NewNetworkFlowDiscoveryReconciler(c client.Client, scheme *runtime.Scheme, remoteClientCache *remoteclient.Cache) *NetworkFlowDiscoveryReconciler {
	updateStatus := nfdchain.NewUpdateStatusHandler(c)
	handlers := []reconciler.Handler[*sreportalv1alpha1.NetworkFlowDiscovery, nfdchain.ChainData]{
		nfdchain.NewFetchRemoteGraphHandler(c, remoteClientCache),
		nfdchain.NewBuildGraphHandler(c),
		nfdchain.NewObserveFlowsHandler(c),
		updateStatus,
	}

	r := &NetworkFlowDiscoveryReconciler{
		Client:       c,
		Scheme:       scheme,
		chain:        reconciler.NewChain(handlers...),
		updateStatus: updateStatus,
	}

	return r
}

// SetFlowGraphWriter sets the writer used to push graph data to the in-memory read store.
func (r *NetworkFlowDiscoveryReconciler) SetFlowGraphWriter(w domainnetpol.FlowGraphWriter) {
	r.flowGraphWriter = w
	r.updateStatus.SetFlowGraphWriter(w)
}

// +kubebuilder:rbac:groups=sreportal.io,resources=networkflowdiscoveries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=networkflowdiscoveries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=networkflowdiscoveries/finalizers,verbs=update
// +kubebuilder:rbac:groups=sreportal.io,resources=flownodesets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=flownodesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=flowedgesets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=flowedgesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.gke.io,resources=fqdnnetworkpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=sreportal.io,resources=portals,verbs=get;list;watch

// Reconcile builds the network flow graph and updates the resource status.
func (r *NetworkFlowDiscoveryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	var resource sreportalv1alpha1.NetworkFlowDiscovery
	if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
		if errors.IsNotFound(err) && r.flowGraphWriter != nil {
			if delErr := r.flowGraphWriter.Delete(ctx, req.Name); delErr != nil {
				logger.Error(delErr, "failed to delete flow graph from read store")
			}
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(1).Info("reconciling NetworkFlowDiscovery", "name", resource.Name, "portalRef", resource.Spec.PortalRef)

	// Skip reconciliation when networkPolicy feature is disabled on the referenced portal.
	// Purge stale data from the read store so the gRPC/MCP layer stops serving it.
	if resource.Spec.PortalRef != "" {
		var portal sreportalv1alpha1.Portal
		if err := r.Get(ctx, client.ObjectKey{Name: resource.Spec.PortalRef, Namespace: resource.Namespace}, &portal); err == nil {
			if !portal.Spec.Features.IsNetworkPolicyEnabled() {
				logger.V(1).Info("networkPolicy feature disabled for portal, skipping", "portal", resource.Spec.PortalRef)
				if r.flowGraphWriter != nil {
					if delErr := r.flowGraphWriter.Delete(ctx, req.Name); delErr != nil {
						logger.Error(delErr, "failed to delete flow graph from read store")
					}
				}
				return ctrl.Result{}, nil
			}
		}
	}

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, nfdchain.ChainData]{
		Resource: &resource,
	}

	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation chain failed")
		metrics.ReconcileTotal.WithLabelValues("networkflowdiscovery", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("networkflowdiscovery").Observe(time.Since(start).Seconds())

		return ctrl.Result{RequeueAfter: networkFlowDiscoveryRequeueAfter}, nil
	}

	metrics.ReconcileTotal.WithLabelValues("networkflowdiscovery", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("networkflowdiscovery").Observe(time.Since(start).Seconds())

	if rc.Result.RequeueAfter > 0 {
		return rc.Result, nil
	}

	return ctrl.Result{RequeueAfter: networkFlowDiscoveryRequeueAfter}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkFlowDiscoveryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.NetworkFlowDiscovery{}).
		Watches(
			&sreportalv1alpha1.Portal{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
				portal, ok := obj.(*sreportalv1alpha1.Portal)
				if !ok {
					return nil
				}
				return portalfeatures.NetworkFlowDiscoveryReconcileRequestsForPortal(ctx, r.Client, portal)
			}),
			builder.WithPredicates(portalfeatures.PortalNetworkPolicyEnabledWakeupPredicate()),
		).
		Named("networkflowdiscovery").
		Complete(r)
}
