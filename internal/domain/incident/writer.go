package incident

import "context"

// IncidentWriter allows controllers to push incident projections into the store.
type IncidentWriter interface {
	Replace(ctx context.Context, key string, views []IncidentView) error
	Delete(ctx context.Context, key string) error
}
