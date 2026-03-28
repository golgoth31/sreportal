// Package incidentctrl contains Chain-of-Responsibility handlers for the Incident controller.
package incidentctrl

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	domainincident "github.com/golgoth31/sreportal/internal/domain/incident"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ChainData holds shared state between handlers.
type ChainData struct {
	Computed domainincident.ComputedStatus
}

// --- Handler 1: ComputeStatus ---

// ComputeStatusHandler derives the phase, timestamps, and duration from the updates timeline.
type ComputeStatusHandler struct{}

// NewComputeStatusHandler creates a new ComputeStatusHandler.
func NewComputeStatusHandler() *ComputeStatusHandler {
	return &ComputeStatusHandler{}
}

func (h *ComputeStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Incident, ChainData]) error {
	inc := rc.Resource

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
		return fmt.Errorf("compute incident status: %w", err)
	}

	rc.Data.Computed = computed
	return nil
}

// --- Handler 2: UpdateStatus ---

// UpdateStatusHandler patches the CR status, labels, and projects to the ReadStore.
type UpdateStatusHandler struct {
	client         client.Client
	incidentWriter domainincident.IncidentWriter
}

// NewUpdateStatusHandler creates a new UpdateStatusHandler.
func NewUpdateStatusHandler(c client.Client, w domainincident.IncidentWriter) *UpdateStatusHandler {
	return &UpdateStatusHandler{client: c, incidentWriter: w}
}

func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Incident, ChainData]) error {
	inc := rc.Resource
	computed := rc.Data.Computed

	// Apply computed status
	inc.Status.CurrentPhase = sreportalv1alpha1.IncidentPhase(computed.CurrentPhase)
	startedAt := metav1.NewTime(computed.StartedAt)
	inc.Status.StartedAt = &startedAt
	inc.Status.DurationMinutes = computed.DurationMinutes

	if !computed.ResolvedAt.IsZero() {
		resolvedAt := metav1.NewTime(computed.ResolvedAt)
		inc.Status.ResolvedAt = &resolvedAt
	} else {
		inc.Status.ResolvedAt = nil
	}

	// Set Ready condition + patch status
	if err := statusutil.SetConditionAndPatch(ctx, h.client, inc, "Ready", metav1.ConditionTrue, "Reconciled",
		fmt.Sprintf("phase: %s", computed.CurrentPhase)); err != nil {
		return err
	}

	// Re-fetch to get fresh resourceVersion
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: inc.Namespace, Name: inc.Name}, inc); err != nil {
		return fmt.Errorf("re-fetch incident: %w", err)
	}

	// Add portal label
	if inc.Labels == nil {
		inc.Labels = make(map[string]string)
	}
	if inc.Labels["sreportal.io/portal"] != inc.Spec.PortalRef {
		inc.Labels["sreportal.io/portal"] = inc.Spec.PortalRef
		if err := h.client.Update(ctx, inc); err != nil {
			return fmt.Errorf("update incident labels: %w", err)
		}
	}

	// Project to ReadStore
	if h.incidentWriter != nil {
		resourceKey := inc.Namespace + "/" + inc.Name
		view := ToView(inc)
		if err := h.incidentWriter.Replace(ctx, resourceKey, []domainincident.IncidentView{view}); err != nil {
			return fmt.Errorf("write incident store: %w", err)
		}
	}

	return nil
}

// ToView converts an Incident CRD into a domain IncidentView.
func ToView(inc *sreportalv1alpha1.Incident) domainincident.IncidentView {
	view := domainincident.IncidentView{
		Name:            inc.Name,
		Title:           inc.Spec.Title,
		PortalRef:       inc.Spec.PortalRef,
		Components:      inc.Spec.Components,
		Severity:        domainincident.IncidentSeverity(inc.Spec.Severity),
		CurrentPhase:    domainincident.IncidentPhase(inc.Status.CurrentPhase),
		DurationMinutes: inc.Status.DurationMinutes,
	}
	if inc.Status.StartedAt != nil {
		view.StartedAt = inc.Status.StartedAt.Time
	}
	if inc.Status.ResolvedAt != nil {
		view.ResolvedAt = inc.Status.ResolvedAt.Time
	}

	updates := make([]domainincident.UpdateView, 0, len(inc.Spec.Updates))
	for _, u := range inc.Spec.Updates {
		updates = append(updates, domainincident.UpdateView{
			Timestamp: u.Timestamp.Time,
			Phase:     domainincident.IncidentPhase(u.Phase),
			Message:   u.Message,
		})
	}
	view.Updates = updates

	return view
}
