package grpc

import (
	"context"

	"connectrpc.com/connect"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	imagev1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// ImageService implements ImageServiceHandler.
type ImageService struct {
	sreportalv1connect.UnimplementedImageServiceHandler
	reader       domainimage.ImageReader
	portalReader domainportal.PortalReader
}

func NewImageService(reader domainimage.ImageReader, portalReader domainportal.PortalReader) *ImageService {
	return &ImageService{reader: reader, portalReader: portalReader}
}

func (s *ImageService) ListImages(
	ctx context.Context,
	req *connect.Request[imagev1.ListImagesRequest],
) (*connect.Response[imagev1.ListImagesResponse], error) {
	if enabled, err := IsFeatureEnabled(ctx, s.portalReader, req.Msg.Portal, CheckImages); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if !enabled {
		return connect.NewResponse(&imagev1.ListImagesResponse{}), nil
	}

	views, err := s.reader.List(ctx, domainimage.ImageFilters{
		Portal:   req.Msg.Portal,
		Search:   req.Msg.Search,
		Registry: req.Msg.RegistryFilter,
		TagType:  req.Msg.TagTypeFilter,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	images := make([]*imagev1.Image, 0, len(views))
	for _, v := range views {
		workloads := make([]*imagev1.WorkloadRef, 0, len(v.Workloads))
		for _, w := range v.Workloads {
			workloads = append(workloads, &imagev1.WorkloadRef{
				Kind:      w.Kind,
				Namespace: w.Namespace,
				Name:      w.Name,
				Container: w.Container,
			})
		}
		images = append(images, &imagev1.Image{
			Registry:   v.Registry,
			Repository: v.Repository,
			Tag:        v.Tag,
			TagType:    string(v.TagType),
			Workloads:  workloads,
		})
	}

	return connect.NewResponse(&imagev1.ListImagesResponse{
		Images:     images,
		TotalCount: int32(len(images)),
	}), nil
}
