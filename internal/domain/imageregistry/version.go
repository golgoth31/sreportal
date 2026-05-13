/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package imageregistry

import (
	"strings"

	"golang.org/x/mod/semver"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

// IsUpgradable returns true when the given tag type is comparable as semver.
// Only TagTypeSemver qualifies; commits, digests, "latest" and other are
// considered immutable / unresolvable as a version.
func IsUpgradable(tt domainimage.TagType) bool {
	return tt == domainimage.TagTypeSemver
}

// PickLatestSemver scans the given tag list and returns the highest valid
// semver tag, the count of tags rejected as non-semver, and a found flag.
// Returns ("", 0, false) on an empty list and ("", n, false) when every
// non-"latest" tag was rejected.
//
// Rules:
//   - tags equal to "latest" are ignored and do NOT count as rejected,
//   - non-semver tags are rejected and counted (callers may emit a metric),
//   - the tag's original prefix is preserved (e.g. "v1.2.0" returned as-is),
//   - comparison uses golang.org/x/mod/semver, so stable releases win over
//     pre-releases of the same major.minor.patch.
//
// 2-segment tags such as "18.3" or "v1.2" are accepted and padded to three
// segments for internal comparison (the original tag string is returned).
// 4-segment tags ("1.2.3.4") are still rejected.
func PickLatestSemver(tags []string) (latest string, rejected int, found bool) {
	var best string
	var bestCanon string
	for _, t := range tags {
		if t == string(domainimage.TagTypeLatest) {
			continue
		}
		canon := canonicalSemver(t)
		if canon == "" {
			rejected++
			continue
		}
		if bestCanon == "" || semver.Compare(canon, bestCanon) > 0 {
			best = t
			bestCanon = canon
		}
	}
	return best, rejected, best != ""
}

// IsUpgrade reports whether `latest` is a strictly higher semver than
// `current`. Returns false on either side being empty / non-semver.
//
// Pre-release suppression: when `current` is a stable release but `latest` is
// a pre-release (e.g. current="1.3.0", latest="1.4.0-rc.1"), the function
// returns false. Promoting users from a stable tag onto a pre-release would
// surface false-positive "upgrade available" signals in production. Users
// already on a pre-release track may still upgrade between pre-releases or
// onto a stable release of equal-or-higher version.
func IsUpgrade(current, latest string) bool {
	c := canonicalSemver(current)
	l := canonicalSemver(latest)
	if c == "" || l == "" {
		return false
	}
	if semver.Prerelease(c) == "" && semver.Prerelease(l) != "" {
		return false
	}
	return semver.Compare(l, c) > 0
}

// PickLatestMatching scans tags keeping only candidates that share the same
// TagPattern as originalTag, then returns the highest SemVer among them.
//
// When originalTag cannot be parsed as a TagPattern (e.g. "alpine", "latest"),
// the function returns ("", 0, false) — no fallback to cross-variant comparison.
func PickLatestMatching(tags []string, originalTag string) (latest string, rejected int, found bool) {
	pattern, ok := ExtractPattern(originalTag)
	if !ok {
		return "", 0, false
	}

	var filtered []string
	for _, t := range tags {
		if pattern.Matches(t) {
			filtered = append(filtered, t)
		}
	}
	return PickLatestSemver(filtered)
}

// canonicalSemver returns a `v`-prefixed 3-segment canonical form of the
// input tag suitable for golang.org/x/mod/semver comparison, or "" if the
// tag cannot be normalised.
//
// 2-segment tags ("18.3", "v1.2") are padded to 3 segments ("v18.3.0",
// "v1.2.0") for comparison; the original tag string is preserved as the
// return value of the Pick* functions.
func canonicalSemver(tag string) string {
	if tag == "" {
		return ""
	}
	core := strings.TrimPrefix(tag, "v")
	main, mod := splitMainAndModifier(core)
	padded := padToThreeSegments(main)
	candidate := "v" + padded + mod
	if !semver.IsValid(candidate) {
		return ""
	}
	return candidate
}
