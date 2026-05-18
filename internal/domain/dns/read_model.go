package dns

import "time"

// FQDNView is the read-side projection of an FQDN, pre-aggregated by controllers.
// Unlike FQDN (write model), it carries portal context and group membership.
type FQDNView struct {
	Name        string
	Source      Source
	SourceType  string // external-dns source type (e.g. "service", "ingress", "dnsendpoint")
	Groups      []string
	Description string
	RecordType  string
	Targets     []string
	LastSeen    time.Time
	Portals     []string // multiple portals possible after dedup
	Namespace   string   // DNS CR namespace
	OriginRef   *ResourceRef
	SyncStatus  string
}

// FirstPortal returns the first portal in the view, or "" if none.
// Temporary helper while the read path is mid-refactor (Tasks 4.3+ will
// rewrite consumers to handle the full Portals slice).
func (v FQDNView) FirstPortal() string {
	if len(v.Portals) == 0 {
		return ""
	}
	return v.Portals[0]
}

// FQDNFilters are the criteria for listing FQDNs.
type FQDNFilters struct {
	Portal    string
	Namespace string
	Source    string
	Search    string // substring match on Name (case-insensitive)
}
