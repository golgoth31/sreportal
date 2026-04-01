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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	domaincomponent "github.com/golgoth31/sreportal/internal/domain/component"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

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

// Handle implements reconciler.Handler.
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
