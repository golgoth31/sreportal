package portal

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
}

// RemoteSyncView captures the last remote sync state.
type RemoteSyncView struct {
	LastSyncTime  string
	LastSyncError string
	RemoteTitle   string
	FQDNCount     int
}

// PortalFilters are the criteria for listing portals.
type PortalFilters struct {
	Namespace string
}
