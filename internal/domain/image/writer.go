package image

import "context"

// ImageWriter pushes image projections into the readstore at workload and
// portal granularity.
type ImageWriter interface {
	// ReplaceWorkload sets (or replaces) the contribution of a single workload
	// to a portal's image projection.
	ReplaceWorkload(ctx context.Context, portalRef string, wk WorkloadKey, images []ImageView) error

	// DeleteWorkloadAllPortals removes a workload's contribution from every
	// portal that referenced it.
	DeleteWorkloadAllPortals(ctx context.Context, wk WorkloadKey) error

	// ReplaceAll atomically replaces the full projection of a portal, keyed by
	// workload. Used for full rescans triggered by ImageInventory CR changes.
	ReplaceAll(ctx context.Context, portalRef string, byWorkload map[WorkloadKey][]ImageView) error

	// DeletePortal removes all projections for a portal (e.g. when the
	// ImageInventory CR is deleted).
	DeletePortal(ctx context.Context, portalRef string) error
}
