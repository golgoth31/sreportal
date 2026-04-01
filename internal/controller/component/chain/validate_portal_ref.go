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

package component

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	domaincomponent "github.com/golgoth31/sreportal/internal/domain/component"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ValidatePortalRefHandler verifies the referenced Portal exists.
type ValidatePortalRefHandler struct {
	client client.Client
}

// NewValidatePortalRefHandler creates a new ValidatePortalRefHandler.
func NewValidatePortalRefHandler(c client.Client) *ValidatePortalRefHandler {
	return &ValidatePortalRefHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *ValidatePortalRefHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Component, ChainData]) error {
	comp := rc.Resource
	var portal sreportalv1alpha1.Portal
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: comp.Namespace, Name: comp.Spec.PortalRef}, &portal); err != nil {
		if apierrors.IsNotFound(err) {
			_ = statusutil.SetConditionAndPatch(ctx, h.client, comp, "Ready", metav1.ConditionFalse, "PortalNotFound",
				fmt.Sprintf("portal %q not found", comp.Spec.PortalRef))
			return fmt.Errorf("portal %q not found: %w", comp.Spec.PortalRef, domaincomponent.ErrPortalNotFound)
		}
		return fmt.Errorf("get portal: %w", err)
	}
	return nil
}
