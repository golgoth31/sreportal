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
)

func TestExtractPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		tag    string
		want   TagPattern
		wantOK bool
	}{
		{
			name:   "variant suffix alpine",
			tag:    tTag1253Alpine,
			want:   TagPattern{Prefix: "", Suffix: tSuffixAlpine, HasPre: false},
			wantOK: true,
		},
		{
			name:   "variant suffix bookworm-slim",
			tag:    tTagBookwormSlim,
			want:   TagPattern{Prefix: "", Suffix: "-bookworm-slim", HasPre: false},
			wantOK: true,
		},
		{
			name:   "v-prefix no suffix",
			tag:    "v1.2",
			want:   TagPattern{Prefix: "v", Suffix: "", HasPre: false},
			wantOK: true,
		},
		{
			name:   "pre-release rc",
			tag:    tTag123RC1,
			want:   TagPattern{Prefix: "", Suffix: "", HasPre: true},
			wantOK: true,
		},
		{
			name:   "pre-release beta",
			tag:    "2.0.0-beta.3",
			want:   TagPattern{Prefix: "", Suffix: "", HasPre: true},
			wantOK: true,
		},
		{
			name:   "pre-release alpha with v-prefix",
			tag:    "v3.1.0-alpha.1",
			want:   TagPattern{Prefix: "v", Suffix: "", HasPre: true},
			wantOK: true,
		},
		{
			name:   "pre-release dev",
			tag:    "1.0.0-dev",
			want:   TagPattern{Prefix: "", Suffix: "", HasPre: true},
			wantOK: true,
		},
		{
			name:   "pre-release snapshot",
			tag:    "1.0.0-snapshot",
			want:   TagPattern{Prefix: "", Suffix: "", HasPre: true},
			wantOK: true,
		},
		{
			name:   "pre-release m-style",
			tag:    "1.0.0-m42",
			want:   TagPattern{Prefix: "", Suffix: "", HasPre: true},
			wantOK: true,
		},
		{
			name:   "two-segment plain",
			tag:    tTag183,
			want:   TagPattern{Prefix: "", Suffix: "", HasPre: false},
			wantOK: true,
		},
		{
			name:   "two-segment with alpine variant",
			tag:    tTag15Alpine,
			want:   TagPattern{Prefix: "", Suffix: tSuffixAlpine, HasPre: false},
			wantOK: true,
		},
		{
			name:   "three-segment plain",
			tag:    tVersion123,
			want:   TagPattern{Prefix: "", Suffix: "", HasPre: false},
			wantOK: true,
		},
		{
			name:   "mycorp suffix treated as variant",
			tag:    "1.2.3-mycorp",
			want:   TagPattern{Prefix: "", Suffix: "-mycorp", HasPre: false},
			wantOK: true,
		},
		{
			name:   "pure label alpine",
			tag:    tTagAlpine,
			want:   TagPattern{},
			wantOK: false,
		},
		{
			name:   tTagLatest,
			tag:    tTagLatest,
			want:   TagPattern{},
			wantOK: false,
		},
		{
			name:   "commit hash",
			tag:    tTagCommit,
			want:   TagPattern{},
			wantOK: false,
		},
		{
			name:   "mono-numeric build number rejected",
			tag:    "9799770991",
			want:   TagPattern{},
			wantOK: false,
		},
		{
			name:   "mono-numeric plain rejected",
			tag:    "18",
			want:   TagPattern{},
			wantOK: false,
		},
		{
			name:   "mono-numeric with alpine variant accepted",
			tag:    tTag15Alpine,
			want:   TagPattern{Prefix: "", Suffix: tSuffixAlpine, HasPre: false},
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ExtractPattern(tc.tag)
			if ok != tc.wantOK {
				t.Fatalf("ExtractPattern(%q) ok=%v want %v", tc.tag, ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Fatalf("ExtractPattern(%q) = %+v, want %+v", tc.tag, got, tc.want)
			}
		})
	}
}

func TestTagPatternMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		original  string
		candidate string
		want      bool
	}{
		// Variant suffix filtering
		{name: "alpine matches alpine", original: tTag1250Alpine, candidate: tTag1253Alpine, want: true},
		{name: "alpine rejects plain", original: tTag1250Alpine, candidate: tTag1253, want: false},
		{name: "alpine rejects debian", original: tTag1250Alpine, candidate: "1.25.3-debian", want: false},

		// v-prefix
		{name: "v-prefix matches v-prefix", original: tTagV720, candidate: tTagV724, want: true},
		{name: "v-prefix rejects no-prefix", original: tTagV720, candidate: "7.2.4", want: false},
		{name: "no-prefix rejects v-prefix", original: "7.2.0", candidate: tTagV724, want: false},

		// Loose segment count
		{name: "18.3 matches 18.4", original: tTag183, candidate: tTag184, want: true},
		{name: "18.3 matches 18.3.0", original: tTag183, candidate: "18.3.0", want: true},
		{name: "18.3 rejects 18.3-alpine", original: tTag183, candidate: "18.3-alpine", want: false},

		// Pre-release track: original on rc → candidate must be pre-release too
		{name: "rc.1 matches rc.2", original: tTag123RC1, candidate: tTag124RC2, want: true},
		{name: "rc.1 rejects stable", original: tTag123RC1, candidate: tVersion124, want: false},
		{name: "stable matches stable", original: tVersion123, candidate: tVersion124, want: true},
		// Stable original must NOT match pre-release candidates: this prevents
		// nightly/dev tags from leaking into the "latest" UI cell.
		{name: "stable rejects pre-release candidate", original: tVersion123, candidate: tTag123RC1, want: false},
		// Regression: longhorn publishes dev nightlies like v1.12.0-dev-20260503.
		// A stable user on v1.11.1 must not be matched against them.
		{name: "stable rejects dev nightly", original: tTagLonghornV111, candidate: tTagLonghornDev, want: false},
		{name: "stable accepts stable next minor", original: tTagLonghornV111, candidate: tTagLonghornV120, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, ok := ExtractPattern(tc.original)
			if !ok {
				if tc.want {
					t.Fatalf("ExtractPattern(%q) returned false but test expected match", tc.original)
				}
				return
			}
			got := p.Matches(tc.candidate)
			if got != tc.want {
				t.Fatalf("TagPattern{%+v}.Matches(%q) = %v, want %v", p, tc.candidate, got, tc.want)
			}
		})
	}
}
