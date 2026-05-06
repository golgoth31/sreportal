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

	"github.com/google/go-containerregistry/pkg/name"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
)

// nolint:unused
var imageregistrylog = log.Default().WithName("imageregistry-resource")

// SetupImageRegistryWebhookWithManager registers the validation webhook for ImageRegistry.
func SetupImageRegistryWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &sreportalv1alpha1.ImageRegistry{}).
		WithValidator(&ImageRegistryCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha1-imageregistry,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=imageregistries,verbs=create;update,versions=v1alpha1,name=vimageregistry-v1alpha1.kb.io,admissionReviewVersions=v1

// ImageRegistryCustomValidator validates the ImageRegistry resource. The CR
// itself is controller-managed in v1, but the webhook still enforces basic
// invariants (host parses, ChangeType ↔ OriginalImage coherence) so a
// malformed CR cannot poison downstream readers.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen
// from generating DeepCopy methods, as this struct is used only for temporary
// operations and does not need to be deeply copied.
type ImageRegistryCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator.
func (v *ImageRegistryCustomValidator) ValidateCreate(_ context.Context, obj *sreportalv1alpha1.ImageRegistry) (admission.Warnings, error) {
	imageregistrylog.Info("Validation for ImageRegistry upon creation", "name", obj.GetName())
	return nil, validateImageRegistry(obj)
}

// ValidateUpdate implements webhook.CustomValidator.
func (v *ImageRegistryCustomValidator) ValidateUpdate(_ context.Context, _, newObj *sreportalv1alpha1.ImageRegistry) (admission.Warnings, error) {
	imageregistrylog.Info("Validation for ImageRegistry upon update", "name", newObj.GetName())
	return nil, validateImageRegistry(newObj)
}

// ValidateDelete implements webhook.CustomValidator.
func (v *ImageRegistryCustomValidator) ValidateDelete(_ context.Context, obj *sreportalv1alpha1.ImageRegistry) (admission.Warnings, error) {
	imageregistrylog.Info("Validation for ImageRegistry upon deletion", "name", obj.GetName())
	return nil, nil
}

// validateImageRegistry runs the full set of invariant checks documented in
// plan §3.4.
func validateImageRegistry(obj *sreportalv1alpha1.ImageRegistry) error {
	if obj.Spec.Host == "" {
		return fmt.Errorf("spec.host is required")
	}
	if _, err := name.NewRegistry(obj.Spec.Host); err != nil {
		return fmt.Errorf("spec.host %q is not a valid registry: %w", obj.Spec.Host, err)
	}
	if obj.Spec.PortalRef == "" {
		return fmt.Errorf("spec.portalRef is required")
	}
	if obj.Spec.Namespace == "" {
		return fmt.Errorf("spec.namespace is required")
	}

	for i, e := range obj.Spec.Images {
		if e.Key == "" {
			return fmt.Errorf("spec.images[%d].key is required", i)
		}
		if e.MutatedImage == "" {
			return fmt.Errorf("spec.images[%d].mutatedImage is required", i)
		}
		switch e.ChangeType {
		case "none", "mutated":
			if e.OriginalImage == "" {
				return fmt.Errorf("spec.images[%d]: changeType=%q requires originalImage", i, e.ChangeType)
			}
		case "injected":
			if e.OriginalImage != "" {
				return fmt.Errorf("spec.images[%d]: changeType=injected forbids originalImage", i)
			}
		default:
			return fmt.Errorf("spec.images[%d].changeType %q must be one of none|mutated|injected", i, e.ChangeType)
		}
	}
	return nil
}
