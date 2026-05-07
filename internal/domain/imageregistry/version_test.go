/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package imageregistry

import (
	"testing"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

func TestIsUpgradable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tt   domainimage.TagType
		want bool
	}{
		{tt: domainimage.TagTypeSemver, want: true},
		{tt: domainimage.TagTypeCommit, want: false},
		{tt: domainimage.TagTypeDigest, want: false},
		{tt: domainimage.TagTypeLatest, want: false},
		{tt: domainimage.TagTypeOther, want: false},
	}
	for _, tc := range tests {
		t.Run(string(tc.tt), func(t *testing.T) {
			t.Parallel()
			if got := IsUpgradable(tc.tt); got != tc.want {
				t.Fatalf("IsUpgradable(%q) = %v, want %v", tc.tt, got, tc.want)
			}
		})
	}
}

func TestPickLatestSemver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		tags         []string
		wantTag      string
		wantFound    bool
		wantRejected int
	}{
		{
			name:      "plain semver",
			tags:      []string{tVersion123, tVersion124, tVersion100},
			wantTag:   tVersion124,
			wantFound: true,
		},
		{
			name:      "v-prefix preserved",
			tags:      []string{"v1.0.0", tVersionV120, "v1.1.5"},
			wantTag:   tVersionV120,
			wantFound: true,
		},
		{
			name:      "stable beats rc of same version",
			tags:      []string{"1.3.0", tVersionRC, tVersion124},
			wantTag:   "1.3.0",
			wantFound: true,
		},
		{
			name:      "rc returned when no stable for that version",
			tags:      []string{tVersion123, tVersionRC},
			wantTag:   tVersionRC,
			wantFound: true,
		},
		{
			name:      "ignore latest tag does not count as rejected",
			tags:      []string{"latest", tVersion123},
			wantTag:   tVersion123,
			wantFound: true,
		},
		{
			name:         "ignore non-semver counts as rejected",
			tags:         []string{tPortalMain, "abcdef1", tVersion100},
			wantTag:      tVersion100,
			wantFound:    true,
			wantRejected: 2,
		},
		{
			name:         "no semver counts every non-latest tag as rejected",
			tags:         []string{tPortalMain, "latest", "abcdef1"},
			wantTag:      "",
			wantFound:    false,
			wantRejected: 2,
		},
		{
			name:      "empty list",
			tags:      nil,
			wantTag:   "",
			wantFound: false,
		},
		{
			name:      "preserves prefix when both present picks v-prefixed if higher",
			tags:      []string{tVersion100, tVersionV120},
			wantTag:   tVersionV120,
			wantFound: true,
		},
		{
			name:         "4-segment version is rejected (semver limitation)",
			tags:         []string{"1.2.3.4"},
			wantTag:      "",
			wantFound:    false,
			wantRejected: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, rejected, found := PickLatestSemver(tc.tags)
			if found != tc.wantFound || got != tc.wantTag || rejected != tc.wantRejected {
				t.Fatalf("PickLatestSemver(%v) = (%q, %d, %v), want (%q, %d, %v)",
					tc.tags, got, rejected, found, tc.wantTag, tc.wantRejected, tc.wantFound)
			}
		})
	}
}

func TestIsUpgrade(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{name: "same", current: tVersion123, latest: tVersion123, want: false},
		{name: "higher latest", current: tVersion123, latest: tVersion124, want: true},
		{name: "lower latest", current: tVersion124, latest: tVersion123, want: false},
		{name: "v-prefix mixed", current: tVersion100, latest: tVersionV120, want: true},
		{name: "current invalid", current: "main", latest: tVersion100, want: false},
		{name: "latest invalid", current: tVersion100, latest: "main", want: false},
		{name: "both empty", current: "", latest: "", want: false},
		{name: "empty latest", current: tVersion100, latest: "", want: false},
		// Pre-release suppression: stable → pre-release must NOT be flagged as
		// an upgrade — production users on stable tags should not be promoted
		// to release candidates without an explicit opt-in.
		{name: "stable to higher pre-release suppressed", current: tVersion123, latest: tVersion140RC1, want: false},
		{name: "stable to same-version pre-release suppressed", current: tVersion100, latest: "1.0.0-rc.1", want: false},
		// Users already on a pre-release may still upgrade onto a higher
		// pre-release or a stable release of greater version.
		{name: "pre-release to higher pre-release", current: tVersion140RC1, latest: "1.4.0-rc.2", want: true},
		{name: "pre-release to stable", current: tVersion140RC1, latest: "1.4.0", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsUpgrade(tc.current, tc.latest); got != tc.want {
				t.Fatalf("IsUpgrade(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}
