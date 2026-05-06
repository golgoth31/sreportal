/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package registry

import (
	"context"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestHostLimiter_DefaultUsedForUnknownHost(t *testing.T) {
	t.Parallel()

	hl := NewHostLimiter()
	lim := hl.limiterFor("unknown.example.com")
	if lim.Limit() != defaultRate {
		t.Fatalf("unknown host should use defaultRate, got %v", lim.Limit())
	}
}

func TestHostLimiter_KnownHostHasSpecificRate(t *testing.T) {
	t.Parallel()

	hl := NewHostLimiter()
	lim := hl.limiterFor("docker.io")
	if lim.Limit() == defaultRate {
		t.Fatalf("docker.io must have a specific rate (not default)")
	}
}

func TestHostLimiter_SameHostShareLimiter(t *testing.T) {
	t.Parallel()

	hl := NewHostLimiter()
	a := hl.limiterFor("docker.io")
	b := hl.limiterFor("docker.io")
	if a != b {
		t.Fatalf("limiterFor must return the same instance for the same host")
	}
}

func TestHostLimiter_WaitFastWhenBucketFull(t *testing.T) {
	t.Parallel()

	hl := NewHostLimiter()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := hl.Wait(ctx, "ghcr.io"); err != nil {
		t.Fatalf("first call should not block (token already available): %v", err)
	}
}

func TestHostLimiter_WaitRespectsCancellation(t *testing.T) {
	t.Parallel()

	hl := NewHostLimiter()
	// Drain the bucket on a fresh, very-slow host so the next Wait blocks.
	hl.mu.Lock()
	hl.limiters["slow.example.com"] = rate.NewLimiter(rate.Every(time.Hour), 1)
	hl.mu.Unlock()

	// Consume the initial token.
	if err := hl.Wait(context.Background(), "slow.example.com"); err != nil {
		t.Fatalf("first wait should succeed: %v", err)
	}

	// Now the next Wait should block ~1h; cancel quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := hl.Wait(ctx, "slow.example.com")
	if err == nil {
		t.Fatalf("expected cancellation error")
	}
}
