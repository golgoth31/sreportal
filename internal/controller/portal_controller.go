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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	alertmanagerctrl "github.com/golgoth31/sreportal/internal/controller/alertmanager"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/remoteclient"
	"github.com/golgoth31/sreportal/internal/tlsutil"
)

// DefaultRemoteSyncInterval is the default interval for syncing remote portals.
const DefaultRemoteSyncInterval = 5 * time.Minute

// PortalReconciler reconciles a Portal object
type PortalReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	remoteClientCache *remoteclient.Cache
	portalWriter      domainportal.PortalWriter
	fqdnWriter        domaindns.FQDNWriter
}

// SetPortalWriter sets the optional PortalWriter used to push read models into the ReadStore.
func (r *PortalReconciler) SetPortalWriter(w domainportal.PortalWriter) {
	r.portalWriter = w
}

// SetFQDNWriter sets the optional FQDNWriter used to project remote FQDNs into the ReadStore.
func (r *PortalReconciler) SetFQDNWriter(w domaindns.FQDNWriter) {
	r.fqdnWriter = w
}

// NewPortalReconciler creates a new PortalReconciler
func NewPortalReconciler(c client.Client, scheme *runtime.Scheme, cache *remoteclient.Cache) *PortalReconciler {
	return &PortalReconciler{
		Client:            c,
		Scheme:            scheme,
		remoteClientCache: cache,
	}
}

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
				// Clean up remote DNS FQDN projections from read store
				remoteDNSKey := req.Namespace + "/" + remoteDNSName(req.Name)
				_ = r.fqdnWriter.Delete(ctx, remoteDNSKey)
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconciling Portal", "name", portal.Name, "namespace", portal.Namespace)

	var result ctrl.Result
	var reconcileErr error

	// Check if this is a remote portal
	if portal.Spec.Remote != nil {
		result, reconcileErr = r.reconcileRemotePortal(ctx, &portal)
	} else {
		reconcileErr = r.reconcileLocalPortal(ctx, &portal)
	}

	// Push portal view into the ReadStore
	if r.portalWriter != nil {
		resourceKey := portal.Namespace + "/" + portal.Name
		if reconcileErr == nil {
			_ = r.portalWriter.Replace(ctx, resourceKey, portalToView(&portal))
		}
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

	if reconcileErr != nil {
		metrics.ReconcileTotal.WithLabelValues("portal", "error").Inc()
	} else {
		metrics.ReconcileTotal.WithLabelValues("portal", "success").Inc()
	}
	metrics.ReconcileDuration.WithLabelValues("portal").Observe(time.Since(start).Seconds())

	return result, reconcileErr
}

// reconcileLocalPortal handles reconciliation for local portals (no URL specified).
func (r *PortalReconciler) reconcileLocalPortal(ctx context.Context, portal *sreportalv1alpha1.Portal) error {
	base := portal.DeepCopy()

	// Update status for local portal
	portal.Status.Ready = true
	portal.Status.RemoteSync = nil // Clear any previous remote sync status

	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "PortalConfigured",
		Message:            "Portal is fully configured",
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&portal.Status.Conditions, readyCondition)

	if err := r.Status().Patch(ctx, portal, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch Portal status: %w", err)
	}

	return nil
}

// remoteClientFor returns a cached remote client for the given portal.
// Clients are cached per portal and invalidated when referenced TLS secrets change.
func (r *PortalReconciler) remoteClientFor(ctx context.Context, portal *sreportalv1alpha1.Portal) (*remoteclient.Client, error) {
	remote := portal.Spec.Remote
	if remote.TLS == nil {
		return r.remoteClientCache.Fallback(), nil
	}

	key := portal.Namespace + "/" + portal.Name
	versions, err := tlsutil.SecretVersions(ctx, r.Client, portal.Namespace, remote.TLS)
	if err != nil {
		return nil, fmt.Errorf("read TLS secret versions: %w", err)
	}

	if cached := r.remoteClientCache.Get(key, versions); cached != nil {
		return cached, nil
	}

	tlsConfig, err := tlsutil.BuildTLSConfig(ctx, r.Client, portal.Namespace, remote.TLS)
	if err != nil {
		return nil, fmt.Errorf("build TLS config: %w", err)
	}

	c := remoteclient.NewClient(remoteclient.WithTLSConfig(tlsConfig))
	r.remoteClientCache.Put(key, versions, c)

	return c, nil
}

