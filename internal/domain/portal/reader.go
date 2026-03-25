package portal

import "context"

// PortalReader is the read-side interface for portal data.
type PortalReader interface {
	List(ctx context.Context, filters PortalFilters) ([]PortalView, error)
	Subscribe() <-chan struct{}
}
