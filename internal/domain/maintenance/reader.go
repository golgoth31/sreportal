package maintenance

import "context"

// ListOptions defines filtering options for listing maintenances.
type ListOptions struct {
	PortalRef string
	Phase     MaintenancePhase
}

// MaintenanceReader provides read access to maintenance projections.
type MaintenanceReader interface {
	List(ctx context.Context, opts ListOptions) ([]MaintenanceView, error)
	Subscribe() <-chan struct{}
}
