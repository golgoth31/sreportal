package release

import "context"

// ReleaseWriter allows controllers to push release projections into the store.
type ReleaseWriter interface {
	Replace(ctx context.Context, day string, entries []EntryView) error
	Delete(ctx context.Context, day string) error
}
