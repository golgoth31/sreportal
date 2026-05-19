package dns

import "time"

// ConflictEvent records that two DNSRecords produced different targets for
// the same (fqdn, recordType) key. The first writer keeps its data; the
// loser is the rejected writer.
type ConflictEvent struct {
	FQDNKey      ConflictFQDNKey
	WinnerRecord string // resourceKey of the existing winner
	LoserRecord  string // resourceKey of the rejected writer
	PortalRef    string
	At           time.Time
}

// ConflictFQDNKey uniquely identifies an (fqdn, recordType) pair in conflict
// reports. It is distinct from any internal store key types.
type ConflictFQDNKey struct {
	Name       string
	RecordType string
}

// FQDNConflictReader exposes recent conflict events scoped to a DNS owner.
type FQDNConflictReader interface {
	// Conflicts returns conflict events whose loser DNSRecord is owned by the
	// given DNS. Pass empty strings to return all events.
	Conflicts(dnsNamespace, dnsName string) []ConflictEvent
}
