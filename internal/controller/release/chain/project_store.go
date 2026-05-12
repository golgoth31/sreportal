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

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ProjectStoreHandler converts the Release CR's entries into domain views and
// pushes them to the ReleaseWriter.
type ProjectStoreHandler struct {
	releaseWriter domainrelease.ReleaseWriter
}

// NewProjectStoreHandler creates a new ProjectStoreHandler.
func NewProjectStoreHandler(w domainrelease.ReleaseWriter) *ProjectStoreHandler {
	return &ProjectStoreHandler{releaseWriter: w}
}

// Handle writes the entry views to the read store. A nil writer is a no-op.
func (h *ProjectStoreHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Release, ChainData]) error {
	if h.releaseWriter == nil {
		return nil
	}

	rel := rc.Resource
	views := EntriesToViews(rel.Spec.Entries, rel.Spec.PortalRef, rc.Data.Day)
	if err := h.releaseWriter.Replace(ctx, rc.Data.ResourceKey, views); err != nil {
		return fmt.Errorf("write release store: %w", err)
	}
	return nil
}

// EntriesToViews converts CRD release entries into domain read model views.
func EntriesToViews(entries []sreportalv1alpha1.ReleaseEntry, portalRef, day string) []domainrelease.EntryView {
	views := make([]domainrelease.EntryView, 0, len(entries))
	for _, e := range entries {
		views = append(views, domainrelease.EntryView{
			PortalRef: portalRef,
			Day:       day,
			Type:      e.Type,
			Version:   e.Version,
			Origin:    e.Origin,
			Date:      e.Date.Time,
			Author:    e.Author,
			Message:   e.Message,
			Link:      e.Link,
		})
	}
	return views
}
