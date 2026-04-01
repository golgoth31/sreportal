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

package component

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlreconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	componentchain "github.com/golgoth31/sreportal/internal/controller/component/chain"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	domaincomponent "github.com/golgoth31/sreportal/internal/domain/component"
	domainmaint "github.com/golgoth31/sreportal/internal/domain/maintenance"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ComponentReconciler reconciles a Component object using a chain of handlers.
type ComponentReconciler struct {
	client.Client
	chain           *reconciler.Chain[*sreportalv1alpha1.Component, componentchain.ChainData]
	componentWriter domaincomponent.ComponentWriter
}

// NewComponentReconciler creates a new ComponentReconciler with the handler chain.
func NewComponentReconciler(
	c client.Client,
	maintenanceReader domainmaint.MaintenanceReader,
	componentWriter domaincomponent.ComponentWriter,
) *ComponentReconciler {
	now := time.Now
	chain := reconciler.NewChain(
		componentchain.NewValidatePortalRefHandler(c),
		componentchain.NewComputeStatusHandler(maintenanceReader, c),
		componentchain.NewMergeDailyStatusHandler(now),
		componentchain.NewUpdateStatusHandler(c, componentWriter, now),
	)

	return &ComponentReconciler{
		Client:          c,
		chain:           chain,
		componentWriter: componentWriter,
	}
}

// +kubebuilder:rbac:groups=sreportal.io,resources=components,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=components/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=components/finalizers,verbs=update
// +kubebuilder:rbac:groups=sreportal.io,resources=portals,verbs=get;list;watch

// Reconcile computes the component's effective status and projects to the ReadStore.
func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	var comp sreportalv1alpha1.Component
	if err := r.Get(ctx, req.NamespacedName, &comp); err != nil {
		if apierrors.IsNotFound(err) {
			if r.componentWriter != nil {
				_ = r.componentWriter.Delete(ctx, req.Namespace+"/"+req.Name)
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get component CR: %w", err)
	}

	logger.V(1).Info("reconciling Component", "name", comp.Name, "portalRef", comp.Spec.PortalRef)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Component, componentchain.ChainData]{
		Resource: &comp,
	}

	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation chain failed")
		metrics.ReconcileTotal.WithLabelValues("component", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("component").Observe(time.Since(start).Seconds())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	metrics.ReconcileTotal.WithLabelValues("component", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("component").Observe(time.Since(start).Seconds())

	if rc.Result.RequeueAfter > 0 {
		return rc.Result, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the manager.
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.Component{}).
		Watches(&sreportalv1alpha1.Maintenance{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []ctrlreconcile.Request {
				maint, ok := obj.(*sreportalv1alpha1.Maintenance)
				if !ok {
					return nil
				}
				var requests []ctrlreconcile.Request
				for _, compName := range maint.Spec.Components {
					requests = append(requests, ctrlreconcile.Request{
						NamespacedName: types.NamespacedName{
							Namespace: maint.Namespace,
							Name:      compName,
						},
					})
				}
				return requests
			},
		)).
		Watches(&sreportalv1alpha1.Incident{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []ctrlreconcile.Request {
				inc, ok := obj.(*sreportalv1alpha1.Incident)
				if !ok {
					return nil
				}
				var requests []ctrlreconcile.Request
				for _, compName := range inc.Spec.Components {
					requests = append(requests, ctrlreconcile.Request{
						NamespacedName: types.NamespacedName{
							Namespace: inc.Namespace,
							Name:      compName,
						},
					})
				}
				return requests
			},
		)).
		Watches(
			&sreportalv1alpha1.Portal{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
				portal, ok := obj.(*sreportalv1alpha1.Portal)
				if !ok {
					return nil
				}
				return portalfeatures.ComponentReconcileRequestsForPortal(ctx, r.Client, portal)
			}),
			builder.WithPredicates(portalfeatures.PortalStatusPageEnabledWakeupPredicate()),
		).
		Named("component").
		Complete(r)
}
