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

	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// RebuildSourcesHandler ensures typed sources are built before collection.
type RebuildSourcesHandler struct {
	provider SourceProvider
}

// NewRebuildSourcesHandler creates a new RebuildSourcesHandler.
func NewRebuildSourcesHandler(provider SourceProvider) *RebuildSourcesHandler {
	return &RebuildSourcesHandler{provider: provider}
}

// Handle implements reconciler.Handler.
func (h *RebuildSourcesHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[struct{}, ChainData]) error {
	logger := log.FromContext(ctx).WithName("rebuild-sources")

	sources := h.provider.GetTypedSources()
	if len(sources) > 0 {
		rc.Data.TypedSources = sources
		return nil
	}

	logger.Info("no sources built, rebuilding")
	if err := h.provider.RebuildSources(ctx); err != nil {
		return err
	}

	rc.Data.TypedSources = h.provider.GetTypedSources()
	return nil
}
