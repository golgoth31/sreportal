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

package grpc

import (
	"context"

	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
)

// FeatureChecker extracts a feature flag from PortalFeatures.
type FeatureChecker func(domainportal.PortalFeatures) bool

// Pre-defined feature checkers.
var (
	CheckDNS           FeatureChecker = func(f domainportal.PortalFeatures) bool { return f.DNS }
	CheckReleases      FeatureChecker = func(f domainportal.PortalFeatures) bool { return f.Releases }
	CheckNetworkPolicy FeatureChecker = func(f domainportal.PortalFeatures) bool { return f.NetworkPolicy }
	CheckAlerts        FeatureChecker = func(f domainportal.PortalFeatures) bool { return f.Alerts }
	CheckStatusPage    FeatureChecker = func(f domainportal.PortalFeatures) bool { return f.StatusPage }
)

// IsFeatureEnabled looks up a portal by name and checks whether the given
// feature is enabled. It returns true (not gated) when:
//   - reader is nil (no portal store available)
//   - portalName is empty (aggregated query, not portal-specific)
//   - portal is not found (deleted portal, underlying reader returns empty anyway)
func IsFeatureEnabled(ctx context.Context, reader domainportal.PortalReader, portalName string, check FeatureChecker) (bool, error) {
	if reader == nil || portalName == "" {
		return true, nil
	}

	portals, err := reader.List(ctx, domainportal.PortalFilters{})
	if err != nil {
		return false, err
	}

	for _, p := range portals {
		if p.Name == portalName {
			return check(p.Features), nil
		}
	}

	// Portal not found — not gated.
	return true, nil
}
