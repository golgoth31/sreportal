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

package dns

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// ConditionTypeReady indicates the DNS resource is ready
	ConditionTypeReady = "Ready"
)

// UpdateStatusHandler updates the DNS resource status
type UpdateStatusHandler struct {
	client client.Client
}

// NewUpdateStatusHandler creates a new UpdateStatusHandler
func NewUpdateStatusHandler(c client.Client) *UpdateStatusHandler {
	return &UpdateStatusHandler{client: c}
}

// Handle implements reconciler.Handler
func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DNS, ChainData]) error {
	logger := log.FromContext(ctx).WithName("update-status")
	now := metav1.Now()

	// Get aggregated groups
	groups := rc.Data.AggregatedGroups

	// Capture base before modifications for merge patch
	base := rc.Resource.DeepCopy()

	// Update status
	rc.Resource.Status.Groups = groups
	rc.Resource.Status.LastReconcileTime = &now

	// Set Ready condition
	readyCondition := metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "ReconcileSucceeded",
		Message:            "DNS resource reconciled successfully",
		LastTransitionTime: now,
	}
	meta.SetStatusCondition(&rc.Resource.Status.Conditions, readyCondition)

	logger.V(1).Info("updating status", "groupCount", len(groups))
	if err := h.client.Status().Patch(ctx, rc.Resource, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch DNS status: %w", err)
	}

	return nil
}