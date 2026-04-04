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

	"github.com/golgoth31/sreportal/internal/log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var portallog = log.Default().WithName("portal-resource")

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

	// Default all feature flags to true
	if obj.Spec.Features == nil {
		obj.Spec.Features = &sreportalv1alpha1.PortalFeatures{}
	}

	trueVal := true

	if obj.Spec.Features.DNS == nil {
		obj.Spec.Features.DNS = &trueVal
	}

	if obj.Spec.Features.Releases == nil {
		obj.Spec.Features.Releases = &trueVal
	}

	if obj.Spec.Features.NetworkPolicy == nil {
		obj.Spec.Features.NetworkPolicy = &trueVal
	}

	if obj.Spec.Features.Alerts == nil {
		obj.Spec.Features.Alerts = &trueVal
	}

	if obj.Spec.Features.StatusPage == nil {
		obj.Spec.Features.StatusPage = &trueVal
	}

	defaultPortalAuthFields(obj.Spec.Auth)
	if obj.Spec.Features != nil {
		defaultPortalAuthFieldsForFeatures(obj.Spec.Features.Auth)
	}

	return nil
}

func defaultPortalAuthFields(a *sreportalv1alpha1.PortalAuthSpec) {
	if a == nil {
		return
	}
	if a.APIKey != nil && a.APIKey.Enabled {
		if a.APIKey.HeaderName == "" {
			a.APIKey.HeaderName = "X-API-Key"
		}
		if a.APIKey.SecretKey == "" {
			a.APIKey.SecretKey = "api-key"
		}
	}
	if a.JWT != nil {
		for i := range a.JWT.Issuers {
			if a.JWT.Issuers[i].RequiredClaims == nil {
				a.JWT.Issuers[i].RequiredClaims = map[string]string{}
			}
		}
	}
}

func defaultPortalAuthFieldsForFeatures(o *sreportalv1alpha1.PortalFeatureAuthOverrides) {
	if o == nil {
		return
	}
	defaultPortalAuthFields(o.DNS)
	defaultPortalAuthFields(o.Releases)
	defaultPortalAuthFields(o.NetworkPolicy)
	defaultPortalAuthFields(o.Alerts)
	defaultPortalAuthFields(o.StatusPage)
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

	if obj.Spec.Remote != nil {
		if obj.Spec.Auth == nil || !obj.Spec.Auth.Enabled() {
			return nil, fmt.Errorf("spec.auth with at least one enabled method is required when spec.remote is set")
		}
	}

	if err := validatePortalAuthSpec(obj.Spec.Auth); err != nil {
		return nil, err
	}
	if obj.Spec.Features != nil {
		if err := validatePortalFeatureAuthOverrides(obj.Spec.Features.Auth); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func validatePortalAuthSpec(p *sreportalv1alpha1.PortalAuthSpec) error {
	if p == nil {
		return nil
	}
	if p.JWT != nil && p.JWT.Enabled {
		if len(p.JWT.Issuers) == 0 {
			return fmt.Errorf("jwt.issuers is required when jwt.enabled is true")
		}
		for i, iss := range p.JWT.Issuers {
			if iss.IssuerURL == "" {
				return fmt.Errorf("jwt.issuers[%d].issuerURL is required", i)
			}
			if iss.JWKSURL == "" {
				return fmt.Errorf("jwt.issuers[%d].jwksURL is required", i)
			}
		}
	}
	if p.APIKey != nil && p.APIKey.Enabled && p.APIKey.SecretRef.Name == "" {
		return fmt.Errorf("apiKey.secretRef.name is required when apiKey.enabled is true")
	}
	return nil
}

func validatePortalFeatureAuthOverrides(o *sreportalv1alpha1.PortalFeatureAuthOverrides) error {
	if o == nil {
		return nil
	}
	if err := validatePortalAuthSpec(o.DNS); err != nil {
		return fmt.Errorf("features.auth.dns: %w", err)
	}
	if err := validatePortalAuthSpec(o.Releases); err != nil {
		return fmt.Errorf("features.auth.releases: %w", err)
	}
	if err := validatePortalAuthSpec(o.NetworkPolicy); err != nil {
		return fmt.Errorf("features.auth.networkPolicy: %w", err)
	}
	if err := validatePortalAuthSpec(o.Alerts); err != nil {
		return fmt.Errorf("features.auth.alerts: %w", err)
	}
	if err := validatePortalAuthSpec(o.StatusPage); err != nil {
		return fmt.Errorf("features.auth.statusPage: %w", err)
	}
	return nil
}
