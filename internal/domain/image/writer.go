package image

import "context"

// ImageWriter pushes image projections into the readstore at the granularity
// of (portalRef, host, namespace). The single writer in production is the
// ImageRegistry controller; remote-portal projections also reuse this same
// interface to write shadow contributions.
type ImageWriter interface {
	// ReplaceForNamespace replaces the full set of images contributed by the
	// (portalRef, host, namespace) tuple with the provided views. Contributions
	// from other namespaces under the same (registry, repository, tag) are
	// preserved and re-aggregated.
	ReplaceForNamespace(ctx context.Context, portalRef, host, namespace string, views []ImageView) error

	// RemoveForNamespace removes every contribution previously associated with
	// the (portalRef, host, namespace) tuple. Used by ImageRegistry finalizer
	// and by parent ImageInventory cleanup.
	RemoveForNamespace(ctx context.Context, portalRef, host, namespace string) error
}
