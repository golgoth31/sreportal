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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// SyncEndpointsHashHandler recomputes the EndpointsHash and patches it on the CR
// when it diverges from the actual endpoint contents (e.g. after a manual edit).
// This keeps the SourceReconciler's skip-if-unchanged logic correct.
type SyncEndpointsHashHandler struct {
	client client.Client
}

// NewSyncEndpointsHashHandler creates a new SyncEndpointsHashHandler.
func NewSyncEndpointsHashHandler(c client.Client) *SyncEndpointsHashHandler {
	return &SyncEndpointsHashHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *SyncEndpointsHashHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DNSRecord, ChainData]) error {
	record := rc.Resource
	if len(record.Status.Endpoints) == 0 {
		return nil
	}

	logger := log.FromContext(ctx)
	computedHash := adapter.EndpointStatusHash(record.Status.Endpoints)
	if record.Status.EndpointsHash == computedHash {
		return nil
	}

	logger.Info("endpoints hash out of sync, resyncing",
		"stored", record.Status.EndpointsHash, "computed", computedHash)
	base := record.DeepCopy()
	record.Status.EndpointsHash = computedHash
	if err := h.client.Status().Patch(ctx, record, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch EndpointsHash: %w", err)
	}
	return nil
}
