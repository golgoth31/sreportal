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

package image

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch

// DeploymentImageReconciler reconciles Deployment changes into the image inventory.
type DeploymentImageReconciler struct {
	handler *WorkloadHandler
}

// StatefulSetImageReconciler reconciles StatefulSet changes into the image inventory.
type StatefulSetImageReconciler struct {
	handler *WorkloadHandler
}

// DaemonSetImageReconciler reconciles DaemonSet changes into the image inventory.
type DaemonSetImageReconciler struct {
	handler *WorkloadHandler
}

// CronJobImageReconciler reconciles CronJob changes into the image inventory.
type CronJobImageReconciler struct {
	handler *WorkloadHandler
}

// JobImageReconciler reconciles Job changes into the image inventory.
type JobImageReconciler struct {
	handler *WorkloadHandler
}

// Reconcile implements reconcile.Reconciler for Deployment.
func (r *DeploymentImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj appsv1.Deployment
	if err := r.handler.client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: req.Namespace, Name: req.Name}
			return ctrl.Result{}, r.handler.HandleDelete(ctx, wk)
		}
		return ctrl.Result{}, fmt.Errorf("get Deployment: %w", err)
	}
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: obj.Namespace, Name: obj.Name}
	return ctrl.Result{}, r.handler.HandleUpsert(ctx, wk, obj.Spec.Template.Spec, labels.Set(obj.Labels))
}

// Reconcile implements reconcile.Reconciler for StatefulSet.
func (r *StatefulSetImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj appsv1.StatefulSet
	if err := r.handler.client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			wk := domainimage.WorkloadKey{Kind: "StatefulSet", Namespace: req.Namespace, Name: req.Name}
			return ctrl.Result{}, r.handler.HandleDelete(ctx, wk)
		}
		return ctrl.Result{}, fmt.Errorf("get StatefulSet: %w", err)
	}
	wk := domainimage.WorkloadKey{Kind: "StatefulSet", Namespace: obj.Namespace, Name: obj.Name}
	return ctrl.Result{}, r.handler.HandleUpsert(ctx, wk, obj.Spec.Template.Spec, labels.Set(obj.Labels))
}

// Reconcile implements reconcile.Reconciler for DaemonSet.
func (r *DaemonSetImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj appsv1.DaemonSet
	if err := r.handler.client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			wk := domainimage.WorkloadKey{Kind: "DaemonSet", Namespace: req.Namespace, Name: req.Name}
			return ctrl.Result{}, r.handler.HandleDelete(ctx, wk)
		}
		return ctrl.Result{}, fmt.Errorf("get DaemonSet: %w", err)
	}
	wk := domainimage.WorkloadKey{Kind: "DaemonSet", Namespace: obj.Namespace, Name: obj.Name}
	return ctrl.Result{}, r.handler.HandleUpsert(ctx, wk, obj.Spec.Template.Spec, labels.Set(obj.Labels))
}

// Reconcile implements reconcile.Reconciler for CronJob.
func (r *CronJobImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj batchv1.CronJob
	if err := r.handler.client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			wk := domainimage.WorkloadKey{Kind: "CronJob", Namespace: req.Namespace, Name: req.Name}
			return ctrl.Result{}, r.handler.HandleDelete(ctx, wk)
		}
		return ctrl.Result{}, fmt.Errorf("get CronJob: %w", err)
	}
	wk := domainimage.WorkloadKey{Kind: "CronJob", Namespace: obj.Namespace, Name: obj.Name}
	return ctrl.Result{}, r.handler.HandleUpsert(ctx, wk, obj.Spec.JobTemplate.Spec.Template.Spec, labels.Set(obj.Labels))
}

// Reconcile implements reconcile.Reconciler for Job.
func (r *JobImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj batchv1.Job
	if err := r.handler.client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			wk := domainimage.WorkloadKey{Kind: "Job", Namespace: req.Namespace, Name: req.Name}
			return ctrl.Result{}, r.handler.HandleDelete(ctx, wk)
		}
		return ctrl.Result{}, fmt.Errorf("get Job: %w", err)
	}
	wk := domainimage.WorkloadKey{Kind: "Job", Namespace: obj.Namespace, Name: obj.Name}
	return ctrl.Result{}, r.handler.HandleUpsert(ctx, wk, obj.Spec.Template.Spec, labels.Set(obj.Labels))
}

// SetupWorkloadReconcilersWithManager registers the five thin reconcilers with
// the controller manager. Each one watches a single workload kind and shares
// the passed WorkloadHandler.
func SetupWorkloadReconcilersWithManager(mgr ctrl.Manager, h *WorkloadHandler) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Named("deployment-image").
		Complete(&DeploymentImageReconciler{handler: h}); err != nil {
		return fmt.Errorf("setup deployment-image reconciler: %w", err)
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.StatefulSet{}).
		Named("statefulset-image").
		Complete(&StatefulSetImageReconciler{handler: h}); err != nil {
		return fmt.Errorf("setup statefulset-image reconciler: %w", err)
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.DaemonSet{}).
		Named("daemonset-image").
		Complete(&DaemonSetImageReconciler{handler: h}); err != nil {
		return fmt.Errorf("setup daemonset-image reconciler: %w", err)
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.CronJob{}).
		Named("cronjob-image").
		Complete(&CronJobImageReconciler{handler: h}); err != nil {
		return fmt.Errorf("setup cronjob-image reconciler: %w", err)
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		Named("job-image").
		Complete(&JobImageReconciler{handler: h}); err != nil {
		return fmt.Errorf("setup job-image reconciler: %w", err)
	}
	return nil
}
