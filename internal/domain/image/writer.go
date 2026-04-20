package image

import "context"

// ImageWriter pushes image projections into the readstore.
type ImageWriter interface {
	Replace(ctx context.Context, portalRef string, images []ImageView) error
	Delete(ctx context.Context, portalRef string) error
}
