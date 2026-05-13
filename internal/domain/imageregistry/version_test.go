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
			tags:      []string{tTagLatest, tVersion123},
			wantTag:   tVersion123,
			wantFound: true,
		},
		{
			name:         "ignore non-semver counts as rejected",
			tags:         []string{tPortalMain, tTagCommit, tVersion100},
			wantTag:      tVersion100,
			wantFound:    true,
			wantRejected: 2,
		},
		{
			name:         "no semver counts every non-latest tag as rejected",
			tags:         []string{tPortalMain, tTagLatest, tTagCommit},
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

func TestPickLatestMatching(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		tags         []string
		originalTag  string
		wantTag      string
		wantFound    bool
		wantRejected int
	}{
		{
			name:        "picks variant-correct alpine",
			tags:        []string{tTag1253, tTag1253Alpine, "1.27.0"},
			originalTag: tTag1250Alpine,
			wantTag:     tTag1253Alpine,
			wantFound:   true,
		},
		{
			name:        "two-segment alpine variant",
			tags:        []string{"15.1", "15.1-alpine", "16-alpine"},
			originalTag: tTag15Alpine,
			wantTag:     "16-alpine",
			wantFound:   true,
		},
		{
			name:        "v-prefix preserved",
			tags:        []string{"7.2.4", tTagV724},
			originalTag: tTagV720,
			wantTag:     tTagV724,
			wantFound:   true,
		},
		{
			name:        "two-segment loose upgrade to three-segment",
			tags:        []string{tTag183, tTag184, "18.3.0", "18.3-alpine", "17.5"},
			originalTag: tTag183,
			wantTag:     tTag184,
			wantFound:   true,
		},
		{
			name:        "pre-release track upgrade",
			tags:        []string{tVersion123, tTag124RC2},
			originalTag: tTag123RC1,
			wantTag:     tTag124RC2,
			wantFound:   true,
		},
		{
			name:        "unparseable original returns not found",
			tags:        []string{tVersion123},
			originalTag: tTagAlpine,
			wantTag:     "",
			wantFound:   false,
		},
		{
			name:        "no matching variant returns not found",
			tags:        []string{tTag1253, "1.27.0"},
			originalTag: tTag1250Alpine,
			wantTag:     "",
			wantFound:   false,
		},
		{
			name:        "empty tag list",
			tags:        nil,
			originalTag: tVersion123,
			wantTag:     "",
			wantFound:   false,
		},
		{
			// Regression: grafana/grafana publishes build-number tags like
			// "9799770991" alongside semver tTagGrafana1221. The mono-numeric tag must
			// not be considered a candidate for 12.2.1 — previously it was
			// padded to v9799770991.0.0 and beat v12.2.1.
			name:        "rejects mono-numeric build-number candidates",
			tags:        []string{"9799770991", tTagGrafana1221, "12.2.0", "11.5.0"},
			originalTag: tTagGrafana1221,
			wantTag:     tTagGrafana1221,
			wantFound:   true,
		},
		{
			name:        "mono-numeric original is unparseable",
			tags:        []string{tTagGrafana1221, "12.2.2"},
			originalTag: "9799770991",
			wantTag:     "",
			wantFound:   false,
		},
		{
			// Regression: longhorn publishes dev nightlies alongside stable
			// releases. A stable user on v1.11.1 must get v1.12.0, not the
			// nightly v1.12.0-dev-20260503.
			name:        "stable user ignores dev nightlies",
			tags:        []string{tTagLonghornV111, tTagLonghornV120, tTagLonghornDev, "v1.13.0-dev-20260601"},
			originalTag: tTagLonghornV111,
			wantTag:     tTagLonghornV120,
			wantFound:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, rejected, found := PickLatestMatching(tc.tags, tc.originalTag)
			if found != tc.wantFound || got != tc.wantTag || rejected != tc.wantRejected {
				t.Fatalf("PickLatestMatching(%v, %q) = (%q, %d, %v), want (%q, %d, %v)",
					tc.tags, tc.originalTag, got, rejected, found, tc.wantTag, tc.wantRejected, tc.wantFound)
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
