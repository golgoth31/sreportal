package release

import "context"

// ReleaseWriter allows controllers to push release projections into the store.
// resourceKey is namespace/name of the Release CR (same convention as other read stores).
type ReleaseWriter interface {
	Replace(ctx context.Context, resourceKey string, entries []EntryView) error
	Delete(ctx context.Context, resourceKey string) error
}
