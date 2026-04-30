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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// nolint:unused
var imageinventorylog = log.Default().WithName("imageinventory-resource")

// SetupImageInventoryWebhookWithManager registers the validation webhook for ImageInventory in the manager.
func SetupImageInventoryWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &sreportalv1alpha1.ImageInventory{}).
		WithValidator(&ImageInventoryCustomValidator{client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha1-imageinventory,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=imageinventories,verbs=create;update,versions=v1alpha1,name=vimageinventory-v1alpha1.kb.io,admissionReviewVersions=v1

// ImageInventoryCustomValidator validates the ImageInventory resource when it is created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ImageInventoryCustomValidator struct {
	client client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ImageInventory.
func (v *ImageInventoryCustomValidator) ValidateCreate(ctx context.Context, obj *sreportalv1alpha1.ImageInventory) (admission.Warnings, error) {
	imageinventorylog.Info("Validation for ImageInventory upon creation", "name", obj.GetName())

	return v.validatePortalRef(ctx, obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ImageInventory.
func (v *ImageInventoryCustomValidator) ValidateUpdate(ctx context.Context, _, newObj *sreportalv1alpha1.ImageInventory) (admission.Warnings, error) {
	imageinventorylog.Info("Validation for ImageInventory upon update", "name", newObj.GetName())

	return v.validatePortalRef(ctx, newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ImageInventory.
func (v *ImageInventoryCustomValidator) ValidateDelete(_ context.Context, obj *sreportalv1alpha1.ImageInventory) (admission.Warnings, error) {
	imageinventorylog.Info("Validation for ImageInventory upon deletion", "name", obj.GetName())

	return nil, nil
}

// validatePortalRef checks that the referenced portal exists.
func (v *ImageInventoryCustomValidator) validatePortalRef(ctx context.Context, obj *sreportalv1alpha1.ImageInventory) (admission.Warnings, error) {
	if obj.Spec.PortalRef == "" {
		return nil, fmt.Errorf("spec.portalRef is required")
	}

	var portal sreportalv1alpha1.Portal
	key := types.NamespacedName{
		Name:      obj.Spec.PortalRef,
		Namespace: obj.Namespace,
	}

	if err := v.client.Get(ctx, key, &portal); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("referenced portal %q not found in namespace %q", obj.Spec.PortalRef, obj.Namespace)
		}
		return nil, fmt.Errorf("failed to check portal reference: %w", err)
	}

	return nil, nil
}
