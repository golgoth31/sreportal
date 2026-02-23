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
	"github.com/golgoth31/sreportal/internal/remoteclient"
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

// +kubebuilder:rbac:groups=sreportal.my.domain,resources=portals,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.my.domain,resources=portals/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.my.domain,resources=portals/finalizers,verbs=update

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
	log := logf.FromContext(ctx)

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

	if err := r.Status().Update(ctx, portal); err != nil {
		log.Error(err, "failed to update Portal status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// remoteClientFor returns the appropriate remote client for the given remote spec.
// If InsecureSkipVerify is set, a new client with TLS verification disabled is created.
func (r *PortalReconciler) remoteClientFor(remote *sreportalv1alpha1.RemotePortalSpec) *remoteclient.Client {
	if remote.InsecureSkipVerify {
		return remoteclient.NewClient(remoteclient.WithInsecureSkipVerify(true))
	}
	return r.RemoteClient
}

// reconcileRemotePortal handles reconciliation for remote portals (Remote specified).
func (r *PortalReconciler) reconcileRemotePortal(ctx context.Context, portal *sreportalv1alpha1.Portal) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	remote := portal.Spec.Remote
	log.Info("reconciling remote portal", "url", remote.URL, "remotePortal", remote.Portal)

	remoteClient := r.remoteClientFor(remote)

	// Initialize RemoteSync status if nil
	if portal.Status.RemoteSync == nil {
		portal.Status.RemoteSync = &sreportalv1alpha1.RemoteSyncStatus{}
	}

	// Perform health check on remote portal
	err := remoteClient.HealthCheck(ctx, remote.URL)
	if err != nil {
		log.Error(err, "remote portal health check failed", "url", remote.URL)

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

		if updateErr := r.Status().Update(ctx, portal); updateErr != nil {
			log.Error(updateErr, "failed to update Portal status")
			return ctrl.Result{}, updateErr
		}

		// Requeue to retry connection
		return ctrl.Result{RequeueAfter: DefaultRemoteSyncInterval}, nil
	}

	// Fetch remote portal info
	result, err := remoteClient.FetchFQDNs(ctx, remote.URL, remote.Portal)
	if err != nil {
		log.Error(err, "failed to fetch FQDNs from remote portal", "url", remote.URL, "remotePortal", remote.Portal)

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

		if updateErr := r.Status().Update(ctx, portal); updateErr != nil {
			log.Error(updateErr, "failed to update Portal status")
			return ctrl.Result{}, updateErr
		}

		return ctrl.Result{RequeueAfter: DefaultRemoteSyncInterval}, nil
	}

	// Update status with successful sync
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

	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "RemoteSyncSuccess",
		Message:            "Successfully synced with remote portal",
		LastTransitionTime: metav1.Now(),
	}
	setPortalCondition(&portal.Status.Conditions, readyCondition)

	if err := r.Status().Update(ctx, portal); err != nil {
		log.Error(err, "failed to update Portal status")
		return ctrl.Result{}, err
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

	if err := r.Status().Update(ctx, dns); err != nil {
		return fmt.Errorf("failed to update DNS status: %w", err)
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
