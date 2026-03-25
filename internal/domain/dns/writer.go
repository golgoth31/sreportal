package dns

import "context"

// FQDNWriter pushes pre-aggregated FQDN read models into the store.
// Used by controllers after reconciliation.
type FQDNWriter interface {
	// Replace atomically replaces all FQDNs for a given DNS resource key.
	Replace(ctx context.Context, resourceKey string, fqdns []FQDNView) error
	// Delete removes all FQDNs for a given DNS resource key.
	Delete(ctx context.Context, resourceKey string) error
}