// reconcileRemotePortal handles reconciliation for remote portals (Remote specified).
func (r *PortalReconciler) reconcileRemotePortal(ctx context.Context, portal *sreportalv1alpha1.Portal) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	remote := portal.Spec.Remote
	logger.Info("reconciling remote portal", "url", remote.URL, "remotePortal", remote.Portal)

	remoteClient, err := r.remoteClientFor(ctx, portal)
	if err != nil {
		logger.Error(err, "failed to build remote client", "url", remote.URL)

		base := portal.DeepCopy()
		portal.Status.Ready = false
		if portal.Status.RemoteSync == nil {
			portal.Status.RemoteSync = &sreportalv1alpha1.RemoteSyncStatus{}
		}
		portal.Status.RemoteSync.LastSyncError = err.Error()

		meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "TLSConfigFailed",
			Message:            "Failed to build TLS configuration: " + err.Error(),
			LastTransitionTime: metav1.Now(),
		})

		if patchErr := r.Status().Patch(ctx, portal, client.MergeFrom(base)); patchErr != nil {
			return ctrl.Result{}, fmt.Errorf("patch Portal status: %w", patchErr)
		}

		return ctrl.Result{RequeueAfter: DefaultRemoteSyncInterval}, nil
	}

	// Initialize RemoteSync status if nil
	if portal.Status.RemoteSync == nil {
		portal.Status.RemoteSync = &sreportalv1alpha1.RemoteSyncStatus{}
	}

	// Perform health check on remote portal
	remoteLog := log.Default().WithName("portal").WithName("remote")
	err = remoteClient.HealthCheck(ctx, remote.URL)
	if err != nil {
		metrics.PortalRemoteSyncErrorsTotal.WithLabelValues(portal.Name).Inc()
		remoteLog.Error(err, "remote portal health check failed", "name", portal.Name, "namespace", portal.Namespace, "url", remote.URL, "error", err.Error())

		base := portal.DeepCopy()
		portal.Status.Ready = false
		portal.Status.RemoteSync.LastSyncError = err.Error()

		readyCondition := metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "RemoteConnectionFailed",
			Message:            "Failed to connect to remote portal: " + err.Error(),
			LastTransitionTime: metav1.Now(),
		}
		meta.SetStatusCondition(&portal.Status.Conditions, readyCondition)

		if patchErr := r.Status().Patch(ctx, portal, client.MergeFrom(base)); patchErr != nil {
			return ctrl.Result{}, fmt.Errorf("patch Portal status: %w", patchErr)
		}

		// Requeue to retry connection
		return ctrl.Result{RequeueAfter: DefaultRemoteSyncInterval}, nil
	}

	// Fetch remote portal info
	result, err := remoteClient.FetchFQDNs(ctx, remote.URL, remote.Portal)
	if err != nil {
		metrics.PortalRemoteSyncErrorsTotal.WithLabelValues(portal.Name).Inc()
		remoteLog.Warn("failed to fetch FQDNs from remote portal", "name", portal.Name, "namespace", portal.Namespace, "url", remote.URL, "remotePortal", remote.Portal, "error", err.Error())

		base := portal.DeepCopy()
		portal.Status.Ready = false
		portal.Status.RemoteSync.LastSyncError = err.Error()

		readyCondition := metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "RemoteFetchFailed",
			Message:            "Failed to fetch data from remote portal: " + err.Error(),
			LastTransitionTime: metav1.Now(),
		}
		meta.SetStatusCondition(&portal.Status.Conditions, readyCondition)

		if patchErr := r.Status().Patch(ctx, portal, client.MergeFrom(base)); patchErr != nil {
			return ctrl.Result{}, fmt.Errorf("patch Portal status: %w", patchErr)
		}

		return ctrl.Result{RequeueAfter: DefaultRemoteSyncInterval}, nil
	}

	// Update status with successful sync
	base := portal.DeepCopy()
	now := metav1.Now()
	portal.Status.Ready = true
	portal.Status.RemoteSync.LastSyncTime = &now
	portal.Status.RemoteSync.LastSyncError = ""
	portal.Status.RemoteSync.RemoteTitle = result.RemoteTitle
	portal.Status.RemoteSync.FQDNCount = result.FQDNCount

	// Create or update DNS CR with fetched FQDNs. A failure here does not
	// block the portal being marked Ready — the remote connection succeeded —
	// but we surface it as a separate DNSSynced condition so operators can act.
	if portal.Spec.Features.IsDNSEnabled() {
		if err := r.reconcileRemoteDNS(ctx, portal, result); err != nil {
			remoteLog.Warn("failed to reconcile DNS for remote portal", "name", portal.Name, "namespace", portal.Namespace, "error", err.Error())
			meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
				Type:               "DNSSynced",
				Status:             metav1.ConditionFalse,
				Reason:             "DNSSyncFailed",
				Message:            fmt.Sprintf("Failed to sync DNS from remote portal: %v", err),
				LastTransitionTime: metav1.Now(),
			})
		} else {
			meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
				Type:               "DNSSynced",
				Status:             metav1.ConditionTrue,
				Reason:             "DNSSyncSuccess",
				Message:            fmt.Sprintf("Synced %d FQDNs from remote portal", result.FQDNCount),
				LastTransitionTime: metav1.Now(),
			})
		}
	} else {
		remoteLog.V(1).Info("DNS feature disabled, skipping remote DNS sync", "portal", portal.Name)
	}

	// Create or update Alertmanager CR for remote portal so the alertmanager
	// controller can fetch alerts from the remote portal's API.
	if portal.Spec.Features.IsAlertsEnabled() {
		if err := r.reconcileRemoteAlertmanager(ctx, portal); err != nil {
			remoteLog.Error(err, "failed to reconcile Alertmanager for remote portal")
			meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
				Type:               "AlertsSynced",
				Status:             metav1.ConditionFalse,
				Reason:             "AlertsSyncFailed",
				Message:            fmt.Sprintf("Failed to sync Alertmanager resources for remote portal: %v", err),
				LastTransitionTime: metav1.Now(),
			})
		} else {
			meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
				Type:               "AlertsSynced",
				Status:             metav1.ConditionTrue,
				Reason:             "AlertsSyncSuccess",
				Message:            "Alertmanager resources synced for remote portal",
				LastTransitionTime: metav1.Now(),
			})
		}
	} else {
		remoteLog.V(1).Info("alerts feature disabled, skipping remote alertmanager sync", "portal", portal.Name)
	}

	// Create or update FlowNodeSet/FlowEdgeSet CRs for remote network flows.
	if portal.Spec.Features.IsNetworkPolicyEnabled() {
		if err := r.reconcileRemoteNetworkFlows(ctx, portal); err != nil {
			remoteLog.Warn("failed to reconcile network flows for remote portal", "name", portal.Name, "namespace", portal.Namespace, "error", err.Error())
			meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
				Type:               "NetworkFlowsSynced",
				Status:             metav1.ConditionFalse,
				Reason:             "NetworkFlowsSyncFailed",
				Message:            fmt.Sprintf("Failed to sync network flows from remote portal: %v", err),
				LastTransitionTime: metav1.Now(),
			})
		} else {
			meta.SetStatusCondition(&portal.Status.Conditions, metav1.Condition{
				Type:               "NetworkFlowsSynced",
				Status:             metav1.ConditionTrue,
				Reason:             "NetworkFlowsSyncSuccess",
				Message:            "Network flows synced for remote portal",
				LastTransitionTime: metav1.Now(),
			})
		}
	} else {
		remoteLog.V(1).Info("networkPolicy feature disabled, skipping remote network flows sync", "portal", portal.Name)
	}

	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "RemoteSyncSuccess",
		Message:            "Successfully synced with remote portal",
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&portal.Status.Conditions, readyCondition)

	if err := r.Status().Patch(ctx, portal, client.MergeFrom(base)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch Portal status: %w", err)
	}

	metrics.PortalRemoteFQDNsSynced.WithLabelValues(portal.Name).Set(float64(result.FQDNCount))

	remoteLog.Info("remote portal sync successful",
		"url", remote.URL,
		"remotePortal", remote.Portal,
		"fqdnCount", result.FQDNCount,
		"groupCount", len(result.Groups),
		"remoteTitle", result.RemoteTitle)

	// Requeue to periodically sync with remote
	return ctrl.Result{RequeueAfter: DefaultRemoteSyncInterval}, nil
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
			DNS:           p.Spec.Features.IsDNSEnabled(),
			Releases:      p.Spec.Features.IsReleasesEnabled(),
			NetworkPolicy: p.Spec.Features.IsNetworkPolicyEnabled(),
			Alerts:        p.Spec.Features.IsAlertsEnabled(),
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

