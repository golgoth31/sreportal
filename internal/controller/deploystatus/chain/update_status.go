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
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// UpdateStatusHandler stamps LastCheckedAt / state / aheadBy onto each processed
// Spec.Services entry, then updates the CR spec (via client.Update) and the
// status counters (via client.Status().Update).
type UpdateStatusHandler struct {
	client client.Client
	now    func() metav1.Time
}

// NewUpdateStatusHandler constructs an UpdateStatusHandler.
func NewUpdateStatusHandler(c client.Client) *UpdateStatusHandler {
	return &UpdateStatusHandler{
		client: c,
		now:    func() metav1.Time { return metav1.Now() },
	}
}

// Handle implements reconciler.Handler.
func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]) error {
	logger := log.FromContext(ctx).WithName("update-status")

	if len(rc.Data.Computed) == 0 {
		logger.V(2).Info("no computed entries this cycle, skipping status update")
		return nil
	}

	cr := rc.Resource

	// Index computed entries by key for O(1) lookup.
	computedByKey := make(map[string]ComputedEntry, len(rc.Data.Computed))
	for _, c := range rc.Data.Computed {
		computedByKey[c.Key] = c
	}

	now := h.now()

	// Stamp each matching Spec.Services entry.
	for i := range cr.Spec.Services {
		s := &cr.Spec.Services[i]
		c, ok := computedByKey[s.Key]
		if !ok {
			continue
		}
		s.LastCheckedAt = now
		s.State = c.State
		s.AheadBy = c.AheadBy
		s.Error = c.Error
		s.DefaultBranch = c.DefaultBranch
		s.DeployRunURL = c.DeployRunURL
		s.PendingTruncated = c.PendingTrunc
		s.PendingCommits = toCRDCommits(c.PendingCommits)
	}

	if err := h.client.Update(ctx, cr); err != nil {
		return fmt.Errorf("update DeployStatus spec: %w", err)
	}

	// Update status counters.
	cr.Status.ServiceCount = len(cr.Spec.Services)
	cr.Status.ObservedGeneration = cr.Generation

	if err := h.client.Status().Update(ctx, cr); err != nil {
		return fmt.Errorf("update DeployStatus status: %w", err)
	}

	logger.V(1).Info("status updated",
		"portal", cr.Spec.PortalRef,
		"namespace", cr.Spec.Namespace,
		"serviceCount", cr.Status.ServiceCount,
	)
	return nil
}

// toCRDCommits maps forge.Commit slice to the CRD DeployStatusCommit slice.
// Uses the same SHA→Sha mapping as toDomCommits but targets the CRD type.
func toCRDCommits(commits []forge.Commit) []sreportalv1alpha1.DeployStatusCommit {
	if len(commits) == 0 {
		return nil
	}
	out := make([]sreportalv1alpha1.DeployStatusCommit, 0, len(commits))
	for _, c := range commits {
		out = append(out, sreportalv1alpha1.DeployStatusCommit{
			Sha:     c.SHA,
			Message: c.Message,
			Author:  c.Author,
			Date:    metav1.NewTime(c.Date),
			URL:     c.URL,
		})
	}
	return out
}
