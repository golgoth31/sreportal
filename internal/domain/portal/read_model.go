package portal

// PortalFeatures contains the feature toggles for a portal.
type PortalFeatures struct {
	DNS            bool
	Releases       bool
	NetworkPolicy  bool
	Alerts         bool
	StatusPage     bool
	ImageInventory bool
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
