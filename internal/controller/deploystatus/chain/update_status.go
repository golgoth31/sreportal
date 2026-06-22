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

// UpdateStatusHandler projects the computed deploy-lag results into the CR's
// STATUS (Status.Services), preserving the status of services not re-checked
// this cycle, and writes only via client.Status().Update. It never writes the
// Spec — observed state belongs in Status, and writing Spec on every reconcile
// would bump metadata.generation and re-trigger the controller (reconcile loop).
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

	// Index this cycle's computed entries by key.
	computedByKey := make(map[string]ComputedEntry, len(rc.Data.Computed))
	for _, c := range rc.Data.Computed {
		computedByKey[c.Key] = c
	}

	// Preserve the status of services not re-checked this cycle (keeps
	// LastCheckedAt across cycles so SelectDue pacing works).
	prevStatusByKey := make(map[string]sreportalv1alpha1.DeployStatusEntry, len(cr.Status.Services))
	for _, s := range cr.Status.Services {
		prevStatusByKey[s.Key] = s
	}

	now := h.now()

	// Rebuild Status.Services from the controller-managed Spec.Services input:
	// every tracked service gets a status entry — freshly computed if processed
	// this cycle, otherwise the preserved prior status (or a bare entry).
	statusServices := make([]sreportalv1alpha1.DeployStatusEntry, 0, len(cr.Spec.Services))
	for _, in := range cr.Spec.Services {
		if c, ok := computedByKey[in.Key]; ok {
			statusServices = append(statusServices, sreportalv1alpha1.DeployStatusEntry{
				Key:              in.Key,
				Workload:         in.Workload,
				Image:            in.Image,
				SourceRepo:       c.SourceRepo,
				DeployedRef:      in.DeployedRef,
				DefaultBranch:    c.DefaultBranch,
				AheadBy:          c.AheadBy,
				PendingCommits:   toCRDCommits(c.PendingCommits),
				PendingTruncated: c.PendingTrunc,
				DeployRunURL:     c.DeployRunURL,
				State:            c.State,
				Error:            c.Error,
				LastCheckedAt:    now,
			})
			continue
		}
		if prev, ok := prevStatusByKey[in.Key]; ok {
			statusServices = append(statusServices, prev)
			continue
		}
		// Tracked but never checked yet: carry identity only.
		statusServices = append(statusServices, sreportalv1alpha1.DeployStatusEntry{
			Key:         in.Key,
			Workload:    in.Workload,
			Image:       in.Image,
			SourceRepo:  in.SourceRepo,
			DeployedRef: in.DeployedRef,
		})
	}

	cr.Status.Services = statusServices
	cr.Status.ServiceCount = len(statusServices)
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
// Maps forge.Commit.SHA (uppercase) → DeployStatusCommit.Sha (lowercase, JSON tag).
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
