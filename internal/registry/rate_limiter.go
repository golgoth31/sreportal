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
	"container/list"
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

// defaultLimiterCacheCap caps the number of cached per-host limiters. Any
// realistic deployment touches a handful of registries; the bound exists to
// prevent unbounded map growth if a misbehaving CR keeps rotating hosts.
const defaultLimiterCacheCap = 256

// HostLimiter is a per-host token bucket gate around registry calls. It
// serialises requests by host so we never hammer a single registry, while
// still allowing parallelism across distinct hosts.
//
// The internal limiter cache is an LRU bounded to defaultLimiterCacheCap
// entries — evicting the least-recently-used host when full. Because each
// limiter is a token bucket whose rate refills on its own timeline, evicting
// a cold limiter has no functional consequence: the next lookup recreates it
// with a fresh full bucket. This prevents an attacker from exhausting memory
// by spawning CRs targeting unique hosts.
type HostLimiter struct {
	mu      sync.Mutex
	cap     int
	entries map[string]*list.Element
	order   *list.List // front = MRU, back = LRU
}

type hostLimiterEntry struct {
	host    string
	limiter *rate.Limiter
}

// NewHostLimiter creates an empty HostLimiter with the default cache size.
func NewHostLimiter() *HostLimiter {
	return newHostLimiterWithCap(defaultLimiterCacheCap)
}

func newHostLimiterWithCap(c int) *HostLimiter {
	return &HostLimiter{
		cap:     c,
		entries: make(map[string]*list.Element, c),
		order:   list.New(),
	}
}

// Wait blocks until a token is available for the given host, or until ctx
// is done. Bursts are limited to one token to keep the gate strict.
func (hl *HostLimiter) Wait(ctx context.Context, host string) error {
	return hl.limiterFor(host).Wait(ctx)
}

// limiterFor returns (creating if needed) the limiter for the given host
// and marks it as most-recently-used.
func (hl *HostLimiter) limiterFor(host string) *rate.Limiter {
	hl.mu.Lock()
	defer hl.mu.Unlock()

	if elem, ok := hl.entries[host]; ok {
		hl.order.MoveToFront(elem)
		return elem.Value.(*hostLimiterEntry).limiter
	}

	limit, ok := hostLimits[host]
	if !ok {
		limit = defaultRate
	}
	lim := rate.NewLimiter(limit, 1)
	elem := hl.order.PushFront(&hostLimiterEntry{host: host, limiter: lim})
	hl.entries[host] = elem

	if hl.order.Len() > hl.cap {
		oldest := hl.order.Back()
		if oldest != nil {
			hl.order.Remove(oldest)
			delete(hl.entries, oldest.Value.(*hostLimiterEntry).host)
		}
	}
	return lim
}
