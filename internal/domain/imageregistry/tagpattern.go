/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package imageregistry

import (
	"regexp"
	"strings"
)

// tagDecompRE decomposes a semver-ish tag into three groups:
//
//	group 1 – optional "v" prefix
//	group 2 – numeric core (1, 1.2, or 1.2.3 — up to three dot-segments)
//	group 3 – trailing modifier (may be empty; includes the leading "-" or "+")
var tagDecompRE = regexp.MustCompile(`^(v?)(\d+(?:\.\d+){0,2})((?:[-+][0-9A-Za-z.+\-]+)?)$`)

// preReleaseRE matches the well-known pre-release identifiers that should be
// treated as SemVer pre-release (HasPre=true) rather than as a variant flavor.
// Only `alpha`, `beta`, `rc`, `pre`, `dev`, `snapshot`, and `m<digits>` are
// recognised. Everything else (e.g. `-alpine`, `-bookworm`, `-mycorp`) is a
// variant suffix.
var preReleaseRE = regexp.MustCompile(`^-(?:alpha|beta|rc|pre|dev|snapshot|m\d+)(?:[.\-]\w+)*$`)

// TagPattern captures the structural properties of a semver-ish tag that must
// be shared by all candidate upgrade tags.
type TagPattern struct {
	Prefix string // "v" or ""
	Suffix string // variant flavor: "-alpine", "-bookworm-slim", …
	HasPre bool   // tag carries a recognised SemVer pre-release segment
}

// ExtractPattern decomposes tag into a TagPattern.
// Returns (TagPattern{}, false) when tag does not look like a versioned tag
// (e.g. pure string labels like "alpine", "latest", "main").
func ExtractPattern(tag string) (TagPattern, bool) {
	m := tagDecompRE.FindStringSubmatch(tag)
	if m == nil {
		return TagPattern{}, false
	}
	prefix := m[1]
	core := m[2]
	modifier := m[3]

	// Reject mono-numeric tags without a modifier ("18", "9799770991"). These
	// are ambiguous — build numbers, unix timestamps, commit counts — and
	// would otherwise be padded to "v9799770991.0.0" and beat real semvers.
	// A mono-numeric tag with a modifier ("15-alpine") is fine: the variant
	// suffix contextualises it as a release.
	if !strings.Contains(core, ".") && modifier == "" {
		return TagPattern{}, false
	}

	var suffix string
	hasPre := false

	if modifier != "" {
		if preReleaseRE.MatchString(modifier) {
			hasPre = true
		} else {
			suffix = modifier
		}
	}

	return TagPattern{
		Prefix: prefix,
		Suffix: suffix,
		HasPre: hasPre,
	}, true
}

// Matches reports whether candidate shares the same TagPattern as p.
// Segment count is intentionally ignored so that "18.3" matches "18.4.0".
func (p TagPattern) Matches(candidate string) bool {
	cp, ok := ExtractPattern(candidate)
	if !ok {
		return false
	}
	if cp.Prefix != p.Prefix {
		return false
	}
	if cp.Suffix != p.Suffix {
		return false
	}
	// Pre-release status must match symmetrically:
	//   - stable → stable (a user on v1.11.1 must not be offered v1.12.0-dev-…)
	//   - pre-release → pre-release (rc.1 → rc.2 OK, rc.1 → stable handled by
	//     IsUpgrade not by Matches)
	// Letting stable users see pre-release candidates leaks nightly/dev tags
	// (Longhorn v1.12.0-dev-20260503, etc.) into the "latest" UI cell even
	// though IsUpgrade flags the upgrade as false.
	if p.HasPre != cp.HasPre {
		return false
	}
	return true
}

// splitMainAndModifier separates the numeric-core portion from the trailing
// semver modifier (everything from the first "-" or "+" onward).
func splitMainAndModifier(s string) (main, mod string) {
	for i, c := range s {
		if c == '-' || c == '+' {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

// padToThreeSegments pads a dot-separated numeric string to exactly three
// segments by appending ".0" as needed.
func padToThreeSegments(core string) string {
	parts := strings.Split(core, ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	return strings.Join(parts, ".")
}
