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

	"github.com/golgoth31/sreportal/internal/config"
	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
)

// Discover creates the appropriate FlowObserver based on configuration.
// Returns nil if no provider is available (graceful degradation).
func Discover(ctx context.Context, logger logr.Logger, cfg *config.FlowObservationConfig) domainnetpol.FlowObserver {
	if cfg == nil {
		cfg = &config.FlowObservationConfig{}
	}

	provider := cfg.Provider
	if provider == "" {
		provider = "auto"
	}

	switch provider {
	case "none":
		logger.Info("flow observation disabled by configuration")
		return nil

	case "hubble":
		return discoverHubble(ctx, logger, cfg.Hubble)

	case "prometheus":
		return discoverPrometheus(ctx, logger, cfg.Prometheus)

	case "auto":
		if obs := discoverHubble(ctx, logger, cfg.Hubble); obs != nil {
			return obs
		}

		if obs := discoverPrometheus(ctx, logger, cfg.Prometheus); obs != nil {
			return obs
		}

		logger.Info("no flow observation provider available")

		return nil

	default:
		logger.Error(nil, "unknown flow observation provider", "provider", provider)
		return nil
	}
}

func discoverHubble(ctx context.Context, logger logr.Logger, cfg *config.HubbleObserverConfig) domainnetpol.FlowObserver {
	address := defaultHubbleAddress

	var opts []HubbleOption

	if cfg != nil {
		if cfg.Address != "" {
			address = cfg.Address
		}

		if cfg.FlowWindow.Duration() > 0 {
			opts = append(opts, WithHubbleFlowWindow(cfg.FlowWindow.Duration()))
		}
	}

	obs := NewHubbleObserver(address, opts...)

	ok, err := obs.Available(ctx)
	if err != nil || !ok {
		logger.V(1).Info("hubble relay not available", "address", address, "err", err)
		return nil
	}

	logger.Info("hubble flow observer available", "address", address)

	return obs
}

func discoverPrometheus(ctx context.Context, logger logr.Logger, cfg *config.PrometheusObserverConfig) domainnetpol.FlowObserver {
	address := "http://prometheus.monitoring.svc.cluster.local:9090"

	var opts []PrometheusOption

	if cfg != nil {
		if cfg.Address != "" {
			address = cfg.Address
		}

		if cfg.QueryWindow.Duration() > 0 {
			opts = append(opts, WithPrometheusQueryWindow(cfg.QueryWindow.Duration()))
		}
	}

	obs, err := NewPrometheusObserver(address, opts...)
	if err != nil {
		logger.V(1).Info("prometheus client creation failed", "address", address, "err", err)
		return nil
	}

	ok, err := obs.Available(ctx)
	if err != nil || !ok {
		logger.V(1).Info("prometheus not available or no mesh metrics found", "address", address, "err", err)
		return nil
	}

	logger.Info("prometheus flow observer available", "address", address, "mesh", obs.MeshName())

	return obs
}
