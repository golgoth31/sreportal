package portal

import "context"

// PortalWriter is the write-side interface for portal data.
type PortalWriter interface {
	Replace(ctx context.Context, key string, portal PortalView) error
	Delete(ctx context.Context, key string) error
}
