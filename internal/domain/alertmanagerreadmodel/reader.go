package alertmanagerreadmodel

import "context"

// AlertmanagerReader is the read-side interface for Alertmanager data.
type AlertmanagerReader interface {
	List(ctx context.Context, filters AlertmanagerFilters) ([]AlertmanagerView, error)
	Subscribe() <-chan struct{}
}
