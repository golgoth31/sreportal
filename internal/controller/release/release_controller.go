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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	"github.com/golgoth31/sreportal/internal/log"
)

const releaseRequeueInterval = 12 * time.Hour

// ReleaseReconciler watches Release CRs, pushes read projections into the
// ReleaseWriter, and deletes expired CRs based on the configured TTL.
type ReleaseReconciler struct {
	client.Client
	releaseWriter domainrelease.ReleaseWriter
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

// SetReleaseWriter sets the writer used to push release projections.
func (r *ReleaseReconciler) SetReleaseWriter(w domainrelease.ReleaseWriter) {
	r.releaseWriter = w
}

// +kubebuilder:rbac:groups=sreportal.io,resources=releases,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=releases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=releases/finalizers,verbs=update
// +kubebuilder:rbac:groups=sreportal.io,resources=portals,verbs=get;list;watch

// Reconcile pushes release entries into the read store and deletes expired CRs.
func (r *ReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	day, err := domainrelease.ParseDateFromCRName(req.Name)
	if err != nil {
		logger.V(1).Info("skipping release CR with unparseable name", "name", req.Name)
		return ctrl.Result{}, nil
	}

	resourceKey := req.Namespace + "/" + req.Name

	// Check if the CR still exists (may have been deleted)
	var rel sreportalv1alpha1.Release
	if err := r.Get(ctx, req.NamespacedName, &rel); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("release CR deleted, removing from store", "day", day, "key", resourceKey)
			if r.releaseWriter != nil {
				_ = r.releaseWriter.Delete(ctx, resourceKey)
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get release CR: %w", err)
	}

	// Check TTL: delete if expired
	crDate, err := time.Parse("2006-01-02", day)
	if err != nil {
		return ctrl.Result{}, nil
	}

	cutoff := r.now().UTC().Add(-r.ttl)
	if crDate.Before(cutoff) {
		logger.Info("deleting expired release CR", "name", req.Name, "day", day)
		if err := r.Delete(ctx, &rel); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to delete expired release CR", "name", req.Name, "day", day)
			return ctrl.Result{}, nil
		}
		// Store cleanup will happen when the delete triggers a new reconcile
		return ctrl.Result{}, nil
	}

	if r.releaseWriter == nil {
		return ctrl.Result{RequeueAfter: releaseRequeueInterval}, nil
	}

	var portal sreportalv1alpha1.Portal
	if err := r.Get(ctx, types.NamespacedName{
		Name: rel.Spec.PortalRef, Namespace: rel.Namespace,
	}, &portal); err != nil {
		return ctrl.Result{}, fmt.Errorf("get portal %q: %w", rel.Spec.PortalRef, err)
	}
	if !portal.Spec.Features.IsReleasesEnabled() {
		logger.V(1).Info("releases feature disabled, skipping store projection",
			"day", day, "portal", rel.Spec.PortalRef)
		return ctrl.Result{RequeueAfter: releaseRequeueInterval}, nil
	}

	views := releaseEntriesToViews(rel.Spec.Entries, rel.Spec.PortalRef, day)
	if err := r.releaseWriter.Replace(ctx, resourceKey, views); err != nil {
		return ctrl.Result{}, fmt.Errorf("write release store: %w", err)
	}

	return ctrl.Result{RequeueAfter: releaseRequeueInterval}, nil
}

// releaseEntriesToViews converts CRD entries to domain read model views.
func releaseEntriesToViews(entries []sreportalv1alpha1.ReleaseEntry, portalRef, day string) []domainrelease.EntryView {
	views := make([]domainrelease.EntryView, 0, len(entries))
	for _, e := range entries {
		views = append(views, domainrelease.EntryView{
			PortalRef: portalRef,
			Day:       day,
			Type:      e.Type,
			Version:   e.Version,
			Origin:    e.Origin,
			Date:      e.Date.Time,
			Author:    e.Author,
			Message:   e.Message,
			Link:      e.Link,
		})
	}
	return views
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
