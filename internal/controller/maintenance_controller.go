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
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	maintenancectrl "github.com/golgoth31/sreportal/internal/controller/maintenancectrl"
	domainmaint "github.com/golgoth31/sreportal/internal/domain/maintenance"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// MaintenanceReconciler reconciles a Maintenance object using a chain of handlers.
type MaintenanceReconciler struct {
	client.Client
	chain             *reconciler.Chain[*sreportalv1alpha1.Maintenance, maintenancectrl.ChainData]
	maintenanceWriter domainmaint.MaintenanceWriter
}

// NewMaintenanceReconciler creates a new MaintenanceReconciler with the handler chain.
func NewMaintenanceReconciler(c client.Client, maintenanceWriter domainmaint.MaintenanceWriter) *MaintenanceReconciler {
	chain := reconciler.NewChain(
		maintenancectrl.NewComputePhaseHandler(),
		maintenancectrl.NewUpdateStatusHandler(c, maintenanceWriter),
	)

	return &MaintenanceReconciler{
		Client:            c,
		chain:             chain,
		maintenanceWriter: maintenanceWriter,
	}
}

// +kubebuilder:rbac:groups=sreportal.io,resources=maintenances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=maintenances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=maintenances/finalizers,verbs=update

// Reconcile computes the maintenance phase and projects to the ReadStore.
func (r *MaintenanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	var maint sreportalv1alpha1.Maintenance
	if err := r.Get(ctx, req.NamespacedName, &maint); err != nil {
		if apierrors.IsNotFound(err) {
			if r.maintenanceWriter != nil {
				_ = r.maintenanceWriter.Delete(ctx, req.Namespace+"/"+req.Name)
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get maintenance CR: %w", err)
	}

	logger.V(1).Info("reconciling Maintenance", "name", maint.Name, "portalRef", maint.Spec.PortalRef)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Maintenance, maintenancectrl.ChainData]{
		Resource: &maint,
	}

	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation chain failed")
		metrics.ReconcileTotal.WithLabelValues("maintenance", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("maintenance").Observe(time.Since(start).Seconds())
		return ctrl.Result{}, nil
	}

	metrics.ReconcileTotal.WithLabelValues("maintenance", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("maintenance").Observe(time.Since(start).Seconds())

	if rc.Result.RequeueAfter > 0 {
		return rc.Result, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the manager.
func (r *MaintenanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.Maintenance{}).
		Named("maintenance").
		Complete(r)
}