// remoteAlertmanagerName returns the name of the local Alertmanager CR
// that mirrors a specific remote alertmanager.
func remoteAlertmanagerName(portalName, remoteAMName string) string {
	return fmt.Sprintf("remote-%s-%s", portalName, remoteAMName)
}

// reconcileRemoteAlertmanager discovers alertmanagers on the remote portal and creates
// one local Alertmanager CR per remote alertmanager. Each CR is configured with
// isRemote=true and the correct spec.url.remote so the alertmanager controller can
// fetch alerts independently. Orphaned local CRs (remote alertmanager no longer exists)
// are garbage-collected via owner references.
func (r *PortalReconciler) reconcileRemoteAlertmanager(ctx context.Context, portal *sreportalv1alpha1.Portal) error {
	logger := log.FromContext(ctx)

	remoteClient, err := r.remoteClientFor(ctx, portal)
	if err != nil {
		return fmt.Errorf("build remote client: %w", err)
	}

	// Discover all alertmanagers on the remote portal.
	remoteAMs, err := remoteClient.DiscoverAlertmanagers(ctx, portal.Spec.Remote.URL, portal.Spec.Remote.Portal)
	if err != nil {
		return fmt.Errorf("discover remote alertmanagers: %w", err)
	}

	logger.V(1).Info("discovered remote alertmanagers", "count", len(remoteAMs))

	// Track which local CR names are expected so we can clean up orphans.
	expectedNames := make(map[string]struct{}, len(remoteAMs))

	for _, remoteAM := range remoteAMs {
		amName := remoteAlertmanagerName(portal.Name, remoteAM.Name)
		expectedNames[amName] = struct{}{}

		if err := r.ensureAlertmanagerCR(ctx, portal, amName, remoteAM); err != nil {
			return fmt.Errorf("ensure alertmanager CR %s: %w", amName, err)
		}
	}

	// Clean up orphaned local CRs that no longer have a corresponding remote alertmanager.
	if err := r.cleanupOrphanedAlertmanagers(ctx, portal, expectedNames); err != nil {
		return fmt.Errorf("cleanup orphaned alertmanagers: %w", err)
	}

	return nil
}

