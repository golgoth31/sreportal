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

// Package imageregistry contains the reconciler for the ImageRegistry CRD.
package imageregistry

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	imageregistrychain "github.com/golgoth31/sreportal/internal/controller/imageregistry/chain"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/registry"
)

const (
	// finalizerName is the finalizer added to every ImageRegistry CR.
	finalizerName = "imageregistry.sreportal.io/cleanup"

	// portalRefField is the field index name used to look up ImageRegistry CRs by portalRef.
	portalRefField = "spec.portalRef"

	// defaultRequeueInterval is the nominal cadence for reconciliation.
	defaultRequeueInterval = 24 * time.Hour

	// requeueJitter is half the jitter window applied to the default requeue interval
	// so peers don't all reconcile at once: final interval ∈ [23h, 25h].
	requeueJitter = time.Hour
)

// ImageRegistryReconciler reconciles an ImageRegistry object.
//
// +kubebuilder:rbac:groups=sreportal.io,resources=imageregistries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=imageregistries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=imageregistries/finalizers,verbs=update
type ImageRegistryReconciler struct {
	client.Client
	chain *reconciler.Chain[*sreportalv1alpha1.ImageRegistry, imageregistrychain.ChainData]

	imageStore domainimage.ImageWriter
}

// NewImageRegistryReconciler builds an ImageRegistryReconciler wired with the
// handler chain.
//
// baseCtx is the manager's root context — propagated to the resolve handler so
// async registry-lookup goroutines are cancelled on operator shutdown.
func NewImageRegistryReconciler(
	c client.Client,
	imageStore domainimage.ImageWriter,
	registryClient domainimageregistry.Client,
	hostLimiter *registry.HostLimiter,
	baseCtx context.Context,
) *ImageRegistryReconciler {
	handlers := []reconciler.Handler[*sreportalv1alpha1.ImageRegistry, imageregistrychain.ChainData]{
		imageregistrychain.NewValidateSpecHandler(c),
		imageregistrychain.NewSelectDueImagesHandler(),
		// Early readstore pass: populate immediately from current spec+status so
		// readers see up-to-date data without waiting for the registry lookup.
		imageregistrychain.NewUpdateReadstoreHandler(imageStore),
		imageregistrychain.NewResolveLatestVersionsHandler(registryClient, hostLimiter, c, baseCtx),
		// Second readstore pass: only runs when the lookup produced new versions.
		imageregistrychain.NewUpdateReadstoreIfResolvedHandler(imageStore),
		imageregistrychain.NewUpdateStatusHandler(c),
	}

	return &ImageRegistryReconciler{
		Client:     c,
		chain:      reconciler.NewChain(handlers...),
		imageStore: imageStore,
	}
}

// Reconcile is the main reconciliation loop entry point.
func (r *ImageRegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	var ir sreportalv1alpha1.ImageRegistry
	if err := r.Get(ctx, req.NamespacedName, &ir); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.V(1).Info("reconciling ImageRegistry",
		"name", ir.Name,
		"portal", ir.Spec.PortalRef,
		"host", ir.Spec.Host,
		"namespace", ir.Spec.Namespace,
	)

	// Handle deletion — run finalizer cleanup.
	if !ir.DeletionTimestamp.IsZero() {
		return r.handleFinalizer(ctx, &ir)
	}

	// Ensure finalizer is registered.
	if !controllerutil.ContainsFinalizer(&ir, finalizerName) {
		controllerutil.AddFinalizer(&ir, finalizerName)
		if err := r.Update(ctx, &ir); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		// Re-fetch after update.
		if err := r.Get(ctx, req.NamespacedName, &ir); err != nil {
			return ctrl.Result{}, err
		}
	}

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, imageregistrychain.ChainData]{
		Resource: &ir,
	}

	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation chain failed")
		metrics.ReconcileTotal.WithLabelValues("imageregistry", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("imageregistry").Observe(time.Since(start).Seconds())
		// Return the error so controller-runtime applies its native exponential
		// backoff. The registry is protected by the running sync.Map (single-flight
		// per CR), the per-host HostLimiter, and isDue()'s 24h pacing — fast retries
		// won't cascade into registry calls.
		return ctrl.Result{}, err
	}

	metrics.ReconcileTotal.WithLabelValues("imageregistry", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("imageregistry").Observe(time.Since(start).Seconds())

	// If a handler requested an early requeue (e.g. catch-up jitter), honour it.
	if rc.Data.RequeueAfter > 0 {
		return ctrl.Result{RequeueAfter: rc.Data.RequeueAfter}, nil
	}

	return ctrl.Result{RequeueAfter: nextInterval()}, nil
}

// handleFinalizer cleans up readstore contributions and Prometheus metrics, then
// removes the finalizer so Kubernetes can garbage-collect the CR.
func (r *ImageRegistryReconciler) handleFinalizer(ctx context.Context, ir *sreportalv1alpha1.ImageRegistry) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(ir, finalizerName) {
		// Clean readstore. Return on error so controller-runtime retries — the
		// finalizer is removed only after a successful cleanup. The current
		// in-memory store cannot fail outside of context cancellation, but a
		// future persistent store (Redis, BoltDB, …) would leak entries if we
		// swallowed the error here.
		if err := r.imageStore.RemoveForNamespace(ctx, ir.Spec.PortalRef, ir.Spec.Host, ir.Spec.Namespace); err != nil {
			return ctrl.Result{}, fmt.Errorf("finalizer: remove readstore contributions: %w", err)
		}

		// Clean Prometheus metrics.
		metrics.ResetImageRegistryMetrics(ir.Spec.PortalRef, ir.Spec.Host, ir.Spec.Namespace)

		controllerutil.RemoveFinalizer(ir, finalizerName)
		if err := r.Update(ctx, ir); err != nil {
			return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
		}
	}
	return ctrl.Result{}, nil
}

// nextInterval returns the default requeue duration with ±jitter applied.
// The result is in [defaultRequeueInterval - requeueJitter, defaultRequeueInterval + requeueJitter].
func nextInterval() time.Duration {
	// rand.N returns a value in [0, requeueJitter*2).
	jitter := rand.N(requeueJitter * 2)
	// Shift by -requeueJitter so the range is [-requeueJitter, +requeueJitter).
	return defaultRequeueInterval - requeueJitter + jitter
}

// SetupWithManager registers the controller and installs the field indexer.
func (r *ImageRegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Index ImageRegistry by spec.portalRef so we can efficiently look up all
	// CRs belonging to a given portal.
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&sreportalv1alpha1.ImageRegistry{},
		portalRefField,
		func(obj client.Object) []string {
			ir, ok := obj.(*sreportalv1alpha1.ImageRegistry)
			if !ok || ir.Spec.PortalRef == "" {
				return nil
			}
			return []string{ir.Spec.PortalRef}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.ImageRegistry{}).
		Named("imageregistry").
		Complete(r)
}
