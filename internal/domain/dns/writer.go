package dns

import "context"

// FQDNWriter pushes pre-aggregated FQDN read models into the store.
// Used by controllers after reconciliation.
type FQDNWriter interface {
	// Replace atomically replaces all FQDNs contributed by a single DNSRecord,
	// recording the portalRef so the store can maintain its portal index.
	Replace(ctx context.Context, recordKey, portalRef string, fqdns []FQDNView) error
	// Delete removes all FQDNs contributed by a single DNSRecord.
	Delete(ctx context.Context, recordKey string) error
	// AnnotateOwner records the DNS owner (namespace/name) of a DNSRecord so
	// conflicts can be filtered per DNS. Called after a successful Replace.
	AnnotateOwner(recordKey, dnsNamespace, dnsName string)
}
