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

// Package imageinventory contains the ImageInventory controller and its chain handlers.
package imageinventory

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	imageinventorychain "github.com/golgoth31/sreportal/internal/controller/imageinventory/chain"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ImageInventoryReconciler reconciles an ImageInventory object using a chain of handlers.
type ImageInventoryReconciler struct {
	client.Client
	chain *reconciler.Chain[*sreportalv1alpha1.ImageInventory, imageinventorychain.ChainData]
}

// NewImageInventoryReconciler creates a new ImageInventoryReconciler with the handler chain.
func NewImageInventoryReconciler(c client.Client) *ImageInventoryReconciler {
	chain := reconciler.NewChain(
		imageinventorychain.NewValidateSpecHandler(c),
		imageinventorychain.NewValidatePortalRefHandler(c),
		imageinventorychain.NewUpdateStatusHandler(c),
	)
	return &ImageInventoryReconciler{
		Client: c,
		chain:  chain,
	}
}

// +kubebuilder:rbac:groups=sreportal.io,resources=imageinventories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=imageinventories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=imageinventories/finalizers,verbs=update

// Reconcile validates an ImageInventory resource via the handler chain.
func (r *ImageInventoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	var inv sreportalv1alpha1.ImageInventory
	if err := r.Get(ctx, req.NamespacedName, &inv); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get image inventory CR: %w", err)
	}

	logger.V(1).Info("reconciling ImageInventory", "name", inv.Name, "portalRef", inv.Spec.PortalRef)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, imageinventorychain.ChainData]{
		Resource: &inv,
	}

	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation chain failed")
		metrics.ReconcileTotal.WithLabelValues("imageinventory", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("imageinventory").Observe(time.Since(start).Seconds())
		// Invalid spec / unknown portal are surfaced via status conditions; no requeue needed
		// since the chain will re-run when the resource (or the referenced portal) changes.
		if errors.Is(err, imageinventorychain.ErrInvalidSpec) || errors.Is(err, imageinventorychain.ErrPortalNotFound) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	metrics.ReconcileTotal.WithLabelValues("imageinventory", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("imageinventory").Observe(time.Since(start).Seconds())

	if rc.Result.RequeueAfter > 0 {
		return rc.Result, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the manager.
func (r *ImageInventoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.ImageInventory{}).
		Named("imageinventory").
		Complete(r)
}
