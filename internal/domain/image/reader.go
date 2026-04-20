package image

import "context"

// ImageReader provides read access to image projections.
type ImageReader interface {
	List(ctx context.Context, filters ImageFilters) ([]ImageView, error)
	Count(ctx context.Context, filters ImageFilters) (int, error)
	Subscribe() <-chan struct{}
}
