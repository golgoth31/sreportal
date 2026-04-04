package auth

import domainportal "github.com/golgoth31/sreportal/internal/domain/portal"

func featureOverride(
	o *domainportal.PortalFeatureAuthOverridesView,
	f AuthFeature,
) *domainportal.PortalAuthView {
	if o == nil {
		return nil
	}
	switch f {
	case FeatureReleases:
		return o.Releases
	case FeatureStatusPage:
		return o.StatusPage
	case FeatureUnset:
		return nil
	default:
		return nil
	}
}

// effectiveAuth resolves which auth view applies for a write RPC.
func effectiveAuth(
	main *domainportal.PortalView,
	target *domainportal.PortalView,
	feat AuthFeature,
) *domainportal.PortalAuthView {
	if target == nil {
		return nil
	}
	if o := featureOverride(target.FeatureAuth, feat); o != nil {
		if o.Enabled() {
			return o
		}
		return nil
	}
	if target.AuthExplicit {
		if target.Auth != nil && target.Auth.Enabled() {
			return target.Auth
		}
		return nil
	}
	if target.IsRemote {
		return nil
	}
	if main == nil {
		return nil
	}
	return main.Auth
}
