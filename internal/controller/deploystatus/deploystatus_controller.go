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

// Package deploystatus contains the reconciler for the DeployStatus CRD.
package deploystatus

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	deploystatuschain "github.com/golgoth31/sreportal/internal/controller/deploystatus/chain"
	domdeploystatus "github.com/golgoth31/sreportal/internal/domain/deploystatus"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
	"github.com/golgoth31/sreportal/internal/tlsutil"
)

const (
	// finalizerName is the finalizer added to every DeployStatus CR.
	finalizerName = "deploystatus.sreportal.io/cleanup"

	// portalRefField is the field index name used to look up DeployStatus CRs by portalRef.
	portalRefField = "spec.portalRef"

	// defaultRequeueInterval is the fallback periodic requeue cadence when the
	// configured refresh interval is zero.
	defaultRequeueInterval = 5 * time.Minute
)

// DeployStatusReconciler reconciles a DeployStatus object.
type DeployStatusReconciler struct {
	client.Client
	chain *reconciler.Chain[*sreportalv1alpha1.DeployStatus, deploystatuschain.ChainData]

	store             domdeploystatus.Writer
	remoteClientCache *remoteclient.Cache
	refreshInterval   time.Duration
}

// +kubebuilder:rbac:groups=sreportal.io,resources=deploystatuses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=deploystatuses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=deploystatuses/finalizers,verbs=update

// NewDeployStatusReconciler builds a DeployStatusReconciler wired with the
// handler chain.
func NewDeployStatusReconciler(
	c client.Client,
	store domdeploystatus.Writer,
	clientFor func(host string) forge.Client,
	remoteClientCache *remoteclient.Cache,
	cfg *config.DeployStatusConfig,
) *DeployStatusReconciler {
	refreshInterval := defaultRequeueInterval
	var forges []config.ForgeConfig
	if cfg != nil {
		if d := time.Duration(cfg.RefreshInterval); d > 0 {
			refreshInterval = d
		}
		forges = cfg.Forges
	}

	handlers := []reconciler.Handler[*sreportalv1alpha1.DeployStatus, deploystatuschain.ChainData]{
		deploystatuschain.NewSelectDueHandler(refreshInterval),
		deploystatuschain.NewResolveOCISourceHandler(forges),
		deploystatuschain.NewForgeCompareHandler(clientFor),
		deploystatuschain.NewResolveDeployRunHandler(clientFor, forges),
		deploystatuschain.NewUpdateReadStoreHandler(store),
		deploystatuschain.NewUpdateStatusHandler(c),
	}

	return &DeployStatusReconciler{
		Client:            c,
		chain:             reconciler.NewChain("deploystatus", handlers...),
		store:             store,
		remoteClientCache: remoteClientCache,
		refreshInterval:   refreshInterval,
	}
}

// Reconcile is the main reconciliation loop entry point.
func (r *DeployStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var cr sreportalv1alpha1.DeployStatus
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion — run finalizer cleanup.
	if !cr.DeletionTimestamp.IsZero() {
		return r.handleFinalizer(ctx, &cr)
	}

	// Ensure finalizer is registered.
	if !controllerutil.ContainsFinalizer(&cr, finalizerName) {
		controllerutil.AddFinalizer(&cr, finalizerName)
		if err := r.Update(ctx, &cr); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		// Re-fetch after update.
		if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Remote shadow CR: entries are fetched from a remote portal's
	// DeployStatusService rather than computed locally. Skip the local compute
	// chain and project the remote entries into the local readstore.
	if cr.Spec.IsRemote {
		if err := r.reconcileRemote(ctx, &cr); err != nil {
			// Best-effort: a remote fetch error must not crash the reconcile.
			// Surface it on status + log, then requeue with the periodic interval.
			logger.Error(err, "remote DeployStatus fetch failed", "name", cr.Name, "portal", cr.Spec.PortalRef)
			r.setRemoteLastError(ctx, &cr, err.Error())
			return ctrl.Result{RequeueAfter: r.refreshInterval}, nil
		}
		r.setRemoteLastError(ctx, &cr, "")
		return ctrl.Result{RequeueAfter: r.refreshInterval}, nil
	}

	logger.V(1).Info("reconciling DeployStatus",
		"name", cr.Name,
		"portal", cr.Spec.PortalRef,
		"namespace", cr.Spec.Namespace,
		"services", len(cr.Spec.Services),
	)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, deploystatuschain.ChainData]{
		Resource: &cr,
	}

	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation chain failed")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.refreshInterval}, nil
}

// handleFinalizer removes the readstore contribution for this CR, then removes
// the finalizer so Kubernetes can garbage-collect the CR.
func (r *DeployStatusReconciler) handleFinalizer(ctx context.Context, cr *sreportalv1alpha1.DeployStatus) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(cr, finalizerName) {
		// Drop this CR's readstore contribution. The in-memory store cannot fail,
		// but keeping the cleanup before finalizer removal keeps the contract
		// future-proof against a persistent store.
		r.store.RemoveForNamespace(cr.Spec.PortalRef, cr.Spec.Namespace)

		controllerutil.RemoveFinalizer(cr, finalizerName)
		if err := r.Update(ctx, cr); err != nil {
			return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
		}
	}
	return ctrl.Result{}, nil
}

