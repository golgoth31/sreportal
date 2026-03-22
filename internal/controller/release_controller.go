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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	"github.com/golgoth31/sreportal/internal/log"
)

const releaseRequeueInterval = 12 * time.Hour

// CacheInvalidator is the interface the release service must implement
// for the controller to invalidate its caches.
type CacheInvalidator interface {
	InvalidateDay(day string)
	InvalidateDays()
}

// ReleaseReconciler watches Release CRs, invalidates the in-memory cache,
// and deletes expired CRs based on the configured TTL.
type ReleaseReconciler struct {
	client.Client
	cache CacheInvalidator
	ttl   time.Duration
	now   func() time.Time // injectable for testing
}

// NewReleaseReconciler creates a new ReleaseReconciler.
func NewReleaseReconciler(c client.Client, cache CacheInvalidator, ttl time.Duration) *ReleaseReconciler {
	return &ReleaseReconciler{
		Client: c,
		cache:  cache,
		ttl:    ttl,
		now:    time.Now,
	}
}

// +kubebuilder:rbac:groups=sreportal.io,resources=releases,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=releases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=releases/finalizers,verbs=update

// Reconcile invalidates the release cache for the affected day and deletes
// expired CRs that exceed the configured TTL.
func (r *ReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	day, err := domainrelease.ParseDateFromCRName(req.Name)
	if err != nil {
		logger.V(1).Info("skipping release CR with unparseable name", "name", req.Name)
		return ctrl.Result{}, nil
	}

	// Check if the CR still exists (may have been deleted)
	var rel sreportalv1alpha1.Release
	if err := r.Get(ctx, req.NamespacedName, &rel); err != nil {
		if apierrors.IsNotFound(err) {
			// CR was deleted — invalidate caches
			logger.V(1).Info("release CR deleted, invalidating cache", "day", day)
			r.cache.InvalidateDay(day)
			r.cache.InvalidateDays()
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
		// Cache invalidation will happen when the delete triggers a new reconcile
		return ctrl.Result{}, nil
	}

	// Not expired — invalidate caches and requeue for periodic TTL re-check
	logger.V(1).Info("invalidating release cache", "day", day, "name", req.Name)
	r.cache.InvalidateDay(day)
	r.cache.InvalidateDays()

	return ctrl.Result{RequeueAfter: releaseRequeueInterval}, nil
}

// SetupWithManager registers the controller with the manager.
func (r *ReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.Release{}).
		Named("release").
		Complete(r)
}
