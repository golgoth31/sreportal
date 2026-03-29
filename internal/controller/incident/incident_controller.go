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

package incident

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	incidentchain "github.com/golgoth31/sreportal/internal/controller/incident/chain"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	domainincident "github.com/golgoth31/sreportal/internal/domain/incident"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// IncidentReconciler reconciles an Incident object using a chain of handlers.
type IncidentReconciler struct {
	client.Client
	chain          *reconciler.Chain[*sreportalv1alpha1.Incident, incidentchain.ChainData]
	incidentWriter domainincident.IncidentWriter
}

// NewIncidentReconciler creates a new IncidentReconciler with the handler chain.
func NewIncidentReconciler(c client.Client, incidentWriter domainincident.IncidentWriter) *IncidentReconciler {
	chain := reconciler.NewChain(
		incidentchain.NewComputeStatusHandler(),
		incidentchain.NewUpdateStatusHandler(c, incidentWriter),
	)

	return &IncidentReconciler{
		Client:         c,
		chain:          chain,
		incidentWriter: incidentWriter,
	}
}

// +kubebuilder:rbac:groups=sreportal.io,resources=incidents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=incidents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=incidents/finalizers,verbs=update
// +kubebuilder:rbac:groups=sreportal.io,resources=portals,verbs=get;list;watch

// Reconcile computes the incident phase and duration and projects to the ReadStore.
func (r *IncidentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	var incident sreportalv1alpha1.Incident
	if err := r.Get(ctx, req.NamespacedName, &incident); err != nil {
		if apierrors.IsNotFound(err) {
			if r.incidentWriter != nil {
				_ = r.incidentWriter.Delete(ctx, req.Namespace+"/"+req.Name)
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get incident CR: %w", err)
	}

	logger.V(1).Info("reconciling Incident", "name", incident.Name, "portalRef", incident.Spec.PortalRef)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Incident, incidentchain.ChainData]{
		Resource: &incident,
	}

	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation chain failed")
		metrics.ReconcileTotal.WithLabelValues("incident", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("incident").Observe(time.Since(start).Seconds())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	metrics.ReconcileTotal.WithLabelValues("incident", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("incident").Observe(time.Since(start).Seconds())

	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the manager.
func (r *IncidentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.Incident{}).
		Watches(
			&sreportalv1alpha1.Portal{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
				portal, ok := obj.(*sreportalv1alpha1.Portal)
				if !ok {
					return nil
				}
				return portalfeatures.IncidentReconcileRequestsForPortal(ctx, r.Client, portal)
			}),
			builder.WithPredicates(portalfeatures.PortalStatusPageEnabledWakeupPredicate()),
		).
		Named("incident").
		Complete(r)
}
