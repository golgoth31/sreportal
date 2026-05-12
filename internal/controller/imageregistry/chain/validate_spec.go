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

package chain

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ValidateSpecHandler verifies that the mandatory spec fields are non-empty.
// It sets Ready=False with reason InvalidSpec if validation fails.
type ValidateSpecHandler struct {
	client client.Client
}

// NewValidateSpecHandler constructs a ValidateSpecHandler.
func NewValidateSpecHandler(c client.Client) *ValidateSpecHandler {
	return &ValidateSpecHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *ValidateSpecHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageRegistry, ChainData]) error {
	spec := rc.Resource.Spec
	var reason string
	switch {
	case spec.Host == "":
		reason = "spec.host is required"
	case spec.PortalRef == "":
		reason = "spec.portalRef is required"
	case spec.Namespace == "":
		reason = "spec.namespace is required"
	}
	if reason == "" {
		return nil
	}

	if patchErr := statusutil.SetConditionAndPatch(ctx, h.client, rc.Resource, ConditionTypeReady, metav1.ConditionFalse, ReasonInvalidSpec, reason); patchErr != nil {
		log.FromContext(ctx).WithName("validate-spec").Error(patchErr, "failed to patch Ready=False condition")
	}
	return fmt.Errorf("%w: %s", domainimageregistry.ErrInvalidSpec, reason)
}
