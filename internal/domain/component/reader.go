package component

import "context"

// ListOptions defines filtering options for listing components.
type ListOptions struct {
	PortalRef string
	Group     string
}

// ComponentReader provides read access to component projections.
type ComponentReader interface {
	List(ctx context.Context, opts ListOptions) ([]ComponentView, error)
	Subscribe() <-chan struct{}
}
