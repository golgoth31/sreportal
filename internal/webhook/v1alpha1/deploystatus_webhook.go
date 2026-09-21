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

package v1alpha1

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
)

// nolint:unused
var deploystatuslog = log.Default().WithName("deploystatus-resource")

// SetupDeployStatusWebhookWithManager registers the validation webhook for DeployStatus.
func SetupDeployStatusWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &sreportalv1alpha1.DeployStatus{}).
		WithValidator(&DeployStatusCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha1-deploystatus,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=deploystatuses,verbs=create;update,versions=v1alpha1,name=vdeploystatus-v1alpha1.kb.io,admissionReviewVersions=v1

// DeployStatusCustomValidator validates the DeployStatus resource. The CR is
// controller-managed (Spec.Services written by the imageInventory projection,
// Status by the deploy-status controller). The webhook enforces structural
// invariants so a malformed CR cannot poison downstream readers.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen
// from generating DeepCopy methods, as this struct is used only for temporary
// operations and does not need to be deeply copied.
type DeployStatusCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator.
func (v *DeployStatusCustomValidator) ValidateCreate(_ context.Context, obj *sreportalv1alpha1.DeployStatus) (admission.Warnings, error) {
	deploystatuslog.Info("Validation for DeployStatus upon creation", "name", obj.GetName())
	return nil, validateDeployStatus(obj)
}

// ValidateUpdate implements webhook.CustomValidator.
func (v *DeployStatusCustomValidator) ValidateUpdate(_ context.Context, _, newObj *sreportalv1alpha1.DeployStatus) (admission.Warnings, error) {
	deploystatuslog.Info("Validation for DeployStatus upon update", "name", newObj.GetName())
	return nil, validateDeployStatus(newObj)
}

// ValidateDelete implements webhook.CustomValidator.
func (v *DeployStatusCustomValidator) ValidateDelete(_ context.Context, obj *sreportalv1alpha1.DeployStatus) (admission.Warnings, error) {
	deploystatuslog.Info("Validation for DeployStatus upon deletion", "name", obj.GetName())
	return nil, nil
}

// validStates is the set of allowed non-empty values for DeployStatusEntry.State.
var validStates = map[string]struct{}{
	"ok":         {},
	"behind":     {},
	"unresolved": {},
	"error":      {},
}

// validateDeployStatus runs the full set of invariant checks on the CR.
func validateDeployStatus(obj *sreportalv1alpha1.DeployStatus) error {
	if obj.Spec.PortalRef == "" {
		return fmt.Errorf("spec.portalRef is required")
	}
	if obj.Spec.Namespace == "" {
		return fmt.Errorf("spec.namespace is required")
	}
	for i, svc := range obj.Spec.Services {
		if svc.Key == "" {
			return fmt.Errorf("spec.services[%d].key is required", i)
		}
		if svc.State != "" {
			if _, ok := validStates[svc.State]; !ok {
				return fmt.Errorf("spec.services[%d].state %q must be one of ok|behind|unresolved|error", i, svc.State)
			}
		}
	}
	return nil
}
