package incident

import "context"

// ListOptions defines filtering options for listing incidents.
type ListOptions struct {
	PortalRef string
	Phase     IncidentPhase
}

// IncidentReader provides read access to incident projections.
type IncidentReader interface {
	List(ctx context.Context, opts ListOptions) ([]IncidentView, error)
	Subscribe() <-chan struct{}
}
