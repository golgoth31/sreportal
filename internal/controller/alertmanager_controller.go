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
	alertmanagerchain "github.com/golgoth31/sreportal/internal/controller/alertmanager"
	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanager"
	domainalertmanagerreadmodel "github.com/golgoth31/sreportal/internal/domain/alertmanagerreadmodel"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
)

const (
	alertmanagerRequeueAfter = 2 * time.Minute
)

// AlertmanagerReconciler reconciles an Alertmanager object using a chain of handlers.
type AlertmanagerReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	chain              *reconciler.Chain[*sreportalv1alpha1.Alertmanager, alertmanagerchain.ChainData]
	alertmanagerWriter domainalertmanagerreadmodel.AlertmanagerWriter
}

// SetAlertmanagerWriter sets the optional AlertmanagerWriter used to push read models into the ReadStore.
func (r *AlertmanagerReconciler) SetAlertmanagerWriter(w domainalertmanagerreadmodel.AlertmanagerWriter) {
	r.alertmanagerWriter = w
}

// NewAlertmanagerReconciler creates a new AlertmanagerReconciler with the handler chain.
// localDataFetcher may be nil; then localFetcher is used for basic alerts only.
// The K8s client is used by the FetchAlertsHandler to look up Portal CRs and read
// TLS secrets when fetching alerts from remote portals.
// The remoteClientCache is shared with PortalReconciler to reuse TLS connections.
func NewAlertmanagerReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	localDataFetcher domainalertmanager.DataFetcher,
	localFetcher domainalertmanager.Fetcher,
	remoteClientCache *remoteclient.Cache,
) *AlertmanagerReconciler {
	handlers := []reconciler.Handler[*sreportalv1alpha1.Alertmanager, alertmanagerchain.ChainData]{
		alertmanagerchain.NewFetchAlertsHandler(localDataFetcher, localFetcher, c, remoteClientCache),
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
	start := time.Now()
	logger := log.FromContext(ctx)

	var resource sreportalv1alpha1.Alertmanager
	if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
		if client.IgnoreNotFound(err) == nil && r.alertmanagerWriter != nil {
			_ = r.alertmanagerWriter.Delete(ctx, req.Namespace+"/"+req.Name)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(1).Info("reconciling Alertmanager", "name", resource.Name, "portalRef", resource.Spec.PortalRef, "isRemote", resource.Spec.IsRemote)

	// Skip reconciliation when alerts feature is disabled on the referenced portal.
	if resource.Spec.PortalRef != "" {
		var portal sreportalv1alpha1.Portal
		if err := r.Get(ctx, client.ObjectKey{Name: resource.Spec.PortalRef, Namespace: resource.Namespace}, &portal); err == nil {
			if !portal.Spec.Features.IsAlertsEnabled() {
				logger.V(1).Info("alerts feature disabled for portal, skipping", "portal", resource.Spec.PortalRef)
				return ctrl.Result{}, nil
			}
		}
	}

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Alertmanager, alertmanagerchain.ChainData]{
		Resource: &resource,
	}

	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation chain failed")
		metrics.ReconcileTotal.WithLabelValues("alertmanager", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("alertmanager").Observe(time.Since(start).Seconds())
		metrics.AlertsFetchErrorsTotal.WithLabelValues(resource.Name).Inc()
		return ctrl.Result{RequeueAfter: alertmanagerRequeueAfter}, nil
	}

	// Push alertmanager view into the ReadStore
	if r.alertmanagerWriter != nil {
		resourceKey := resource.Namespace + "/" + resource.Name
		_ = r.alertmanagerWriter.Replace(ctx, resourceKey, alertmanagerToView(&resource))
	}

	// Update active alerts gauge
	metrics.AlertsActive.WithLabelValues(resource.Spec.PortalRef, resource.Name).Set(float64(len(resource.Status.ActiveAlerts)))
	metrics.ReconcileTotal.WithLabelValues("alertmanager", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("alertmanager").Observe(time.Since(start).Seconds())

	if rc.Result.RequeueAfter > 0 {
		return rc.Result, nil
	}

	return ctrl.Result{RequeueAfter: alertmanagerRequeueAfter}, nil
}

// alertmanagerToView converts an Alertmanager CRD into a domain AlertmanagerView for the ReadStore.
func alertmanagerToView(am *sreportalv1alpha1.Alertmanager) domainalertmanagerreadmodel.AlertmanagerView {
	ready := false
	for _, c := range am.Status.Conditions {
		if c.Type == "Ready" && c.Status == "True" {
			ready = true
			break
		}
	}

	view := domainalertmanagerreadmodel.AlertmanagerView{
		Name:      am.Name,
		Namespace: am.Namespace,
		PortalRef: am.Spec.PortalRef,
		LocalURL:  am.Spec.URL.Local,
		RemoteURL: am.Spec.URL.Remote,
		Ready:     ready,
	}

	if am.Status.LastReconcileTime != nil {
		t := am.Status.LastReconcileTime.Time
		view.LastReconcileTime = &t
	}

	alerts := make([]domainalertmanagerreadmodel.AlertView, 0, len(am.Status.ActiveAlerts))
	for _, a := range am.Status.ActiveAlerts {
		av := domainalertmanagerreadmodel.AlertView{
			Fingerprint: a.Fingerprint,
			Labels:      a.Labels,
			Annotations: a.Annotations,
			State:       a.State,
			StartsAt:    a.StartsAt.Time,
			UpdatedAt:   a.UpdatedAt.Time,
			Receivers:   a.Receivers,
			SilencedBy:  a.SilencedBy,
		}
		if a.EndsAt != nil {
			t := a.EndsAt.Time
			av.EndsAt = &t
		}
		alerts = append(alerts, av)
	}
	view.Alerts = alerts

	silences := make([]domainalertmanagerreadmodel.SilenceView, 0, len(am.Status.Silences))
	for _, s := range am.Status.Silences {
		sv := domainalertmanagerreadmodel.SilenceView{
			ID:        s.ID,
			StartsAt:  s.StartsAt.Time,
			EndsAt:    s.EndsAt.Time,
			Status:    s.Status,
			CreatedBy: s.CreatedBy,
			Comment:   s.Comment,
			UpdatedAt: s.UpdatedAt.Time,
		}
		matchers := make([]domainalertmanagerreadmodel.MatcherView, 0, len(s.Matchers))
		for _, m := range s.Matchers {
			matchers = append(matchers, domainalertmanagerreadmodel.MatcherView{
				Name:    m.Name,
				Value:   m.Value,
				IsRegex: m.IsRegex,
			})
		}
		sv.Matchers = matchers
		silences = append(silences, sv)
	}
	view.Silences = silences

	return view
}

// SetupWithManager sets up the controller with the Manager.
func (r *AlertmanagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.Alertmanager{}).
		Named("alertmanager").
		Complete(r)
}