// ensureAlertmanagerCR creates or updates a local Alertmanager CR for a specific remote alertmanager.
func (r *PortalReconciler) ensureAlertmanagerCR(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	amName string,
	remoteAM remoteclient.RemoteAlertmanagerInfo,
) error {
	logger := log.FromContext(ctx)

	am := &sreportalv1alpha1.Alertmanager{}
	err := r.Get(ctx, types.NamespacedName{Name: amName, Namespace: portal.Namespace}, am)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get Alertmanager: %w", err)
	}

	isNew := errors.IsNotFound(err)
	if isNew {
		am = &sreportalv1alpha1.Alertmanager{
			ObjectMeta: metav1.ObjectMeta{
				Name:      amName,
				Namespace: portal.Namespace,
				Labels: map[string]string{
					alertmanagerctrl.LabelRemoteAlertmanagerName: remoteAM.Name,
				},
			},
			Spec: sreportalv1alpha1.AlertmanagerSpec{
				PortalRef: portal.Name,
				URL: sreportalv1alpha1.AlertmanagerURL{
					Local:  portal.Spec.Remote.URL,
					Remote: remoteAM.RemoteURL,
				},
				IsRemote: true,
			},
		}
	}

	if err := controllerutil.SetControllerReference(portal, am, r.Scheme); err != nil {
		return fmt.Errorf("set controller reference: %w", err)
	}

	if isNew {
		if err := r.Create(ctx, am); err != nil {
			return fmt.Errorf("create Alertmanager: %w", err)
		}
		logger.Info("created Alertmanager CR for remote alertmanager", "alertmanager", amName, "remoteAM", remoteAM.Name)

		return nil
	}

	// Update spec fields that may have changed.
	am.Spec.PortalRef = portal.Name
	am.Spec.URL.Local = portal.Spec.Remote.URL
	am.Spec.URL.Remote = remoteAM.RemoteURL
	am.Spec.IsRemote = true

	if am.Labels == nil {
		am.Labels = make(map[string]string)
	}
	am.Labels[alertmanagerctrl.LabelRemoteAlertmanagerName] = remoteAM.Name

	if err := r.Update(ctx, am); err != nil {
		return fmt.Errorf("update Alertmanager: %w", err)
	}
	logger.V(1).Info("updated Alertmanager CR for remote alertmanager", "alertmanager", amName, "remoteAM", remoteAM.Name)

	return nil
}

