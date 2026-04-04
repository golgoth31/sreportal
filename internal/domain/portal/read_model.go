package portal

// PortalFeatures contains the feature toggles for a portal.
type PortalFeatures struct {
	DNS           bool
	Releases      bool
	NetworkPolicy bool
	Alerts        bool
	StatusPage    bool
}

// PortalAuthView is the read-side projection of spec.auth for RPC authentication.
type PortalAuthView struct {
	APIKey *PortalAPIKeyAuthView
	JWT    *PortalJWTAuthView
}

// Enabled returns true if at least one authentication method is enabled.
func (a *PortalAuthView) Enabled() bool {
	if a == nil {
		return false
	}
	if a.APIKey != nil && a.APIKey.Enabled {
		return true
	}
	if a.JWT != nil && a.JWT.Enabled && len(a.JWT.Issuers) > 0 {
		return true
	}
	return false
}

// PortalAPIKeyAuthView holds API key auth settings for the read model.
type PortalAPIKeyAuthView struct {
	Enabled    bool
	HeaderName string
	SecretName string
	SecretKey  string
}

// PortalJWTAuthView holds JWT auth settings for the read model.
type PortalJWTAuthView struct {
	Enabled bool
	Issuers []PortalJWTIssuerView
}

// PortalJWTIssuerView is one JWT issuer in the read model.
type PortalJWTIssuerView struct {
	Name           string
	IssuerURL      string
	Audience       string
	JWKSURL        string
	RequiredClaims map[string]string
}

// PortalFeatureAuthOverridesView holds optional per-feature auth overrides.
type PortalFeatureAuthOverridesView struct {
	DNS           *PortalAuthView
	Releases      *PortalAuthView
	NetworkPolicy *PortalAuthView
	Alerts        *PortalAuthView
	StatusPage    *PortalAuthView
}

// PortalView is the read-side projection of a Portal, pre-aggregated by the controller.
type PortalView struct {
	Name       string
	Title      string
	Main       bool
	SubPath    string
	Namespace  string
	Ready      bool
	IsRemote   bool
	URL        string          // Remote URL, empty for local portals
	RemoteSync *RemoteSyncView // Non-nil only for remote portals with sync status
	Features   PortalFeatures
	// Auth is spec.auth (nil = inherit from main for local portals when resolving writes).
	Auth *PortalAuthView
	// AuthExplicit is true when spec.auth was set on the Portal CR (even if all methods are disabled).
	AuthExplicit bool
	// FeatureAuth is spec.features.auth (per-feature overrides).
	FeatureAuth *PortalFeatureAuthOverridesView
}

// RemoteSyncView captures the last remote sync state.
type RemoteSyncView struct {
	LastSyncTime   string
	LastSyncError  string
	RemoteTitle    string
	FQDNCount      int
	RemoteFeatures *PortalFeatures
}

// PortalFilters are the criteria for listing portals.
type PortalFilters struct {
	Namespace string
}
