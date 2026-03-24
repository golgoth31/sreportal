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

package networkflowdiscovery

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
	// ConditionTypeReady indicates the graph was successfully built.
	ConditionTypeReady = "Ready"
)

// UpdateStatusHandler writes the computed graph into the CRD status.
type UpdateStatusHandler struct {
	client client.Client
}

// NewUpdateStatusHandler creates a new UpdateStatusHandler.
func NewUpdateStatusHandler(c client.Client) *UpdateStatusHandler {
	return &UpdateStatusHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.NetworkFlowDiscovery, ChainData]) error {
	logger := log.FromContext(ctx).WithName("update-status")
	now := metav1.Now()

	base := rc.Resource.DeepCopy()

	rc.Resource.Status.Nodes = rc.Data.Nodes
	rc.Resource.Status.Edges = rc.Data.Edges
	rc.Resource.Status.LastReconcileTime = &now

	readyCondition := metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "ReconcileSucceeded",
		Message:            fmt.Sprintf("discovered %d nodes and %d edges", len(rc.Data.Nodes), len(rc.Data.Edges)),
		LastTransitionTime: now,
	}
	meta.SetStatusCondition(&rc.Resource.Status.Conditions, readyCondition)

	logger.V(1).Info("updating status", "nodes", len(rc.Data.Nodes), "edges", len(rc.Data.Edges))

	if err := h.client.Status().Patch(ctx, rc.Resource, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch NetworkFlowDiscovery status: %w", err)
	}

	return nil
}
