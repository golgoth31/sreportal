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

package release

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
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	releasechain "github.com/golgoth31/sreportal/internal/controller/release/chain"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const releaseRequeueInterval = 12 * time.Hour

// ReleaseReconciler watches Release CRs, pushes read projections into the
// ReleaseWriter, and deletes expired CRs based on the configured TTL.
type ReleaseReconciler struct {
	client.Client
	releaseWriter domainrelease.ReleaseWriter
	chain         *reconciler.Chain[*sreportalv1alpha1.Release, releasechain.ChainData]
	ttl           time.Duration
	now           func() time.Time // injectable for testing
}

// NewReleaseReconciler creates a new ReleaseReconciler.
func NewReleaseReconciler(c client.Client, ttl time.Duration) *ReleaseReconciler {
	return &ReleaseReconciler{
		Client: c,
		ttl:    ttl,
		now:    time.Now,
	}
}

// SetReleaseWriter sets the writer used to push release projections and rebuilds
// the handler chain so the ProjectStoreHandler picks up the new writer.
func (r *ReleaseReconciler) SetReleaseWriter(w domainrelease.ReleaseWriter) {
	r.releaseWriter = w
	r.chain = reconciler.NewChain(
		"release",
		releasechain.NewResolvePortalHandler(r.Client, releaseRequeueInterval),
		releasechain.NewProjectStoreHandler(w),
	)
}

// +kubebuilder:rbac:groups=sreportal.io,resources=releases,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=releases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=releases/finalizers,verbs=update
// +kubebuilder:rbac:groups=sreportal.io,resources=portals,verbs=get;list;watch

// Reconcile orchestrates the release reconciliation: parses the date from the CR name,
// fetches the resource, deletes expired CRs, and otherwise executes the handler chain.
func (r *ReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	day, err := domainrelease.ParseDateFromCRName(req.Name)
	if err != nil {
		logger.V(1).Info("skipping release CR with unparseable name", "name", req.Name)
		return ctrl.Result{}, nil
	}

	resourceKey := req.Namespace + "/" + req.Name

	var rel sreportalv1alpha1.Release
	if err := r.Get(ctx, req.NamespacedName, &rel); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("release CR deleted, removing from store", "day", day, "key", resourceKey)
			if r.releaseWriter != nil {
				if delErr := r.releaseWriter.Delete(ctx, resourceKey); delErr != nil {
					logger.Error(delErr, "failed to delete release entry from read store", "key", resourceKey)
					metrics.ReadstoreWriterErrors.WithLabelValues("release", "delete").Inc()
				}
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get release CR: %w", err)
	}

	crDate, err := time.Parse("2006-01-02", day)
	if err != nil {
		return ctrl.Result{}, nil
	}
	if crDate.Before(r.now().UTC().Add(-r.ttl)) {
		logger.Info("deleting expired release CR", "name", req.Name, "day", day)
		if err := r.Delete(ctx, &rel); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to delete expired release CR", "name", req.Name, "day", day)
		}
		return ctrl.Result{}, nil
	}

	// Without a writer there's no store projection to do — preserve the historical
	// requeue cadence so the controller still revisits the CR.
	if r.chain == nil {
		return ctrl.Result{RequeueAfter: releaseRequeueInterval}, nil
	}

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Release, releasechain.ChainData]{
		Resource: &rel,
		Data: releasechain.ChainData{
			Day:         day,
			ResourceKey: resourceKey,
		},
	}
	if err := r.chain.Execute(ctx, rc); err != nil {
		return ctrl.Result{}, err
	}

	if rc.Result.RequeueAfter == 0 {
		rc.Result.RequeueAfter = releaseRequeueInterval
	}
	return rc.Result, nil
}

// SetupWithManager registers the controller with the manager.
func (r *ReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.Release{}).
		Watches(
			&sreportalv1alpha1.Portal{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
				portal, ok := obj.(*sreportalv1alpha1.Portal)
				if !ok {
					return nil
				}
				return portalfeatures.ReleaseReconcileRequestsForPortal(ctx, r.Client, portal)
			}),
			builder.WithPredicates(portalfeatures.PortalReleasesFeatureWakeupPredicate()),
		).
		Named("release").
		Complete(r)
}
