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
	"strings"

	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
)

// nolint:unused
var releaselog = log.Default().WithName("release-resource")

// SetupReleaseWebhookWithManager registers the validation webhook for Release in the manager.
func SetupReleaseWebhookWithManager(mgr ctrl.Manager, allowedTypes []config.ReleaseTypeConfig) error {
	names := make([]string, len(allowedTypes))
	for i, t := range allowedTypes {
		names[i] = t.Name
	}
	return ctrl.NewWebhookManagedBy(mgr, &sreportalv1alpha1.Release{}).
		WithValidator(&ReleaseCustomValidator{allowedTypes: names}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha1-release,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=releases,verbs=create;update,versions=v1alpha1,name=vrelease-v1alpha1.kb.io,admissionReviewVersions=v1

// ReleaseCustomValidator validates the Release resource when it is created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ReleaseCustomValidator struct {
	allowedTypes []string
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Release.
func (v *ReleaseCustomValidator) ValidateCreate(_ context.Context, obj *sreportalv1alpha1.Release) (admission.Warnings, error) {
	releaselog.Info("Validation for Release upon creation", "name", obj.GetName())

	return v.validateEntryTypes(obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Release.
func (v *ReleaseCustomValidator) ValidateUpdate(_ context.Context, _, newObj *sreportalv1alpha1.Release) (admission.Warnings, error) {
	releaselog.Info("Validation for Release upon update", "name", newObj.GetName())

	return v.validateEntryTypes(newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Release.
func (v *ReleaseCustomValidator) ValidateDelete(_ context.Context, obj *sreportalv1alpha1.Release) (admission.Warnings, error) {
	releaselog.Info("Validation for Release upon deletion", "name", obj.GetName())

	return nil, nil
}

// validateEntryTypes checks that all entry types are in the allowed types list.
func (v *ReleaseCustomValidator) validateEntryTypes(obj *sreportalv1alpha1.Release) (admission.Warnings, error) {
	if len(v.allowedTypes) == 0 {
		return nil, nil
	}

	var errs []string
	for i, entry := range obj.Spec.Entries {
		if err := domainrelease.ValidateType(entry.Type, v.allowedTypes); err != nil {
			errs = append(errs, fmt.Sprintf("spec.entries[%d].type: %v", i, err))
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	return nil, nil
}
