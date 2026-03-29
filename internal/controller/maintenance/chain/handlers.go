// Package chain contains Chain-of-Responsibility handlers for the Maintenance controller.
package chain

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	domainmaint "github.com/golgoth31/sreportal/internal/domain/maintenance"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ChainData holds shared state between handlers.
type ChainData struct {
	Phase sreportalv1alpha1.MaintenancePhase
}

// --- Handler 1: ComputePhase ---

// ComputePhaseHandler determines the phase based on current time vs schedule.
type ComputePhaseHandler struct{}

// NewComputePhaseHandler creates a new ComputePhaseHandler.
func NewComputePhaseHandler() *ComputePhaseHandler {
	return &ComputePhaseHandler{}
}

func (h *ComputePhaseHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Maintenance, ChainData]) error {
	maint := rc.Resource

	// Validate schedule
	if !maint.Spec.ScheduledEnd.After(maint.Spec.ScheduledStart.Time) {
		return fmt.Errorf("invalid schedule: %w", domainmaint.ErrInvalidSchedule)
	}

	now := metav1.Now().Time
	phase := domainmaint.ComputePhase(now, maint.Spec.ScheduledStart.Time, maint.Spec.ScheduledEnd.Time)
	rc.Data.Phase = sreportalv1alpha1.MaintenancePhase(phase)

	return nil
}

// --- Handler 2: UpdateStatus ---

// UpdateStatusHandler patches the CR status, labels, and projects to the ReadStore.
type UpdateStatusHandler struct {
	client            client.Client
	maintenanceWriter domainmaint.MaintenanceWriter
}

// NewUpdateStatusHandler creates a new UpdateStatusHandler.
func NewUpdateStatusHandler(c client.Client, w domainmaint.MaintenanceWriter) *UpdateStatusHandler {
	return &UpdateStatusHandler{client: c, maintenanceWriter: w}
}

func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Maintenance, ChainData]) error {
	maint := rc.Resource
	maint.Status.Phase = rc.Data.Phase

	// Set Ready condition + patch status
	if err := statusutil.SetConditionAndPatch(ctx, h.client, maint, "Ready", metav1.ConditionTrue, "Reconciled",
		fmt.Sprintf("phase: %s", rc.Data.Phase)); err != nil {
		return err
	}

	// Re-fetch to get fresh resourceVersion
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: maint.Namespace, Name: maint.Name}, maint); err != nil {
		return fmt.Errorf("re-fetch maintenance: %w", err)
	}

	// Add portal label
	if maint.Labels == nil {
		maint.Labels = make(map[string]string)
	}
	if maint.Labels["sreportal.io/portal"] != maint.Spec.PortalRef {
		maint.Labels["sreportal.io/portal"] = maint.Spec.PortalRef
		if err := h.client.Update(ctx, maint); err != nil {
			return fmt.Errorf("update maintenance labels: %w", err)
		}
	}

	// Project to ReadStore
	if h.maintenanceWriter != nil {
		resourceKey := maint.Namespace + "/" + maint.Name
		view := ToView(maint)
		if err := h.maintenanceWriter.Replace(ctx, resourceKey, []domainmaint.MaintenanceView{view}); err != nil {
			return fmt.Errorf("write maintenance store: %w", err)
		}
	}

	// Set strategic requeue for next phase transition (set last so chain doesn't short-circuit)
	now := metav1.Now().Time
	requeueAfter := domainmaint.ComputeRequeue(now, maint.Spec.ScheduledStart.Time, maint.Spec.ScheduledEnd.Time)
	if requeueAfter > 0 {
		rc.Result.RequeueAfter = requeueAfter
	}

	return nil
}

// ToView converts a Maintenance CRD into a domain MaintenanceView.
func ToView(maint *sreportalv1alpha1.Maintenance) domainmaint.MaintenanceView {
	return domainmaint.MaintenanceView{
		Name:           maint.Name,
		Title:          maint.Spec.Title,
		Description:    maint.Spec.Description,
		PortalRef:      maint.Spec.PortalRef,
		Components:     maint.Spec.Components,
		ScheduledStart: maint.Spec.ScheduledStart.Time,
		ScheduledEnd:   maint.Spec.ScheduledEnd.Time,
		AffectedStatus: string(maint.Spec.AffectedStatus),
		Phase:          domainmaint.MaintenancePhase(maint.Status.Phase),
	}
}
