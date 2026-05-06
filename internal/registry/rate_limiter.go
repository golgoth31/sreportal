/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Package registry contains the infrastructure adapter that resolves
// container image versions by querying remote OCI registries.
package registry

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// hostLimits is the hardcoded per-host rate budget used when no entry exists
// in the user-supplied config. Values from plan §5.1.
var hostLimits = map[string]rate.Limit{
	"docker.io":            rate.Every(12 * time.Second), // 5/min
	"index.docker.io":      rate.Every(12 * time.Second), // canonical Docker Hub host
	"registry-1.docker.io": rate.Every(12 * time.Second),
	"ghcr.io":              rate.Every(2 * time.Second),
	"registry.k8s.io":      rate.Every(2 * time.Second),
	"quay.io":              rate.Every(2 * time.Second),
	"gcr.io":               rate.Every(2 * time.Second),
}

// defaultRate is used for hosts not present in hostLimits.
const defaultRate rate.Limit = rate.Limit(1.0 / 3.0) // ~20/min

// HostLimiter is a per-host token bucket gate around registry calls. It
// serialises requests by host so we never hammer a single registry, while
// still allowing parallelism across distinct hosts.
type HostLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
}

// NewHostLimiter creates an empty HostLimiter; per-host limiters are
// instantiated lazily on first Wait.
func NewHostLimiter() *HostLimiter {
	return &HostLimiter{limiters: make(map[string]*rate.Limiter)}
}

// Wait blocks until a token is available for the given host, or until ctx
// is done. Bursts are limited to one token to keep the gate strict.
func (hl *HostLimiter) Wait(ctx context.Context, host string) error {
	return hl.limiterFor(host).Wait(ctx)
}

// limiterFor returns (creating if needed) the limiter for the given host.
func (hl *HostLimiter) limiterFor(host string) *rate.Limiter {
	hl.mu.RLock()
	lim, ok := hl.limiters[host]
	hl.mu.RUnlock()
	if ok {
		return lim
	}

	hl.mu.Lock()
	defer hl.mu.Unlock()
	if lim, ok = hl.limiters[host]; ok {
		return lim
	}
	limit, ok := hostLimits[host]
	if !ok {
		limit = defaultRate
	}
	lim = rate.NewLimiter(limit, 1)
	hl.limiters[host] = lim
	return lim
}
