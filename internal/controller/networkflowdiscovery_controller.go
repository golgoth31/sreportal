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

package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	nfdchain "github.com/golgoth31/sreportal/internal/controller/networkflowdiscovery"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	networkFlowDiscoveryRequeueAfter = 1 * time.Minute
)

// NetworkFlowDiscoveryReconciler reconciles a NetworkFlowDiscovery object using a chain of handlers.
type NetworkFlowDiscoveryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	chain  *reconciler.Chain[*sreportalv1alpha1.NetworkFlowDiscovery, nfdchain.ChainData]
}

// NewNetworkFlowDiscoveryReconciler creates a new reconciler with the handler chain.
func NewNetworkFlowDiscoveryReconciler(c client.Client, scheme *runtime.Scheme) *NetworkFlowDiscoveryReconciler {
	handlers := []reconciler.Handler[*sreportalv1alpha1.NetworkFlowDiscovery, nfdchain.ChainData]{
		nfdchain.NewBuildGraphHandler(c),
		nfdchain.NewUpdateStatusHandler(c),
	}

	return &NetworkFlowDiscoveryReconciler{
		Client: c,
		Scheme: scheme,
		chain:  reconciler.NewChain(handlers...),
	}
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

// Reconcile builds the network flow graph and updates the resource status.
func (r *NetworkFlowDiscoveryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	var resource sreportalv1alpha1.NetworkFlowDiscovery
	if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(1).Info("reconciling NetworkFlowDiscovery", "name", resource.Name, "portalRef", resource.Spec.PortalRef)

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
		Named("networkflowdiscovery").
		Complete(r)
}
