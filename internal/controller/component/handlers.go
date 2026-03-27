// Package component contains Chain-of-Responsibility handlers for the Component controller.
package component

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domaincomponent "github.com/golgoth31/sreportal/internal/domain/component"
	domainmaint "github.com/golgoth31/sreportal/internal/domain/maintenance"
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ChainData holds shared state between handlers.
type ChainData struct {
	ComputedStatus sreportalv1alpha1.ComputedComponentStatus
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

// ComputeStatusHandler computes the effective status considering active maintenances.
type ComputeStatusHandler struct {
	maintenanceReader domainmaint.MaintenanceReader
}

// NewComputeStatusHandler creates a new ComputeStatusHandler.
func NewComputeStatusHandler(maintenanceReader domainmaint.MaintenanceReader) *ComputeStatusHandler {
	return &ComputeStatusHandler{maintenanceReader: maintenanceReader}
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

	return nil
}

// --- Handler 3: UpdateStatus ---

// UpdateStatusHandler patches the CR status, labels, and projects to the ReadStore.
type UpdateStatusHandler struct {
	client          client.Client
	componentWriter domaincomponent.ComponentWriter
}

// NewUpdateStatusHandler creates a new UpdateStatusHandler.
func NewUpdateStatusHandler(c client.Client, w domaincomponent.ComponentWriter) *UpdateStatusHandler {
	return &UpdateStatusHandler{client: c, componentWriter: w}
}

func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Component, ChainData]) error {
	comp := rc.Resource

	// Detect status change
	oldStatus := comp.Status.ComputedStatus
	comp.Status.ComputedStatus = rc.Data.ComputedStatus
	if oldStatus != rc.Data.ComputedStatus {
		now := metav1.Now()
		comp.Status.LastStatusChange = &now
	}

	// Set Ready condition + patch status
	if err := statusutil.SetConditionAndPatch(ctx, h.client, comp, "Ready", metav1.ConditionTrue, "Reconciled", "component reconciled"); err != nil {
		return err
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
	return view
}
