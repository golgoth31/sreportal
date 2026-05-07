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
// LIMITATION: golang.org/x/mod/semver only accepts strict 3-segment versions
// (major.minor.patch). Tags such as "1.2" or "1.2.3.4" will be rejected even
// though some registries publish them. This trades a few false negatives for
// a deterministic comparator and is intentional. The rejected counter exposes
// how often the limitation is hit per host.
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

// canonicalSemver returns a `v`-prefixed canonical form of the input tag if
// it is a valid semver per golang.org/x/mod/semver, or "" if not.
//
// The package requires the leading `v`, so we add one if missing.
func canonicalSemver(tag string) string {
	if tag == "" {
		return ""
	}
	candidate := tag
	if !strings.HasPrefix(candidate, "v") {
		candidate = "v" + candidate
	}
	if !semver.IsValid(candidate) {
		return ""
	}
	return candidate
}
