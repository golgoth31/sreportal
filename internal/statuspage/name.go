/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package statuspage

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

const (
	maxNameLen     = 63
	hashLen        = 7
	separatorCount = 2 // two hyphens: portal-title-hash
	fixedLen       = hashLen + separatorCount
	slugBudget     = maxNameLen - fixedLen // 54 chars for portal + title slugs
	maxPortalSlug  = 14
)

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateCRName builds a deterministic, human-readable K8s CR name from
// portalRef and title. The format is "<portal-slug>-<title-slug>-<hash7>",
// capped at 63 characters. The 7-char SHA-256 hash is always computed on the
// full (untruncated) inputs, so two names that would slug-collide after
// truncation still get different hashes.
func GenerateCRName(portalRef, title string) string {
	hash := shortHash(portalRef, title)

	portalSlug := slugify(portalRef)
	titleSlug := slugify(title)

	portalSlug, titleSlug = fitBudget(portalSlug, titleSlug, slugBudget)

	return portalSlug + "-" + titleSlug + "-" + hash
}

func shortHash(portalRef, title string) string {
	h := sha256.Sum256([]byte(portalRef + "/" + title))
	return fmt.Sprintf("%x", h[:4])[:hashLen]
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// fitBudget trims portalSlug and titleSlug to fit within the given budget.
// It reduces titleSlug first (down to minTitleSlug), then portalSlug.
func fitBudget(portalSlug, titleSlug string, budget int) (string, string) {
	// Cap portal to its max first
	if len(portalSlug) > maxPortalSlug {
		portalSlug = trimSlug(portalSlug, maxPortalSlug)
	}

	// Title gets whatever is left after portal
	titleBudget := budget - len(portalSlug)
	if len(titleSlug) > titleBudget {
		titleSlug = trimSlug(titleSlug, titleBudget)
	}

	// If still over budget, shrink portal further
	total := len(portalSlug) + len(titleSlug)
	if total > budget {
		portalBudget := budget - len(titleSlug)
		portalBudget = max(portalBudget, 1)
		portalSlug = trimSlug(portalSlug, portalBudget)
	}

	return portalSlug, titleSlug
}

// trimSlug truncates a slug to maxLen, ensuring no trailing hyphen.
func trimSlug(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	s = s[:maxLen]
	return strings.TrimRight(s, "-")
}
