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

package flowobserver

import (
	"context"

	"github.com/go-logr/logr"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
)

// Discover creates a FlowObserver from a FlowObserver CRD spec.
// Returns nil if the Prometheus instance is not reachable or no mesh metrics are found.
func Discover(ctx context.Context, logger logr.Logger, spec sreportalv1alpha1.FlowObserverSpec) domainnetpol.FlowObserver {
	obs, err := NewPrometheusObserver(spec)
	if err != nil {
		logger.V(1).Info("prometheus client creation failed", "address", spec.Prometheus.Address, "err", err)
		return nil
	}

	ok, err := obs.Available(ctx)
	if err != nil || !ok {
		logger.V(1).Info("prometheus not available or no mesh metrics found", "address", spec.Prometheus.Address, "err", err)
		return nil
	}

	logger.Info("prometheus flow observer available", "address", spec.Prometheus.Address, "mesh", obs.ActiveMesh())

	return obs
}
