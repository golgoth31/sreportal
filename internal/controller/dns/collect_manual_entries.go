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

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// DataKeyManualGroups is the key for storing manual groups in context
	DataKeyManualGroups = "manualGroups"
)

// CollectManualEntriesHandler collects manual DNS groups from the spec
type CollectManualEntriesHandler struct{}

// NewCollectManualEntriesHandler creates a new CollectManualEntriesHandler
func NewCollectManualEntriesHandler() *CollectManualEntriesHandler {
	return &CollectManualEntriesHandler{}
}

// Handle implements reconciler.Handler
func (h *CollectManualEntriesHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DNS]) error {
	log := logf.FromContext(ctx).WithName("collect-manual-entries")

	groups := rc.Resource.Spec.Groups
	log.V(1).Info("collected manual groups", "count", len(groups))

	rc.Data[DataKeyManualGroups] = groups
	return nil
}
