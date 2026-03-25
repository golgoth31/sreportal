package alertmanagerreadmodel

import "context"

// AlertmanagerWriter is the write-side interface for Alertmanager data.
type AlertmanagerWriter interface {
	Replace(ctx context.Context, key string, view AlertmanagerView) error
	Delete(ctx context.Context, key string) error
}
