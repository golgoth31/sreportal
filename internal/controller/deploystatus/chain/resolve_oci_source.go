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
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ResolveOCISourceHandler parses each WorkItem's OCI source URL and matches it
// against the configured forges. Items with an unmatched or unparseable source
// URL become "unresolved" ComputedEntries and are removed from the Due set.
type ResolveOCISourceHandler struct {
	forgesByHost map[string]config.ForgeConfig
}

// NewResolveOCISourceHandler constructs a ResolveOCISourceHandler.
func NewResolveOCISourceHandler(forges []config.ForgeConfig) *ResolveOCISourceHandler {
	m := make(map[string]config.ForgeConfig, len(forges))
	for _, f := range forges {
		m[f.Host] = f
	}
	return &ResolveOCISourceHandler{forgesByHost: m}
}

// Handle implements reconciler.Handler.
func (h *ResolveOCISourceHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]) error {
	kept := rc.Data.Due[:0]
	for _, wi := range rc.Data.Due {
		if wi.SourceURL == "" {
			rc.Data.Computed = append(rc.Data.Computed, unresolvedEntry(wi, "no OCI source label"))
			continue
		}
		ref, err := forge.ParseSourceURL(wi.SourceURL)
		if err != nil {
			rc.Data.Computed = append(rc.Data.Computed, unresolvedEntry(wi, "unparseable source URL: "+err.Error()))
			continue
		}
		if _, ok := h.forgesByHost[ref.Host]; !ok {
			rc.Data.Computed = append(rc.Data.Computed, unresolvedEntry(wi, "no forge configured for host "+ref.Host))
			continue
		}
		wi.Workload = ref
		kept = append(kept, wi)
	}
	rc.Data.Due = kept
	return nil
}

func unresolvedEntry(wi WorkItem, msg string) ComputedEntry {
	return ComputedEntry{
		Key:         wi.Key,
		Image:       wi.Image,
		SourceRepo:  wi.SourceURL,
		DeployedRef: wi.DeployedRef,
		State:       stateUnresolved,
		Error:       msg,
	}
}
