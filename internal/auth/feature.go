package auth

import "strings"

// AuthFeature identifies a portal feature for per-feature auth overrides.
type AuthFeature int

const (
	// FeatureUnset is used when the procedure does not map to a known feature.
	FeatureUnset AuthFeature = iota
	FeatureReleases
	FeatureStatusPage
)

func authFeatureForProcedure(proc string) (AuthFeature, bool) {
	switch proc {
	case "/sreportal.v1.ReleaseService/AddRelease":
		return FeatureReleases, true
	default:
		if strings.HasPrefix(proc, "/sreportal.v1.StatusService/") {
			return FeatureStatusPage, true
		}
	}
	return FeatureUnset, false
}