// reconcileRemote fetches deploy-status entries from the remote portal pointed
// to by this shadow CR and projects them into the local readstore under a
// single (portalRef, namespace) bucket (cr.Spec.Namespace).
//
// All of the remote portal's namespaces are aggregated into that single bucket
// so the finalizer's single RemoveForNamespace(portalRef, namespace) call
// cleans up the whole contribution on deletion. List() on the read side
// aggregates entries across buckets, so the local-vs-remote keying does not
// affect query results.
func (r *DeployStatusReconciler) reconcileRemote(ctx context.Context, cr *sreportalv1alpha1.DeployStatus) error {
	logger := log.FromContext(ctx)

	portal := &sreportalv1alpha1.Portal{}
	portalKey := types.NamespacedName{Name: cr.Spec.PortalRef, Namespace: cr.Namespace}
	if err := r.Get(ctx, portalKey, portal); err != nil {
		return fmt.Errorf("get portal %s: %w", portalKey, err)
	}
	if portal.Spec.Remote == nil {
		return fmt.Errorf("portal %s has no remote configuration but DeployStatus is marked as remote", portalKey.Name)
	}

	remoteClient, err := r.remoteClientFor(ctx, portal)
	if err != nil {
		return fmt.Errorf("build remote client for portal %s: %w", portalKey.Name, err)
	}

	baseURL := portal.Spec.Remote.URL
	portalName := portal.Spec.Remote.Portal
	logger.V(1).Info("fetching deploy status from remote portal", "url", baseURL, "portalName", portalName)

	result, err := remoteClient.FetchDeployStatus(ctx, baseURL, portalName)
	if err != nil {
		return fmt.Errorf("fetch remote deploy status from %s: %w", baseURL, err)
	}

	entries := make([]domdeploystatus.Entry, 0, len(result.Entries))
	for _, re := range result.Entries {
		entries = append(entries, mapRemoteEntry(re))
	}

	r.store.ReplaceForNamespace(cr.Spec.PortalRef, cr.Spec.Namespace, entries)
	logger.V(1).Info("projected remote deploy status", "portal", cr.Spec.PortalRef, "entries", len(entries))
	return nil
}

// setRemoteLastError patches Status.LastError on a remote shadow CR (best-effort:
// a patch failure is logged but never propagated, so it can't crash the reconcile).
func (r *DeployStatusReconciler) setRemoteLastError(ctx context.Context, cr *sreportalv1alpha1.DeployStatus, msg string) {
	if cr.Status.LastError == msg {
		return
	}
	base := cr.DeepCopy()
	cr.Status.LastError = msg
	if err := r.Status().Patch(ctx, cr, client.MergeFrom(base)); err != nil {
		log.FromContext(ctx).Error(err, "failed to patch remote DeployStatus status.lastError", "name", cr.Name)
	}
}

// mapRemoteEntry converts a remoteclient RemoteDeployStatusEntry into the
// readstore domain Entry.
func mapRemoteEntry(re remoteclient.RemoteDeployStatusEntry) domdeploystatus.Entry {
	commits := make([]domdeploystatus.Commit, 0, len(re.PendingCommits))
	for _, c := range re.PendingCommits {
		commit := domdeploystatus.Commit{
			Sha:     c.Sha,
			Message: c.Message,
			Author:  c.Author,
			URL:     c.URL,
		}
		if c.Date != nil {
			commit.Date = *c.Date
		}
		commits = append(commits, commit)
	}

	entry := domdeploystatus.Entry{
		Key: re.Key,
		Workload: domdeploystatus.WorkloadRef{
			Kind:      re.Workload.Kind,
			Namespace: re.Workload.Namespace,
			Name:      re.Workload.Name,
			Container: re.Workload.Container,
		},
		Image:            re.Image,
		SourceRepo:       re.SourceRepo,
		DeployedRef:      re.DeployedRef,
		DefaultBranch:    re.DefaultBranch,
		AheadBy:          re.AheadBy,
		PendingCommits:   commits,
		PendingTruncated: re.PendingTruncated,
		DeployRunURL:     re.DeployRunURL,
		State:            re.State,
		Error:            re.Error,
	}
	if re.DeployedAt != nil {
		entry.DeployedAt = *re.DeployedAt
	}
	if re.LastCheckedAt != nil {
		entry.LastCheckedAt = *re.LastCheckedAt
	}
	return entry
}

// remoteClientFor returns a cached remoteclient configured with TLS from the
// Portal spec, mirroring the ImageInventory FetchRemoteImagesHandler.
func (r *DeployStatusReconciler) remoteClientFor(ctx context.Context, portal *sreportalv1alpha1.Portal) (*remoteclient.Client, error) {
	if portal.Spec.Remote.TLS == nil {
		return r.remoteClientCache.Fallback(), nil
	}

	key := portal.Namespace + "/" + portal.Name
	versions, err := tlsutil.SecretVersions(ctx, r.Client, portal.Namespace, portal.Spec.Remote.TLS)
	if err != nil {
		return nil, fmt.Errorf("read TLS secret versions: %w", err)
	}

	if cached := r.remoteClientCache.Get(key, versions); cached != nil {
		return cached, nil
	}

	tlsConfig, err := tlsutil.BuildTLSConfig(ctx, r.Client, portal.Namespace, portal.Spec.Remote.TLS)
	if err != nil {
		return nil, fmt.Errorf("build TLS config: %w", err)
	}

	c := remoteclient.NewClient(remoteclient.WithTLSConfig(tlsConfig))
	r.remoteClientCache.Put(key, versions, c)

	return c, nil
}

// SetupWithManager registers the controller and installs the field indexer.
func (r *DeployStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Index DeployStatus by spec.portalRef so we can efficiently look up all
	// CRs belonging to a given portal.
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.DeployStatus{},
		portalRefField,
		func(obj client.Object) []string {
			ds, ok := obj.(*sreportalv1alpha1.DeployStatus)
			if !ok || ds.Spec.PortalRef == "" {
				return nil
			}
			return []string{ds.Spec.PortalRef}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.DeployStatus{}).
		Named("deploystatus").
		Complete(r)
}
