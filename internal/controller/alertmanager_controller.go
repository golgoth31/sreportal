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
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	alertmanagerchain "github.com/golgoth31/sreportal/internal/controller/alertmanager"
	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanager"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	alertmanagerRequeueAfter = 2 * time.Minute
)

// AlertmanagerReconciler reconciles an Alertmanager object using a chain of handlers.
type AlertmanagerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	chain  *reconciler.Chain[*sreportalv1alpha1.Alertmanager]
}

// NewAlertmanagerReconciler creates a new AlertmanagerReconciler with the handler chain.
// localDataFetcher may be nil; then localFetcher is used for basic alerts only.
// The K8s client is used by the FetchAlertsHandler to look up Portal CRs and read
// TLS secrets when fetching alerts from remote portals.
func NewAlertmanagerReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	localDataFetcher domainalertmanager.DataFetcher,
	localFetcher domainalertmanager.Fetcher,
) *AlertmanagerReconciler {
	handlers := []reconciler.Handler[*sreportalv1alpha1.Alertmanager]{
		alertmanagerchain.NewFetchAlertsHandler(localDataFetcher, localFetcher, c),
		alertmanagerchain.NewUpdateStatusHandler(c),
	}

	return &AlertmanagerReconciler{
		Client: c,
		Scheme: scheme,
		chain:  reconciler.NewChain(handlers...),
	}
}

// +kubebuilder:rbac:groups=sreportal.io,resources=alertmanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=alertmanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=alertmanagers/finalizers,verbs=update

// Reconcile fetches active alerts from Alertmanager and updates the resource status.
func (r *AlertmanagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var resource sreportalv1alpha1.Alertmanager
	if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.V(1).Info("reconciling Alertmanager", "name", resource.Name, "portalRef", resource.Spec.PortalRef, "isRemote", resource.Spec.IsRemote)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Alertmanager]{
		Resource: &resource,
		Data:     make(map[string]any),
	}

	if err := r.chain.Execute(ctx, rc); err != nil {
		log.Error(err, "reconciliation chain failed")
		return ctrl.Result{RequeueAfter: alertmanagerRequeueAfter}, nil
	}

	if rc.Result.RequeueAfter > 0 {
		return rc.Result, nil
	}

	return ctrl.Result{RequeueAfter: alertmanagerRequeueAfter}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AlertmanagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.Alertmanager{}).
		Named("alertmanager").
		Complete(r)
}
