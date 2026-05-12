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
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ResolvePortalHandler loads the Portal referenced by the Release and short-circuits
// the chain when the releases feature is disabled on that portal.
type ResolvePortalHandler struct {
	client          client.Client
	requeueInterval time.Duration
}

// NewResolvePortalHandler creates a new ResolvePortalHandler.
func NewResolvePortalHandler(c client.Client, requeueInterval time.Duration) *ResolvePortalHandler {
	return &ResolvePortalHandler{client: c, requeueInterval: requeueInterval}
}

// Handle loads the portal and, when releases are disabled, sets RequeueAfter so the
// chain short-circuits without projecting to the store.
func (h *ResolvePortalHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Release, ChainData]) error {
	rel := rc.Resource
	logger := log.FromContext(ctx)

	var portal sreportalv1alpha1.Portal
	if err := h.client.Get(ctx, types.NamespacedName{Name: rel.Spec.PortalRef, Namespace: rel.Namespace}, &portal); err != nil {
		return fmt.Errorf("get portal %q: %w", rel.Spec.PortalRef, err)
	}

	if !portal.Spec.Features.IsReleasesEnabled() {
		logger.V(1).Info("releases feature disabled, skipping store projection",
			"day", rc.Data.Day, "portal", rel.Spec.PortalRef)
		rc.Result.RequeueAfter = h.requeueInterval
		return nil
	}

	rc.Data.Portal = &portal
	return nil
}
