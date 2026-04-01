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

package component

import (
	"context"
	"time"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domaincomponent "github.com/golgoth31/sreportal/internal/domain/component"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// MergeDailyStatusHandler merges the computed status into the daily worst-status history.
type MergeDailyStatusHandler struct {
	now func() time.Time
}

// NewMergeDailyStatusHandler creates a new MergeDailyStatusHandler.
// The now function is used to determine the current UTC date (inject for tests).
func NewMergeDailyStatusHandler(now func() time.Time) *MergeDailyStatusHandler {
	return &MergeDailyStatusHandler{now: now}
}

// Handle implements reconciler.Handler.
func (h *MergeDailyStatusHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Component, ChainData]) error {
	comp := rc.Resource
	today := h.now().UTC().Format("2006-01-02")

	// Convert CRD slice → domain slice
	domainHistory := make([]domaincomponent.DailyStatus, len(comp.Status.DailyWorstStatus))
	for i, entry := range comp.Status.DailyWorstStatus {
		domainHistory[i] = domaincomponent.DailyStatus{
			Date:        entry.Date,
			WorstStatus: domaincomponent.ComponentStatus(entry.WorstStatus),
		}
	}

	// Backfill missing days with "operational" to guarantee 30 consecutive entries
	domainHistory = domaincomponent.BackfillMissingDays(domainHistory, today, DailyStatusWindowDays)

	// Merge + prune via pure domain function
	merged := domaincomponent.MergeDailyWorst(
		domainHistory,
		today,
		domaincomponent.ComponentStatus(rc.Data.ComputedStatus),
		DailyStatusWindowDays,
	)

	// Convert back to CRD slice
	comp.Status.DailyWorstStatus = make([]sreportalv1alpha1.DailyComponentStatus, len(merged))
	for i, entry := range merged {
		comp.Status.DailyWorstStatus[i] = sreportalv1alpha1.DailyComponentStatus{
			Date:        entry.Date,
			WorstStatus: sreportalv1alpha1.ComputedComponentStatus(entry.WorstStatus),
		}
	}

	return nil
}