// cleanupOrphanedAlertmanagers deletes local Alertmanager CRs owned by this portal
// that no longer correspond to a remote alertmanager.
func (r *PortalReconciler) cleanupOrphanedAlertmanagers(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	expectedNames map[string]struct{},
) error {
	logger := log.FromContext(ctx)

	var amList sreportalv1alpha1.AlertmanagerList
	if err := r.List(ctx, &amList, client.InNamespace(portal.Namespace)); err != nil {
		return fmt.Errorf("list alertmanagers: %w", err)
	}

	for i := range amList.Items {
		am := &amList.Items[i]
		if !am.Spec.IsRemote || am.Spec.PortalRef != portal.Name {
			continue
		}

		if _, expected := expectedNames[am.Name]; expected {
			continue
		}

		// Check owner reference to ensure we only delete CRs we own.
		owned := false
		for _, ref := range am.OwnerReferences {
			if ref.UID == portal.UID {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}

		logger.Info("deleting orphaned Alertmanager CR", "alertmanager", am.Name)
		if err := r.Delete(ctx, am); err != nil {
			return fmt.Errorf("delete orphaned Alertmanager %s: %w", am.Name, err)
		}
	}

	return nil
}

// remoteDNSName returns the name of the DNS CR for a remote portal.
func remoteDNSName(portalName string) string {
	return fmt.Sprintf("remote-%s", portalName)
}

// reconcileRemoteDNS creates or updates a DNS CR with FQDNs fetched from a remote portal.
func (r *PortalReconciler) reconcileRemoteDNS(ctx context.Context, portal *sreportalv1alpha1.Portal, result *remoteclient.FetchResult) error {
	logger := log.FromContext(ctx)

	dnsName := remoteDNSName(portal.Name)
	dns := &sreportalv1alpha1.DNS{}

	// Try to get existing DNS CR
	err := r.Get(ctx, types.NamespacedName{Name: dnsName, Namespace: portal.Namespace}, dns)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get DNS: %w", err)
	}

	isNew := errors.IsNotFound(err)
	if isNew {
		// Create new DNS CR
		dns = &sreportalv1alpha1.DNS{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dnsName,
				Namespace: portal.Namespace,
			},
			Spec: sreportalv1alpha1.DNSSpec{
				PortalRef: portal.Name,
				IsRemote:  true,
			},
		}
	}

	// Set owner reference so DNS is deleted when Portal is deleted
	if err := controllerutil.SetControllerReference(portal, dns, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	if isNew {
		if err := r.Create(ctx, dns); err != nil {
			return fmt.Errorf("failed to create DNS: %w", err)
		}
		logger.Info("created DNS CR for remote portal", "dns", dnsName, "portal", portal.Name)
	} else {
		// Update spec if needed (portalRef or isRemote might have changed)
		dns.Spec.PortalRef = portal.Name
		dns.Spec.IsRemote = true
		if err := r.Update(ctx, dns); err != nil {
			return fmt.Errorf("failed to update DNS: %w", err)
		}
	}

	// Update status with fetched groups
	dnsBase := dns.DeepCopy()
	now := metav1.Now()
	dns.Status.Groups = result.Groups
	dns.Status.LastReconcileTime = &now

	// Set condition
	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "RemoteSyncSuccess",
		Message:            fmt.Sprintf("Successfully synced %d FQDNs from remote portal", result.FQDNCount),
		LastTransitionTime: now,
	}
	meta.SetStatusCondition(&dns.Status.Conditions, readyCondition)

	if err := r.Status().Patch(ctx, dns, client.MergeFrom(dnsBase)); err != nil {
		return fmt.Errorf("patch DNS status: %w", err)
	}

	// Project remote FQDNs into FQDN read store (DNS controller skips remote CRs)
	if r.fqdnWriter != nil {
		resourceKey := dns.Namespace + "/" + dns.Name
		views := groupsToFQDNViews(dns)
		if err := r.fqdnWriter.Replace(ctx, resourceKey, views); err != nil {
			logger.Error(err, "failed to update FQDN read store for remote DNS")
		}
	}

	logger.Info("updated DNS status with remote FQDNs",
		"dns", dnsName,
		"portal", portal.Name,
		"fqdnCount", result.FQDNCount,
		"groupCount", len(result.Groups))

	return nil
}

