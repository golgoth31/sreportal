package maintenance

import "context"

// MaintenanceWriter allows controllers to push maintenance projections into the store.
type MaintenanceWriter interface {
	Replace(ctx context.Context, key string, views []MaintenanceView) error
	Delete(ctx context.Context, key string) error
}
