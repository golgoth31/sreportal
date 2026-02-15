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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// PortalReconciler reconciles a Portal object
type PortalReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewPortalReconciler creates a new PortalReconciler
func NewPortalReconciler(c client.Client, scheme *runtime.Scheme) *PortalReconciler {
	return &PortalReconciler{
		Client: c,
		Scheme: scheme,
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

	// Update status
	portal.Status.Ready = true
	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "PortalConfigured",
		Message:            "Portal is fully configured",
		LastTransitionTime: metav1.Now(),
	}
	setPortalCondition(&portal.Status.Conditions, readyCondition)

	if err := r.Status().Update(ctx, &portal); err != nil {
		log.Error(err, "failed to update Portal status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
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
