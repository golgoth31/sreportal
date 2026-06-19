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

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ForgeCompareHandler calls DefaultBranch then Compare on the forge for each
// resolved Due item. Per-entry errors set state="error" and never fail the chain.
type ForgeCompareHandler struct {
	clientFor func(host string) forge.Client
}

// NewForgeCompareHandler constructs a ForgeCompareHandler.
func NewForgeCompareHandler(clientFor func(host string) forge.Client) *ForgeCompareHandler {
	return &ForgeCompareHandler{clientFor: clientFor}
}

// Handle implements reconciler.Handler.
func (h *ForgeCompareHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]) error {
	logger := log.FromContext(ctx).WithName("forge-compare")

	for _, wi := range rc.Data.Due {
		entry := ComputedEntry{
			Key:         wi.Key,
			Image:       wi.Image,
			SourceRepo:  wi.SourceURL,
			DeployedRef: wi.DeployedRef,
		}

		cl := h.clientFor(wi.Workload.Host)

		branch, err := cl.DefaultBranch(ctx, wi.Workload)
		if err != nil {
			logger.V(1).Info("default branch lookup failed", "key", wi.Key, "err", err)
			entry.State = "error"
			entry.Error = err.Error()
			rc.Data.Computed = append(rc.Data.Computed, entry)
			continue
		}
		entry.DefaultBranch = branch

		cr, err := cl.Compare(ctx, wi.Workload, wi.DeployedRef, branch)
		if err != nil {
			logger.V(1).Info("forge compare failed", "key", wi.Key, "err", err)
			entry.State = "error"
			entry.Error = err.Error()
			rc.Data.Computed = append(rc.Data.Computed, entry)
			continue
		}

		entry.AheadBy = cr.AheadBy
		entry.PendingCommits, entry.PendingTrunc = ComputeLag(cr)
		entry.State = StateFor(cr.AheadBy)
		rc.Data.Computed = append(rc.Data.Computed, entry)
	}
	return nil
}
