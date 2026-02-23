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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var portallog = logf.Log.WithName("portal-resource")

// SetupPortalWebhookWithManager registers the webhook for Portal in the manager.
func SetupPortalWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &sreportalv1alpha1.Portal{}).
		WithValidator(&PortalCustomValidator{}).
		WithDefaulter(&PortalCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-sreportal-io-v1alpha1-portal,mutating=true,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=portals,verbs=create;update,versions=v1alpha1,name=mportal-v1alpha1.kb.io,admissionReviewVersions=v1

// PortalCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Portal when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type PortalCustomDefaulter struct{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Portal.
func (d *PortalCustomDefaulter) Default(_ context.Context, obj *sreportalv1alpha1.Portal) error {
	portallog.Info("Defaulting for Portal", "name", obj.GetName())

	// Set subPath to name if not specified
	if obj.Spec.SubPath == "" {
		obj.Spec.SubPath = obj.Name
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha1-portal,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=portals,verbs=create;update,versions=v1alpha1,name=vportal-v1alpha1.kb.io,admissionReviewVersions=v1

// PortalCustomValidator struct is responsible for validating the Portal resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type PortalCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Portal.
func (v *PortalCustomValidator) ValidateCreate(_ context.Context, obj *sreportalv1alpha1.Portal) (admission.Warnings, error) {
	portallog.Info("Validation for Portal upon creation", "name", obj.GetName())

	return v.validatePortal(obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Portal.
func (v *PortalCustomValidator) ValidateUpdate(_ context.Context, _, newObj *sreportalv1alpha1.Portal) (admission.Warnings, error) {
	portallog.Info("Validation for Portal upon update", "name", newObj.GetName())

	return v.validatePortal(newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Portal.
func (v *PortalCustomValidator) ValidateDelete(_ context.Context, obj *sreportalv1alpha1.Portal) (admission.Warnings, error) {
	portallog.Info("Validation for Portal upon deletion", "name", obj.GetName())

	return nil, nil
}

// validatePortal validates the Portal spec.
func (v *PortalCustomValidator) validatePortal(obj *sreportalv1alpha1.Portal) (admission.Warnings, error) {
	// Rule: Remote cannot be set when Main is true
	if obj.Spec.Main && obj.Spec.Remote != nil {
		return nil, fmt.Errorf("spec.remote cannot be set when spec.main is true: the main portal must be local")
	}

	return nil, nil
}
