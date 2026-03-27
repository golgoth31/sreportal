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

package source

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// BuildPortalIndexHandler lists Portal CRs and builds a lookup index.
// If no local portals exist, rc.Data.Index remains nil and subsequent handlers
// must guard against this.
type BuildPortalIndexHandler struct {
	client client.Client
}

// NewBuildPortalIndexHandler creates a new BuildPortalIndexHandler.
func NewBuildPortalIndexHandler(c client.Client) *BuildPortalIndexHandler {
	return &BuildPortalIndexHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *BuildPortalIndexHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[struct{}, ChainData]) error {
	logger := log.FromContext(ctx).WithName("build-portal-index")

	var portalList sreportalv1alpha1.PortalList
	if err := h.client.List(ctx, &portalList); err != nil {
		logger.Error(err, "failed to list Portal resources")
		return err
	}

	if len(portalList.Items) == 0 {
		logger.Info("no portals found, skipping reconciliation")
		return nil
	}

	idx := &PortalIndex{
		ByName: make(map[string]*sreportalv1alpha1.Portal, len(portalList.Items)),
		Local:  make([]*sreportalv1alpha1.Portal, 0),
	}

	for i := range portalList.Items {
		p := &portalList.Items[i]
		idx.ByName[p.Name] = p

		if p.Spec.Remote != nil {
			logger.V(1).Info("skipping remote portal for source collection", "name", p.Name, "url", p.Spec.Remote.URL)
			continue
		}

		if !p.Spec.Features.IsDNSEnabled() {
			logger.V(1).Info("skipping portal with DNS feature disabled", "name", p.Name)
			continue
		}

		idx.Local = append(idx.Local, p)
		if p.Spec.Main {
			idx.Main = p
		}
	}

	if idx.Main == nil {
		if len(idx.Local) > 0 {
			idx.Main = idx.Local[0]
			logger.Info("no main portal found, using first local portal as fallback", "name", idx.Main.Name)
		} else {
			logger.Info("no local portals found, skipping source reconciliation")
			return nil
		}
	}

	rc.Data.Index = idx
	return nil
}
