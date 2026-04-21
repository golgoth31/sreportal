package grpc_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	imagev1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	imagestore "github.com/golgoth31/sreportal/internal/readstore/image"
)

func TestListImages(t *testing.T) {
	store := imagestore.NewStore()
	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "api"}
	require.NoError(t, store.ReplaceWorkload(context.Background(), "main", wk, []domainimage.ImageView{
		{
			PortalRef:  "main",
			Registry:   "ghcr.io",
			Repository: "acme/api",
			Tag:        "1.0.0",
			TagType:    domainimage.TagTypeSemver,
			Workloads:  []domainimage.WorkloadRef{{Kind: "Deployment", Namespace: "default", Name: "api", Container: "main"}},
		},
	}))

	svc := svcgrpc.NewImageService(store, nil)
	resp, err := svc.ListImages(context.Background(), connect.NewRequest(&imagev1.ListImagesRequest{
		Portal:         "main",
		Search:         "acme/",
		RegistryFilter: "ghcr.io",
		TagTypeFilter:  "semver",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Images, 1)
	require.Equal(t, int32(1), resp.Msg.TotalCount)
}
