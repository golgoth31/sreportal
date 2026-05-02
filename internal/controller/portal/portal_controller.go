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

package portal

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	portalchain "github.com/golgoth31/sreportal/internal/controller/portal/chain"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
)

// PortalReconciler reconciles a Portal object
type PortalReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	chain           *reconciler.Chain[*sreportalv1alpha1.Portal, portalchain.ChainData]
	portalWriter    domainportal.PortalWriter
	fqdnWriter      domaindns.FQDNWriter
	releaseWriter   domainrelease.ReleaseWriter
	flowGraphWriter domainnetpol.FlowGraphWriter
}

// SetPortalWriter sets the optional PortalWriter used to push read models into the ReadStore.
func (r *PortalReconciler) SetPortalWriter(w domainportal.PortalWriter) {
	r.portalWriter = w
}

// SetFQDNWriter sets the optional FQDNWriter used to project remote FQDNs into the ReadStore.
func (r *PortalReconciler) SetFQDNWriter(w domaindns.FQDNWriter) {
	r.fqdnWriter = w
}

// SetReleaseWriter sets the optional ReleaseWriter used to flush release projections when the feature is disabled.
func (r *PortalReconciler) SetReleaseWriter(w domainrelease.ReleaseWriter) {
	r.releaseWriter = w
}

// SetFlowGraphWriter sets the optional FlowGraphWriter used to purge network flow projections when the feature is disabled.
func (r *PortalReconciler) SetFlowGraphWriter(w domainnetpol.FlowGraphWriter) {
	r.flowGraphWriter = w
}

// NewPortalReconciler creates a new PortalReconciler with the handler chain.
func NewPortalReconciler(c client.Client, scheme *runtime.Scheme, cache *remoteclient.Cache) *PortalReconciler {
	handlers := []reconciler.Handler[*sreportalv1alpha1.Portal, portalchain.ChainData]{
		portalchain.NewCleanupDisabledFeaturesHandler(c),
		portalchain.NewEnsureLocalResourcesHandler(c, scheme),
		portalchain.NewBuildRemoteClientHandler(c, cache),
		portalchain.NewHealthCheckRemoteHandler(c),
		portalchain.NewFetchRemoteDataHandler(c),
		portalchain.NewSyncRemoteDNSHandler(c, scheme),
		portalchain.NewSyncRemoteAlertmanagerHandler(c, scheme),
		portalchain.NewSyncRemoteNetworkFlowsHandler(c, scheme),
		portalchain.NewSyncRemoteImageInventoryHandler(c, scheme),
		portalchain.NewUpdateStatusHandler(c),
	}

	return &PortalReconciler{
		Client: c,
		Scheme: scheme,
		chain:  reconciler.NewChain(handlers...),
	}
}

