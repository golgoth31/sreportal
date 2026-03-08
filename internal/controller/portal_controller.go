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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	alertmanagerctrl "github.com/golgoth31/sreportal/internal/controller/alertmanager"
	"github.com/golgoth31/sreportal/internal/remoteclient"
	"github.com/golgoth31/sreportal/internal/tlsutil"
)

// DefaultRemoteSyncInterval is the default interval for syncing remote portals.
const DefaultRemoteSyncInterval = 5 * time.Minute

// PortalReconciler reconciles a Portal object
type PortalReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	RemoteClient *remoteclient.Client
}

// NewPortalReconciler creates a new PortalReconciler
func NewPortalReconciler(c client.Client, scheme *runtime.Scheme) *PortalReconciler {
	return &PortalReconciler{
		Client:       c,
		Scheme:       scheme,
		RemoteClient: remoteclient.NewClient(),
	}
}

// +kubebuilder:rbac:groups=sreportal.io,resources=portals,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=portals/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=portals/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile updates the Portal status conditions.
func (r *PortalReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var portal sreportalv1alpha1.Portal
	if err := r.Get(ctx, req.NamespacedName, &portal); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling Portal", "name", portal.Name, "namespace", portal.Namespace)

	// Check if this is a remote portal
	if portal.Spec.Remote != nil {
		return r.reconcileRemotePortal(ctx, &portal)
	}

	return r.reconcileLocalPortal(ctx, &portal)
}

// reconcileLocalPortal handles reconciliation for local portals (no URL specified).
func (r *PortalReconciler) reconcileLocalPortal(ctx context.Context, portal *sreportalv1alpha1.Portal) (ctrl.Result, error) {
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
	setPortalCondition(&portal.Status.Conditions, readyCondition)

	if err := r.Status().Patch(ctx, portal, client.MergeFrom(base)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch Portal status: %w", err)
	}

	return ctrl.Result{}, nil
}

// remoteClientFor returns the appropriate remote client for the given portal.
// If TLS settings are configured, a new client with the built TLS config is created.
func (r *PortalReconciler) remoteClientFor(ctx context.Context, portal *sreportalv1alpha1.Portal) (*remoteclient.Client, error) {
	remote := portal.Spec.Remote
	if remote.TLS == nil {
		return r.RemoteClient, nil
	}

	tlsConfig, err := tlsutil.BuildTLSConfig(ctx, r.Client, portal.Namespace, remote.TLS)
	if err != nil {
		return nil, fmt.Errorf("build TLS config: %w", err)
	}

	return remoteclient.NewClient(remoteclient.WithTLSConfig(tlsConfig)), nil
}

// reconcileRemotePortal handles reconciliation for remote portals (Remote specified).
func (r *PortalReconciler) reconcileRemotePortal(ctx context.Context, portal *sreportalv1alpha1.Portal) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	remote := portal.Spec.Remote
	log.Info("reconciling remote portal", "url", remote.URL, "remotePortal", remote.Portal)

	remoteClient, err := r.remoteClientFor(ctx, portal)
	if err != nil {
		log.Error(err, "failed to build remote client", "url", remote.URL)

		base := portal.DeepCopy()
		portal.Status.Ready = false
		if portal.Status.RemoteSync == nil {
			portal.Status.RemoteSync = &sreportalv1alpha1.RemoteSyncStatus{}
		}
		portal.Status.RemoteSync.LastSyncError = err.Error()

		setPortalCondition(&portal.Status.Conditions, metav1.Condition{
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
	err = remoteClient.HealthCheck(ctx, remote.URL)
	if err != nil {
		log.Error(err, "remote portal health check failed", "url", remote.URL)

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
		setPortalCondition(&portal.Status.Conditions, readyCondition)

		if patchErr := r.Status().Patch(ctx, portal, client.MergeFrom(base)); patchErr != nil {
			return ctrl.Result{}, fmt.Errorf("patch Portal status: %w", patchErr)
		}

		// Requeue to retry connection
		return ctrl.Result{RequeueAfter: DefaultRemoteSyncInterval}, nil
	}

	// Fetch remote portal info
	result, err := remoteClient.FetchFQDNs(ctx, remote.URL, remote.Portal)
	if err != nil {
		log.Error(err, "failed to fetch FQDNs from remote portal", "url", remote.URL, "remotePortal", remote.Portal)

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
		setPortalCondition(&portal.Status.Conditions, readyCondition)

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
	if err := r.reconcileRemoteDNS(ctx, portal, result); err != nil {
		log.Error(err, "failed to reconcile DNS for remote portal")
		setPortalCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "DNSSynced",
			Status:             metav1.ConditionFalse,
			Reason:             "DNSSyncFailed",
			Message:            fmt.Sprintf("Failed to sync DNS from remote portal: %v", err),
			LastTransitionTime: metav1.Now(),
		})
	} else {
		setPortalCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "DNSSynced",
			Status:             metav1.ConditionTrue,
			Reason:             "DNSSyncSuccess",
			Message:            fmt.Sprintf("Synced %d FQDNs from remote portal", result.FQDNCount),
			LastTransitionTime: metav1.Now(),
		})
	}

	// Create or update Alertmanager CR for remote portal so the alertmanager
	// controller can fetch alerts from the remote portal's API.
	if err := r.reconcileRemoteAlertmanager(ctx, portal); err != nil {
		log.Error(err, "failed to reconcile Alertmanager for remote portal")
		setPortalCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "AlertsSynced",
			Status:             metav1.ConditionFalse,
			Reason:             "AlertsSyncFailed",
			Message:            fmt.Sprintf("Failed to sync Alertmanager resources for remote portal: %v", err),
			LastTransitionTime: metav1.Now(),
		})
	} else {
		setPortalCondition(&portal.Status.Conditions, metav1.Condition{
			Type:               "AlertsSynced",
			Status:             metav1.ConditionTrue,
			Reason:             "AlertsSyncSuccess",
			Message:            "Alertmanager resources synced for remote portal",
			LastTransitionTime: metav1.Now(),
		})
	}

	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "RemoteSyncSuccess",
		Message:            "Successfully synced with remote portal",
		LastTransitionTime: metav1.Now(),
	}
	setPortalCondition(&portal.Status.Conditions, readyCondition)

	if err := r.Status().Patch(ctx, portal, client.MergeFrom(base)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch Portal status: %w", err)
	}

	log.Info("remote portal sync successful",
		"url", remote.URL,
		"remotePortal", remote.Portal,
		"fqdnCount", result.FQDNCount,
		"groupCount", len(result.Groups),
		"remoteTitle", result.RemoteTitle)

	// Requeue to periodically sync with remote
	return ctrl.Result{RequeueAfter: DefaultRemoteSyncInterval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PortalReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.Portal{}).
		Named("portal").
		Complete(r)
}

