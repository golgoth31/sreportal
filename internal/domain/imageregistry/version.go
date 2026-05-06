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
// semver tag (and true) — or ("", false) when no semver tag was found.
//
// Rules:
//   - tags equal to "latest" are ignored (cf. plan §4.3),
//   - non-semver tags are ignored,
//   - the tag's original prefix is preserved (e.g. "v1.2.0" returned as-is),
//   - comparison uses golang.org/x/mod/semver, so stable releases win over
//     pre-releases of the same major.minor.patch.
func PickLatestSemver(tags []string) (string, bool) {
	var best string
	var bestCanon string
	for _, t := range tags {
		if t == string(domainimage.TagTypeLatest) {
			continue
		}
		canon := canonicalSemver(t)
		if canon == "" {
			continue
		}
		if bestCanon == "" || semver.Compare(canon, bestCanon) > 0 {
			best = t
			bestCanon = canon
		}
	}
	return best, best != ""
}

// IsUpgrade reports whether `latest` is a strictly higher semver than
// `current`. Returns false on either side being empty / non-semver.
func IsUpgrade(current, latest string) bool {
	c := canonicalSemver(current)
	l := canonicalSemver(latest)
	if c == "" || l == "" {
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
