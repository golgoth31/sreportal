package release

import "context"

// ReleaseReader provides read access to release projections.
// When portal is empty, ListDays and ListEntries aggregate across all portals.
type ReleaseReader interface {
	ListEntries(ctx context.Context, day, portal string) ([]EntryView, error)
	ListDays(ctx context.Context, portal string) ([]string, error)
	Subscribe() <-chan struct{}
}