// setPortalCondition sets or updates a condition in the conditions slice.
func setPortalCondition(conditions *[]metav1.Condition, newCondition metav1.Condition) {
	if conditions == nil {
		return
	}

	for i, c := range *conditions {
		if c.Type == newCondition.Type {
			if c.Status != newCondition.Status {
				(*conditions)[i] = newCondition
			} else {
				newCondition.LastTransitionTime = c.LastTransitionTime
				(*conditions)[i] = newCondition
			}
			return
		}
	}

	*conditions = append(*conditions, newCondition)
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
	log := logf.FromContext(ctx)

	remoteClient, err := r.remoteClientFor(ctx, portal)
	if err != nil {
		return fmt.Errorf("build remote client: %w", err)
	}

	// Discover all alertmanagers on the remote portal.
	remoteAMs, err := remoteClient.DiscoverAlertmanagers(ctx, portal.Spec.Remote.URL, portal.Spec.Remote.Portal)
	if err != nil {
		return fmt.Errorf("discover remote alertmanagers: %w", err)
	}

	log.V(1).Info("discovered remote alertmanagers", "count", len(remoteAMs))

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
	log := logf.FromContext(ctx)

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
		log.Info("created Alertmanager CR for remote alertmanager", "alertmanager", amName, "remoteAM", remoteAM.Name)

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
	log.V(1).Info("updated Alertmanager CR for remote alertmanager", "alertmanager", amName, "remoteAM", remoteAM.Name)

	return nil
}

// cleanupOrphanedAlertmanagers deletes local Alertmanager CRs owned by this portal
// that no longer correspond to a remote alertmanager.
func (r *PortalReconciler) cleanupOrphanedAlertmanagers(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	expectedNames map[string]struct{},
) error {
	log := logf.FromContext(ctx)

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

		log.Info("deleting orphaned Alertmanager CR", "alertmanager", am.Name)
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
	log := logf.FromContext(ctx)

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
		log.Info("created DNS CR for remote portal", "dns", dnsName, "portal", portal.Name)
	} else {
		// Update spec if needed (portalRef might have changed)
		dns.Spec.PortalRef = portal.Name
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
	setDNSCondition(&dns.Status.Conditions, readyCondition)

	if err := r.Status().Patch(ctx, dns, client.MergeFrom(dnsBase)); err != nil {
		return fmt.Errorf("patch DNS status: %w", err)
	}

	log.Info("updated DNS status with remote FQDNs",
		"dns", dnsName,
		"portal", portal.Name,
		"fqdnCount", result.FQDNCount,
		"groupCount", len(result.Groups))

	return nil
}

// setDNSCondition sets or updates a condition in the DNS conditions slice.
func setDNSCondition(conditions *[]metav1.Condition, newCondition metav1.Condition) {
	if conditions == nil {
		return
	}

	for i, c := range *conditions {
		if c.Type == newCondition.Type {
			if c.Status != newCondition.Status {
				(*conditions)[i] = newCondition
			} else {
				newCondition.LastTransitionTime = c.LastTransitionTime
				(*conditions)[i] = newCondition
			}
			return
		}
	}

	*conditions = append(*conditions, newCondition)
}
