package component

import "context"

// ComponentWriter allows controllers to push component projections into the store.
type ComponentWriter interface {
	Replace(ctx context.Context, key string, views []ComponentView) error
	Delete(ctx context.Context, key string) error
}
