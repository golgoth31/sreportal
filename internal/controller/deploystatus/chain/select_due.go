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
	"time"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const defaultRefreshInterval = 5 * time.Minute

// SelectDueHandler builds the Due work list from the CR's Spec.Services whose
// LastCheckedAt is older than the configured refreshInterval (default 5m).
type SelectDueHandler struct {
	refreshInterval time.Duration
	now             func() time.Time
}

// NewSelectDueHandler constructs a SelectDueHandler.
// refreshInterval of 0 defaults to 5 minutes.
func NewSelectDueHandler(refreshInterval time.Duration) *SelectDueHandler {
	if refreshInterval == 0 {
		refreshInterval = defaultRefreshInterval
	}
	return &SelectDueHandler{refreshInterval: refreshInterval, now: time.Now}
}

// Handle implements reconciler.Handler.
func (h *SelectDueHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]) error {
	rc.Data.Due = selectDue(rc.Resource.Spec.Services, h.refreshInterval, h.now())
	return nil
}

// selectDue returns WorkItems for services that need a forge check this cycle.
// A service is due when it has never been checked, or its LastCheckedAt is
// older than the refresh interval.
func selectDue(svcs []sreportalv1alpha1.DeployStatusEntry, refresh time.Duration, now time.Time) []WorkItem {
	var out []WorkItem
	for _, s := range svcs {
		if !s.LastCheckedAt.IsZero() && now.Sub(s.LastCheckedAt.Time) < refresh {
			// Checked recently — skip this cycle.
			continue
		}
		out = append(out, WorkItem{
			Key:               s.Key,
			Image:             s.Image,
			WorkloadKind:      s.Workload.Kind,
			WorkloadNamespace: s.Workload.Namespace,
			WorkloadName:      s.Workload.Name,
			WorkloadContainer: s.Workload.Container,
			SourceURL:         s.SourceRepo,
			DeployedRef:       s.DeployedRef,
		})
	}
	return out
}
