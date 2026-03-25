package release

import "context"

// ReleaseReader provides read access to release projections.
type ReleaseReader interface {
	ListEntries(ctx context.Context, day string) ([]EntryView, error)
	ListDays(ctx context.Context) ([]string, error)
	Subscribe() <-chan struct{}
}
