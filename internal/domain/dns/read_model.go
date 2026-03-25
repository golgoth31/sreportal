package dns

import "time"

// FQDNView is the read-side projection of an FQDN, pre-aggregated by controllers.
// Unlike FQDN (write model), it carries portal context and group membership.
type FQDNView struct {
	Name        string
	Source      Source
	Groups      []string
	Description string
	RecordType  string
	Targets     []string
	LastSeen    time.Time
	PortalName  string // DNS CR name (== portal via portalRef)
	Namespace   string // DNS CR namespace
	OriginRef   *ResourceRef
	SyncStatus  string
}

// FQDNFilters are the criteria for listing FQDNs.
type FQDNFilters struct {
	Portal    string
	Namespace string
	Source    string
	Search    string // substring match on Name (case-insensitive)
}
