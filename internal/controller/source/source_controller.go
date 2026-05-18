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

package source

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// SourceReconciler is the global producer: periodically lists every enabled
// source kind cluster-wide and populates the SourceEndpointStore.
type SourceReconciler struct {
	Client   client.Client
	Registry *registry.Registry
	Store    domainsource.SourceEndpointWriter
	Interval time.Duration

	previousKinds map[registry.SourceType]bool
}

var _ manager.Runnable = (*SourceReconciler)(nil)

// Start runs the producer loop until ctx is cancelled. Each tick rebuilds the
// kind-set from non-remote DNS CRs and refreshes the SourceEndpointStore.
func (r *SourceReconciler) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("source.reconciler")
	r.previousKinds = Cycle(ctx, r.Client, r.Registry, r.Store, r.previousKinds)
	t := time.NewTicker(r.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			r.previousKinds = Cycle(ctx, r.Client, r.Registry, r.Store, r.previousKinds)
			logger.V(2).Info("cycle complete", "kinds", len(r.previousKinds))
		}
	}
}
