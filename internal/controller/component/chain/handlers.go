// Package component contains Chain-of-Responsibility handlers for the Component controller.
package component

import (
	"context"
	"fmt"
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	domaincomponent "github.com/golgoth31/sreportal/internal/domain/component"
	domainincident "github.com/golgoth31/sreportal/internal/domain/incident"
	domainmaint "github.com/golgoth31/sreportal/internal/domain/maintenance"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ChainData holds shared state between handlers.
type ChainData struct {
	ComputedStatus  sreportalv1alpha1.ComputedComponentStatus
	ActiveIncidents int
}

// --- Handler 1: ValidatePortalRef ---

// ValidatePortalRefHandler verifies the referenced Portal exists.
type ValidatePortalRefHandler struct {
	client client.Client
}

// NewValidatePortalRefHandler creates a new ValidatePortalRefHandler.
func NewValidatePortalRefHandler(c client.Client) *ValidatePortalRefHandler {
	return &ValidatePortalRefHandler{client: c}
}

func (h *ValidatePortalRefHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Component, ChainData]) error {
	comp := rc.Resource
	var portal sreportalv1alpha1.Portal
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: comp.Namespace, Name: comp.Spec.PortalRef}, &portal); err != nil {
		if apierrors.IsNotFound(err) {
			_ = statusutil.SetConditionAndPatch(ctx, h.client, comp, "Ready", metav1.ConditionFalse, "PortalNotFound",
				fmt.Sprintf("portal %q not found", comp.Spec.PortalRef))
			return fmt.Errorf("portal %q not found: %w", comp.Spec.PortalRef, domaincomponent.ErrPortalNotFound)
		}
		return fmt.Errorf("get portal: %w", err)
	}
	return nil
}

// --- Handler 2: ComputeStatus ---

// ComputeStatusHandler computes the effective status considering active maintenances and incidents.
type ComputeStatusHandler struct {
	maintenanceReader domainmaint.MaintenanceReader
	incidentLister    client.Reader
}

// NewComputeStatusHandler creates a new ComputeStatusHandler.
func NewComputeStatusHandler(maintenanceReader domainmaint.MaintenanceReader, incidentLister client.Reader) *ComputeStatusHandler {
	return &ComputeStatusHandler{maintenanceReader: maintenanceReader, incidentLister: incidentLister}
}

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

// --- Handler 3: MergeDailyStatus ---

// DailyStatusWindowDays is the number of days in the sliding daily-status window.
const DailyStatusWindowDays = 30

// MergeDailyStatusHandler merges the computed status into the daily worst-status history.
type MergeDailyStatusHandler struct {
	now func() time.Time
}

// NewMergeDailyStatusHandler creates a new MergeDailyStatusHandler.
// The now function is used to determine the current UTC date (inject for tests).
func NewMergeDailyStatusHandler(now func() time.Time) *MergeDailyStatusHandler {
	return &MergeDailyStatusHandler{now: now}
}

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

// --- Handler 4: UpdateStatus ---

// UpdateStatusHandler patches the CR status, labels, and projects to the ReadStore.
// It also sets RequeueAfter to the next UTC midnight to ensure daily status entries.
type UpdateStatusHandler struct {
	client          client.Client
	componentWriter domaincomponent.ComponentWriter
	now             func() time.Time
}

// NewUpdateStatusHandler creates a new UpdateStatusHandler.
func NewUpdateStatusHandler(c client.Client, w domaincomponent.ComponentWriter, now func() time.Time) *UpdateStatusHandler {
	return &UpdateStatusHandler{client: c, componentWriter: w, now: now}
}

// requeueDuration returns the duration until the next UTC midnight.
func (h *UpdateStatusHandler) requeueDuration() time.Duration {
	return domaincomponent.DurationUntilNextMidnightUTC(h.now())
}

func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Component, ChainData]) error {
	comp := rc.Resource

	// Detect status change
	oldStatus := comp.Status.ComputedStatus
	comp.Status.ComputedStatus = rc.Data.ComputedStatus
	comp.Status.ActiveIncidents = rc.Data.ActiveIncidents
	statusChanged := oldStatus != rc.Data.ComputedStatus
	if statusChanged {
		now := metav1.Now()
		comp.Status.LastStatusChange = &now
	}

	// Set Ready condition + patch status
	if err := statusutil.SetConditionAndPatch(ctx, h.client, comp, "Ready", metav1.ConditionTrue, "Reconciled", "component reconciled"); err != nil {
		return err
	}

	// If ComputedStatus changed but condition didn't (already Ready), the status
	// subresource was not patched above. Persist it explicitly.
	if statusChanged {
		if err := h.client.Status().Update(ctx, comp); err != nil {
			return fmt.Errorf("update component computed status: %w", err)
		}
	}

	// Re-fetch to get fresh resourceVersion after status patch
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: comp.Namespace, Name: comp.Name}, comp); err != nil {
		return fmt.Errorf("re-fetch component: %w", err)
	}

	// Add portal label
	if comp.Labels == nil {
		comp.Labels = make(map[string]string)
	}
	if comp.Labels["sreportal.io/portal"] != comp.Spec.PortalRef {
		comp.Labels["sreportal.io/portal"] = comp.Spec.PortalRef
		if err := h.client.Update(ctx, comp); err != nil {
			return fmt.Errorf("update component labels: %w", err)
		}
	}

	// Project to ReadStore
	if h.componentWriter != nil {
		resourceKey := comp.Namespace + "/" + comp.Name
		view := ToView(comp)
		if err := h.componentWriter.Replace(ctx, resourceKey, []domaincomponent.ComponentView{view}); err != nil {
			return fmt.Errorf("write component store: %w", err)
		}
	}

	// Requeue at next UTC midnight for daily status rollover
	rc.Result.RequeueAfter = h.requeueDuration()

	return nil
}

// ToView converts a Component CRD into a domain ComponentView for the ReadStore.
func ToView(comp *sreportalv1alpha1.Component) domaincomponent.ComponentView {
	view := domaincomponent.ComponentView{
		Name:            comp.Name,
		DisplayName:     comp.Spec.DisplayName,
		Description:     comp.Spec.Description,
		Group:           comp.Spec.Group,
		Link:            comp.Spec.Link,
		PortalRef:       comp.Spec.PortalRef,
		DeclaredStatus:  domaincomponent.ComponentStatus(comp.Spec.Status),
		ComputedStatus:  domaincomponent.ComponentStatus(comp.Status.ComputedStatus),
		ActiveIncidents: comp.Status.ActiveIncidents,
	}
	if comp.Status.LastStatusChange != nil {
		view.LastStatusChange = comp.Status.LastStatusChange.Time
	}
	if len(comp.Status.DailyWorstStatus) > 0 {
		view.DailyWorstStatus = make([]domaincomponent.DailyStatus, len(comp.Status.DailyWorstStatus))
		for i, entry := range comp.Status.DailyWorstStatus {
			view.DailyWorstStatus[i] = domaincomponent.DailyStatus{
				Date:        entry.Date,
				WorstStatus: domaincomponent.ComponentStatus(entry.WorstStatus),
			}
		}
	}
	return view
}
