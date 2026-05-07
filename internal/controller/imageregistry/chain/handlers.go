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

// Package chain contains Chain-of-Responsibility handlers for the ImageRegistry controller.
package chain

import (
	"time"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// Condition type and reason constants for the ImageRegistry Ready condition.
const (
	ConditionTypeReady = "Ready"

	ReasonInvalidSpec  = "InvalidSpec"
	ReasonReconciled   = "Reconciled"
	ReasonLookupFailed = "LookupFailed"
	ReconciledMessage  = "image registry ready"
)

// DueImage carries the spec entry and its prior status entry (nil if never checked).
type DueImage struct {
	Spec   sreportalv1alpha1.ImageRegistrySpecEntry
	Status *sreportalv1alpha1.ImageRegistryStatusEntry // nil = never checked
}

// Resolution holds the lookup result for a single due image.
type Resolution struct {
	// Key matches DueImage.Spec.Key.
	Key string

	// LatestVersion is the highest semver tag found. Empty when not applicable.
	LatestVersion string
	// UpgradeAvailable is true when LatestVersion > OriginalTag (semver).
	UpgradeAvailable bool
	// LastCheckedAt is the timestamp of this lookup.
	LastCheckedAt time.Time
	// LastError carries a lookup failure message. Empty on success.
	LastError string
}

// ChainData is the typed shared state passed between ImageRegistry handlers.
type ChainData struct {
	// DueImages is populated by SelectDueImagesHandler: images whose
	// LastCheckedAt+24h < now (or never checked).
	DueImages []DueImage

	// Resolutions is populated by ResolveLatestVersionsHandler, keyed by
	// ImageRegistrySpecEntry.Key.
	Resolutions map[string]Resolution

	// RequeueAfter carries the handler-computed requeue duration. The controller
	// uses this if set; otherwise it falls back to its default 24h±1h interval.
	RequeueAfter time.Duration
}
