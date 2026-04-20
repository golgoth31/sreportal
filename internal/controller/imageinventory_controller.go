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
	"slices"

	sreportaliov1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ImageInventoryReconciler reconciles a ImageInventory object
type ImageInventoryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=sreportal.io,resources=imageinventories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=imageinventories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=imageinventories/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ImageInventory object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/reconcile
func (r *ImageInventoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var inv sreportaliov1alpha1.ImageInventory
	if err := r.Get(ctx, req.NamespacedName, &inv); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := validateImageInventorySpec(inv.Spec); err != nil {
		logger.Error(err, "invalid ImageInventory spec", "name", req.Name, "namespace", req.Namespace)
		r.setStatusConditionAndError(ctx, &inv, metav1.ConditionFalse, "InvalidSpec", err.Error())
		return ctrl.Result{}, nil
	}

	var portal sreportaliov1alpha1.Portal
	if err := r.Get(ctx, types.NamespacedName{Namespace: inv.Namespace, Name: inv.Spec.PortalRef}, &portal); err != nil {
		err = fmt.Errorf("portalRef %q not found in namespace %q: %w", inv.Spec.PortalRef, inv.Namespace, err)
		logger.Error(err, "invalid portalRef", "name", req.Name, "namespace", req.Namespace)
		r.setStatusConditionAndError(ctx, &inv, metav1.ConditionFalse, "PortalNotFound", err.Error())
		return ctrl.Result{}, nil
	}

	r.setStatusConditionAndError(ctx, &inv, metav1.ConditionTrue, "Ready", "")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ImageInventoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportaliov1alpha1.ImageInventory{}).
		Named("imageinventory").
		Complete(r)
}

func validateImageInventorySpec(spec sreportaliov1alpha1.ImageInventorySpec) error {
	if spec.PortalRef == "" {
		return field.Required(field.NewPath("spec", "portalRef"), "portalRef is required")
	}
	if spec.LabelSelector != "" {
		if _, err := labels.Parse(spec.LabelSelector); err != nil {
			return field.Invalid(field.NewPath("spec", "labelSelector"), spec.LabelSelector, err.Error())
		}
	}
	for _, kind := range spec.EffectiveWatchedKinds() {
		if !isSupportedWorkloadKind(kind) {
			return field.NotSupported(
				field.NewPath("spec", "watchedKinds"),
				kind,
				[]string{
					sreportaliov1alpha1.ImageInventoryKindDeployment,
					sreportaliov1alpha1.ImageInventoryKindStatefulSet,
					sreportaliov1alpha1.ImageInventoryKindDaemonSet,
					sreportaliov1alpha1.ImageInventoryKindCronJob,
					sreportaliov1alpha1.ImageInventoryKindJob,
				},
			)
		}
	}
	return nil
}

func isSupportedWorkloadKind(kind string) bool {
	return slices.Contains([]string{
		sreportaliov1alpha1.ImageInventoryKindDeployment,
		sreportaliov1alpha1.ImageInventoryKindStatefulSet,
		sreportaliov1alpha1.ImageInventoryKindDaemonSet,
		sreportaliov1alpha1.ImageInventoryKindCronJob,
		sreportaliov1alpha1.ImageInventoryKindJob,
	}, kind)
}

func (r *ImageInventoryReconciler) setStatusConditionAndError(
	ctx context.Context,
	inv *sreportaliov1alpha1.ImageInventory,
	status metav1.ConditionStatus,
	reason string,
	lastErr string,
) {
	base := inv.DeepCopy()
	inv.Status.ObservedGeneration = inv.GetGeneration()
	now := metav1.Now()
	inv.Status.LastScanTime = &now
	inv.Status.LastScanError = lastErr
	meta.SetStatusCondition(&inv.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             status,
		Reason:             reason,
		Message:            reason,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: inv.GetGeneration(),
	})
	_ = r.Status().Patch(ctx, inv, client.MergeFrom(base))
}
