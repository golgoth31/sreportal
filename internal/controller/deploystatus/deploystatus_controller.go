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

// Package deploystatus contains the reconciler for the DeployStatus CRD.
package deploystatus

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	deploystatuschain "github.com/golgoth31/sreportal/internal/controller/deploystatus/chain"
	domdeploystatus "github.com/golgoth31/sreportal/internal/domain/deploystatus"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// finalizerName is the finalizer added to every DeployStatus CR.
	finalizerName = "deploystatus.sreportal.io/cleanup"

	// portalRefField is the field index name used to look up DeployStatus CRs by portalRef.
	portalRefField = "spec.portalRef"

	// defaultRequeueInterval is the fallback periodic requeue cadence when the
	// configured refresh interval is zero.
	defaultRequeueInterval = 5 * time.Minute
)

// DeployStatusReconciler reconciles a DeployStatus object.
type DeployStatusReconciler struct {
	client.Client
	chain *reconciler.Chain[*sreportalv1alpha1.DeployStatus, deploystatuschain.ChainData]

	store           domdeploystatus.Writer
	refreshInterval time.Duration
}

// +kubebuilder:rbac:groups=sreportal.io,resources=deploystatuses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=deploystatuses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=deploystatuses/finalizers,verbs=update

// NewDeployStatusReconciler builds a DeployStatusReconciler wired with the
// handler chain.
func NewDeployStatusReconciler(
	c client.Client,
	store domdeploystatus.Writer,
	clientFor func(host string) forge.Client,
	cfg *config.DeployStatusConfig,
) *DeployStatusReconciler {
	refreshInterval := defaultRequeueInterval
	var forges []config.ForgeConfig
	if cfg != nil {
		if d := time.Duration(cfg.RefreshInterval); d > 0 {
			refreshInterval = d
		}
		forges = cfg.Forges
	}

	handlers := []reconciler.Handler[*sreportalv1alpha1.DeployStatus, deploystatuschain.ChainData]{
		deploystatuschain.NewSelectDueHandler(refreshInterval),
		deploystatuschain.NewResolveOCISourceHandler(forges),
		deploystatuschain.NewForgeCompareHandler(clientFor),
		deploystatuschain.NewResolveDeployRunHandler(clientFor, forges),
		deploystatuschain.NewUpdateReadStoreHandler(store),
		deploystatuschain.NewUpdateStatusHandler(c),
	}

	return &DeployStatusReconciler{
		Client:          c,
		chain:           reconciler.NewChain("deploystatus", handlers...),
		store:           store,
		refreshInterval: refreshInterval,
	}
}

// Reconcile is the main reconciliation loop entry point.
func (r *DeployStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var cr sreportalv1alpha1.DeployStatus
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion — run finalizer cleanup.
	if !cr.DeletionTimestamp.IsZero() {
		return r.handleFinalizer(ctx, &cr)
	}

	// Ensure finalizer is registered.
	if !controllerutil.ContainsFinalizer(&cr, finalizerName) {
		controllerutil.AddFinalizer(&cr, finalizerName)
		if err := r.Update(ctx, &cr); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		// Re-fetch after update.
		if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Remote shadow CR: entries are fetched from a remote portal's
	// DeployStatusService rather than computed locally. Skip the local compute
	// chain and just requeue periodically.
	// TODO(phase9): remote fetch
	if cr.Spec.IsRemote {
		logger.V(1).Info("skipping remote DeployStatus (federation not yet implemented)",
			"name", cr.Name, "portal", cr.Spec.PortalRef)
		return ctrl.Result{RequeueAfter: r.refreshInterval}, nil
	}

	logger.V(1).Info("reconciling DeployStatus",
		"name", cr.Name,
		"portal", cr.Spec.PortalRef,
		"namespace", cr.Spec.Namespace,
		"services", len(cr.Spec.Services),
	)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, deploystatuschain.ChainData]{
		Resource: &cr,
	}

	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation chain failed")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.refreshInterval}, nil
}

// handleFinalizer removes the readstore contribution for this CR, then removes
// the finalizer so Kubernetes can garbage-collect the CR.
func (r *DeployStatusReconciler) handleFinalizer(ctx context.Context, cr *sreportalv1alpha1.DeployStatus) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(cr, finalizerName) {
		// Drop this CR's readstore contribution. The in-memory store cannot fail,
		// but keeping the cleanup before finalizer removal keeps the contract
		// future-proof against a persistent store.
		r.store.RemoveForNamespace(cr.Spec.PortalRef, cr.Spec.Namespace)

		controllerutil.RemoveFinalizer(cr, finalizerName)
		if err := r.Update(ctx, cr); err != nil {
			return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller and installs the field indexer.
func (r *DeployStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Index DeployStatus by spec.portalRef so we can efficiently look up all
	// CRs belonging to a given portal.
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.DeployStatus{},
		portalRefField,
		func(obj client.Object) []string {
			ds, ok := obj.(*sreportalv1alpha1.DeployStatus)
			if !ok || ds.Spec.PortalRef == "" {
				return nil
			}
			return []string{ds.Spec.PortalRef}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.DeployStatus{}).
		Named("deploystatus").
		Complete(r)
}
