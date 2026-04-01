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
	"fmt"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domaincomponent "github.com/golgoth31/sreportal/internal/domain/component"
	domainincident "github.com/golgoth31/sreportal/internal/domain/incident"
	domainmaint "github.com/golgoth31/sreportal/internal/domain/maintenance"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ComputeStatusHandler computes the effective status considering active maintenances and incidents.
type ComputeStatusHandler struct {
	maintenanceReader domainmaint.MaintenanceReader
	incidentLister    client.Reader
}

// NewComputeStatusHandler creates a new ComputeStatusHandler.
func NewComputeStatusHandler(maintenanceReader domainmaint.MaintenanceReader, incidentLister client.Reader) *ComputeStatusHandler {
	return &ComputeStatusHandler{maintenanceReader: maintenanceReader, incidentLister: incidentLister}
}

// Handle implements reconciler.Handler.
func (h *ComputeStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Component, ChainData]) error {
	comp := rc.Resource

	// Default: declared status promoted to computed type
	rc.Data.ComputedStatus = sreportalv1alpha1.ComputedComponentStatus(comp.Spec.Status)

	// Check if any in-progress maintenance overrides the status
	if h.maintenanceReader != nil {
		views, err := h.maintenanceReader.List(ctx, domainmaint.ListOptions{
			PortalRef: comp.Spec.PortalRef,
			Phase:     domainmaint.PhaseInProgress,
		})
		if err != nil {
			return fmt.Errorf("list maintenances: %w", err)
		}
		for _, v := range views {
			if v.AffectsComponent(comp.Spec.PortalRef, comp.Name) {
				rc.Data.ComputedStatus = sreportalv1alpha1.ComputedComponentStatus(v.AffectedStatus)
				break
			}
		}
	}

	// Check active incidents — incidents take precedence over maintenance.
	// Read Incident CRs directly from the K8s informer cache and compute
	// the phase from spec.updates to avoid a race with the Incident controller
	// updating the ReadStore.
	if h.incidentLister != nil {
		var incidentList sreportalv1alpha1.IncidentList
		if err := h.incidentLister.List(ctx, &incidentList,
			client.InNamespace(comp.Namespace),
			client.MatchingLabels{"sreportal.io/portal": comp.Spec.PortalRef},
		); err != nil {
			return fmt.Errorf("list incidents: %w", err)
		}

		var activeCount int
		worstStatus := string(rc.Data.ComputedStatus)

		for i := range incidentList.Items {
			inc := &incidentList.Items[i]

			// Compute the authoritative phase from spec.updates
			updates := make([]domainincident.UpdateView, 0, len(inc.Spec.Updates))
			for _, u := range inc.Spec.Updates {
				updates = append(updates, domainincident.UpdateView{
					Timestamp: u.Timestamp.Time,
					Phase:     domainincident.IncidentPhase(u.Phase),
					Message:   u.Message,
				})
			}
			computed, err := domainincident.ComputeStatus(updates)
			if err != nil {
				continue // skip incidents with no updates
			}
			if computed.CurrentPhase == domainincident.PhaseResolved {
				continue
			}
			if !slices.Contains(inc.Spec.Components, comp.Name) {
				continue
			}

			activeCount++
			incStatus := domainincident.SeverityToComponentStatus(domainincident.IncidentSeverity(inc.Spec.Severity))
			if domaincomponent.StatusSeverityRank(incStatus) > domaincomponent.StatusSeverityRank(worstStatus) {
				worstStatus = incStatus
			}
		}

		if activeCount > 0 {
			rc.Data.ComputedStatus = sreportalv1alpha1.ComputedComponentStatus(worstStatus)
		}
		rc.Data.ActiveIncidents = activeCount
	}

	return nil
}
