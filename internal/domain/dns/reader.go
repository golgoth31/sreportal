package dns

import "context"

// FQDNReader provides read access to the FQDN projection.
// Consumed by gRPC and MCP services.
type FQDNReader interface {
	// List returns FQDNs matching the given filters, sorted by (Name, RecordType).
	List(ctx context.Context, filters FQDNFilters) ([]FQDNView, error)
	// Get returns a single FQDN by exact name and record type.
	// Returns ErrFQDNNotFound if no match exists.
	Get(ctx context.Context, name, recordType string) (FQDNView, error)
	// Count returns the number of FQDNs matching the given filters.
	Count(ctx context.Context, filters FQDNFilters) (int, error)
	// Subscribe returns a channel that is closed when the store is mutated.
	// Callers must call Subscribe() again after each notification.
	Subscribe() <-chan struct{}
}
