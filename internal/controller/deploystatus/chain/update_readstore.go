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
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// UpdateReadStoreHandler projects the CR's full Status.Services into the
// deploy-status read store, replacing the previous contribution for this
// (portalRef, namespace).
//
// It deliberately reads from Status.Services (the complete, just-updated set —
// UpdateStatusHandler runs immediately before it in the chain) rather than from
// this cycle's computed (due) subset. This way a partial cycle still publishes
// the full set, and a "nothing due" cycle re-publishes the existing full Status
// (self-healing) instead of wiping the read store.
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

	services := rc.Resource.Status.Services
	entries := make([]dom.Entry, 0, len(services))
	for i := range services {
		entries = append(entries, statusEntryToDom(&services[i]))
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

// statusEntryToDom maps a CRD DeployStatusEntry (Status.Services) to the
// read-model dom.Entry, mapping every field.
func statusEntryToDom(s *sreportalv1alpha1.DeployStatusEntry) dom.Entry {
	return dom.Entry{
		Key: s.Key,
		Workload: dom.WorkloadRef{
			Kind:      s.Workload.Kind,
			Namespace: s.Workload.Namespace,
			Name:      s.Workload.Name,
			Container: s.Workload.Container,
		},
		Image:            s.Image,
		SourceRepo:       s.SourceRepo,
		DeployedRef:      s.DeployedRef,
		DefaultBranch:    s.DefaultBranch,
		AheadBy:          s.AheadBy,
		PendingCommits:   crdCommitsToDom(s.PendingCommits),
		PendingTruncated: s.PendingTruncated,
		DeployedAt:       s.DeployedAt.Time,
		DeployRunURL:     s.DeployRunURL,
		State:            s.State,
		Error:            s.Error,
		LastCheckedAt:    s.LastCheckedAt.Time,
	}
}

// crdCommitsToDom maps the CRD DeployStatusCommit slice to the read-model
// dom.Commit slice.
func crdCommitsToDom(commits []sreportalv1alpha1.DeployStatusCommit) []dom.Commit {
	if len(commits) == 0 {
		return nil
	}
	out := make([]dom.Commit, 0, len(commits))
	for _, c := range commits {
		out = append(out, dom.Commit{
			Sha:     c.Sha,
			Message: c.Message,
			Author:  c.Author,
			Date:    c.Date.Time,
			URL:     c.URL,
		})
	}
	return out
}