// remoteNFDName returns the name of the NetworkFlowDiscovery CR for a remote portal.
func remoteNFDName(portalName string) string {
	return fmt.Sprintf("remote-%s", portalName)
}

// reconcileRemoteNetworkFlows creates or updates a NetworkFlowDiscovery CR with
// isRemote=true for the given remote portal. The NFD controller will then handle
// fetching data from the remote portal and creating FlowNodeSet/FlowEdgeSet CRs,
// maintaining the same ownership chain as local: Portal → NFD → FlowNodeSet/FlowEdgeSet.
func (r *PortalReconciler) reconcileRemoteNetworkFlows(ctx context.Context, portal *sreportalv1alpha1.Portal) error {
	logger := log.FromContext(ctx)

	nfdName := remoteNFDName(portal.Name)
	nfd := &sreportalv1alpha1.NetworkFlowDiscovery{}

	err := r.Get(ctx, types.NamespacedName{Name: nfdName, Namespace: portal.Namespace}, nfd)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get NetworkFlowDiscovery: %w", err)
	}

	isNew := errors.IsNotFound(err)
	if isNew {
		nfd = &sreportalv1alpha1.NetworkFlowDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nfdName,
				Namespace: portal.Namespace,
			},
			Spec: sreportalv1alpha1.NetworkFlowDiscoverySpec{
				PortalRef: portal.Name,
				IsRemote:  true,
				RemoteURL: portal.Spec.Remote.URL,
			},
		}
	}

	if err := controllerutil.SetControllerReference(portal, nfd, r.Scheme); err != nil {
		return fmt.Errorf("set controller reference: %w", err)
	}

	if isNew {
		if err := r.Create(ctx, nfd); err != nil {
			return fmt.Errorf("create NetworkFlowDiscovery: %w", err)
		}
		logger.Info("created NetworkFlowDiscovery CR for remote portal", "nfd", nfdName)
	} else {
		nfd.Spec.PortalRef = portal.Name
		nfd.Spec.IsRemote = true
		nfd.Spec.RemoteURL = portal.Spec.Remote.URL
		if err := r.Update(ctx, nfd); err != nil {
			return fmt.Errorf("update NetworkFlowDiscovery: %w", err)
		}
	}

	return nil
}