// +kubebuilder:rbac:groups=sreportal.io,resources=imageinventories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=releases,verbs=list
// +kubebuilder:rbac:groups=sreportal.io,resources=portals,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=portals/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=portals/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile updates the Portal status conditions.
func (r *PortalReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	var portal sreportalv1alpha1.Portal
	if err := r.Get(ctx, req.NamespacedName, &portal); err != nil {
		if client.IgnoreNotFound(err) == nil {
			if r.portalWriter != nil {
				_ = r.portalWriter.Delete(ctx, req.Namespace+"/"+req.Name)
			}
			if r.fqdnWriter != nil {
				remoteDNSKey := req.Namespace + "/" + portalchain.RemoteDNSName(req.Name)
				_ = r.fqdnWriter.Delete(ctx, remoteDNSKey)
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconciling Portal", "name", portal.Name, "namespace", portal.Namespace)

	// Create reconcile context with writer dependencies
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Portal, portalchain.ChainData]{
		Resource: &portal,
		Data: portalchain.ChainData{
			FQDNWriter:      r.fqdnWriter,
			ReleaseWriter:   r.releaseWriter,
			FlowGraphWriter: r.flowGraphWriter,
		},
	}

	// Execute handler chain
	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation failed")
		metrics.ReconcileTotal.WithLabelValues("portal", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("portal").Observe(time.Since(start).Seconds())
		return ctrl.Result{}, err
	}

	// Push portal view into the ReadStore
	if r.portalWriter != nil {
		resourceKey := portal.Namespace + "/" + portal.Name
		_ = r.portalWriter.Replace(ctx, resourceKey, portalToView(&portal))
	}

	// Recount all portals by type only from the main portal reconciliation
	if portal.Spec.Main {
		var allPortals sreportalv1alpha1.PortalList
		if listErr := r.List(ctx, &allPortals); listErr == nil {
			var local, remote float64
			for i := range allPortals.Items {
				if allPortals.Items[i].Spec.Remote != nil {
					remote++
				} else {
					local++
				}
			}
			metrics.PortalsTotal.WithLabelValues("local").Set(local)
			metrics.PortalsTotal.WithLabelValues("remote").Set(remote)
		}
	}

	metrics.ReconcileTotal.WithLabelValues("portal", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("portal").Observe(time.Since(start).Seconds())

	return rc.Result, nil
}

// portalToView converts a Portal CRD into a domain PortalView for the ReadStore.
func portalToView(p *sreportalv1alpha1.Portal) domainportal.PortalView {
	view := domainportal.PortalView{
		Name:      p.Name,
		Title:     p.Spec.Title,
		Main:      p.Spec.Main,
		SubPath:   p.Spec.SubPath,
		Namespace: p.Namespace,
		Ready:     p.Status.Ready,
		IsRemote:  p.Spec.Remote != nil,
		Features: domainportal.PortalFeatures{
			DNS:            p.Spec.Features.IsDNSEnabled(),
			Releases:       p.Spec.Features.IsReleasesEnabled(),
			NetworkPolicy:  p.Spec.Features.IsNetworkPolicyEnabled(),
			Alerts:         p.Spec.Features.IsAlertsEnabled(),
			StatusPage:     p.Spec.Features.IsStatusPageEnabled(),
			ImageInventory: p.Spec.Features.IsImageInventoryEnabled(),
		},
	}
	if p.Spec.Remote != nil {
		view.URL = p.Spec.Remote.URL
	}
	if p.Status.RemoteSync != nil {
		rs := &domainportal.RemoteSyncView{
			LastSyncError: p.Status.RemoteSync.LastSyncError,
			RemoteTitle:   p.Status.RemoteSync.RemoteTitle,
			FQDNCount:     p.Status.RemoteSync.FQDNCount,
		}
		if p.Status.RemoteSync.LastSyncTime != nil {
			rs.LastSyncTime = p.Status.RemoteSync.LastSyncTime.Format("2006-01-02T15:04:05Z07:00")
		}
		if p.Status.RemoteSync.Features != nil {
			rf := p.Status.RemoteSync.Features
			rs.RemoteFeatures = &domainportal.PortalFeatures{
				DNS:            rf.DNS,
				Releases:       rf.Releases,
				NetworkPolicy:  rf.NetworkPolicy,
				Alerts:         rf.Alerts,
				StatusPage:     rf.StatusPage,
				ImageInventory: rf.ImageInventory,
			}
			// Effective features for remote portals: local AND remote.
			view.Features.DNS = view.Features.DNS && rf.DNS
			view.Features.Releases = view.Features.Releases && rf.Releases
			view.Features.NetworkPolicy = view.Features.NetworkPolicy && rf.NetworkPolicy
			view.Features.Alerts = view.Features.Alerts && rf.Alerts
			view.Features.StatusPage = view.Features.StatusPage && rf.StatusPage
			view.Features.ImageInventory = view.Features.ImageInventory && rf.ImageInventory
		}
		view.RemoteSync = rs
	}
	return view
}

// SetupWithManager sets up the controller with the Manager.
func (r *PortalReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.Portal{}).
		Named("portal").
		Complete(r)
}
