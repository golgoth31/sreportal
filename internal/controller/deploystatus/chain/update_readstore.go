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
	dom "github.com/golgoth31/sreportal/internal/domain/deploystatus"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// UpdateReadStoreHandler projects computed entries into the deploy-status read store,
// replacing the previous contribution for this (portalRef, namespace).
type UpdateReadStoreHandler struct {
	store dom.Writer
}

// NewUpdateReadStoreHandler constructs an UpdateReadStoreHandler.
func NewUpdateReadStoreHandler(store dom.Writer) *UpdateReadStoreHandler {
	return &UpdateReadStoreHandler{store: store}
}

// Handle implements reconciler.Handler.
func (h *UpdateReadStoreHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]) error {
	logger := log.FromContext(ctx).WithName("update-readstore")

	entries := make([]dom.Entry, 0, len(rc.Data.Computed))
	for _, c := range rc.Data.Computed {
		entries = append(entries, dom.Entry{
			Key:              c.Key,
			Image:            c.Image,
			SourceRepo:       c.SourceRepo,
			DeployedRef:      c.DeployedRef,
			DefaultBranch:    c.DefaultBranch,
			AheadBy:          c.AheadBy,
			PendingCommits:   toDomCommits(c.PendingCommits),
			PendingTruncated: c.PendingTrunc,
			DeployRunURL:     c.DeployRunURL,
			State:            c.State,
			Error:            c.Error,
		})
	}

	spec := rc.Resource.Spec
	logger.V(1).Info("updating readstore",
		"portal", spec.PortalRef,
		"namespace", spec.Namespace,
		"entries", len(entries),
	)
	h.store.ReplaceForNamespace(spec.PortalRef, spec.Namespace, entries)
	return nil
}

// toDomCommits maps forge.Commit slice to domain deploystatus.Commit slice.
// Key field mapping: forge.Commit.SHA → dom.Commit.Sha (forge uses uppercase SHA,
// the domain read model uses lowercase Sha to match the CRD JSON tag).
func toDomCommits(commits []forge.Commit) []dom.Commit {
	if len(commits) == 0 {
		return nil
	}
	out := make([]dom.Commit, 0, len(commits))
	for _, c := range commits {
		out = append(out, dom.Commit{
			Sha:     c.SHA,
			Message: c.Message,
			Author:  c.Author,
			Date:    c.Date,
			URL:     c.URL,
		})
	}
	return out
}
