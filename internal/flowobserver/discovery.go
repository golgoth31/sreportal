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
	if cfg != nil && cfg.Address != "" {
		address = cfg.Address
	}

	obs := NewHubbleObserver(address)

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
	if cfg != nil && cfg.Address != "" {
		address = cfg.Address
	}

	obs, err := NewPrometheusObserver(address)
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
