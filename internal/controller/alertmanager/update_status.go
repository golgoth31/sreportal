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

package alertmanager

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanager"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	ConditionTypeReady = "Ready"
)

// UpdateStatusHandler writes the fetched alerts into the Alertmanager resource status.
type UpdateStatusHandler struct {
	client client.Client
}

// NewUpdateStatusHandler creates a new UpdateStatusHandler.
func NewUpdateStatusHandler(c client.Client) *UpdateStatusHandler {
	return &UpdateStatusHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Alertmanager]) error {
	logger := log.FromContext(ctx).WithName("update-status")
	now := metav1.Now()

	var domainAlerts []domainalertmanager.Alert
	if data, ok := rc.Data[DataKeyAlerts].([]domainalertmanager.Alert); ok {
		domainAlerts = data
	}

	var domainSilences []domainalertmanager.Silence
	if data, ok := rc.Data[DataKeySilences].([]domainalertmanager.Silence); ok {
		domainSilences = data
	}

	base := rc.Resource.DeepCopy()

	rc.Resource.Status.ActiveAlerts = toAlertStatuses(domainAlerts)
	rc.Resource.Status.Silences = toSilenceStatuses(domainSilences)
	rc.Resource.Status.LastReconcileTime = &now

	readyCondition := metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "ReconcileSucceeded",
		Message:            fmt.Sprintf("fetched %d active alerts", len(domainAlerts)),
		LastTransitionTime: now,
	}
	meta.SetStatusCondition(&rc.Resource.Status.Conditions, readyCondition)

	logger.V(1).Info("updating status", "alertCount", len(domainAlerts))
	if err := h.client.Status().Patch(ctx, rc.Resource, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch Alertmanager status: %w", err)
	}

	return nil
}

func toAlertStatuses(alerts []domainalertmanager.Alert) []sreportalv1alpha1.AlertStatus {
	if len(alerts) == 0 {
		return nil
	}

	statuses := make([]sreportalv1alpha1.AlertStatus, 0, len(alerts))
	for _, a := range alerts {
		receivers := make([]string, 0, len(a.Receivers))
		for _, r := range a.Receivers {
			receivers = append(receivers, r.Name())
		}
		s := sreportalv1alpha1.AlertStatus{
			Fingerprint: a.Fingerprint,
			Labels:      a.Labels,
			Annotations: a.Annotations,
			State:       string(a.State),
			StartsAt:    metav1.NewTime(a.StartsAt),
			UpdatedAt:   metav1.NewTime(a.UpdatedAt),
			Receivers:   receivers,
			SilencedBy:  a.SilencedBy,
		}
		if a.EndsAt != nil {
			endsAt := metav1.NewTime(*a.EndsAt)
			s.EndsAt = &endsAt
		}
		statuses = append(statuses, s)
	}
	return statuses
}

func toSilenceStatuses(silences []domainalertmanager.Silence) []sreportalv1alpha1.SilenceStatus {
	if len(silences) == 0 {
		return nil
	}

	statuses := make([]sreportalv1alpha1.SilenceStatus, 0, len(silences))
	for _, s := range silences {
		matchers := make([]sreportalv1alpha1.MatcherStatus, 0, len(s.Matchers()))
		for _, m := range s.Matchers() {
			matchers = append(matchers, sreportalv1alpha1.MatcherStatus{
				Name:    m.Name,
				Value:   m.Value,
				IsRegex: m.IsRegex,
			})
		}
		statuses = append(statuses, sreportalv1alpha1.SilenceStatus{
			ID:        s.ID(),
			Matchers:  matchers,
			StartsAt:  metav1.NewTime(s.StartsAt()),
			EndsAt:    metav1.NewTime(s.EndsAt()),
			Status:    string(s.Status()),
			CreatedBy: s.CreatedBy(),
			Comment:   s.Comment(),
			UpdatedAt: metav1.NewTime(s.UpdatedAt()),
		})
	}
	return statuses
}