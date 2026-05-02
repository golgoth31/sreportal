/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package chain

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	sreportalv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/remoteclient"
)

type fetchRemoteImagesMockService struct {
	sreportalv1connect.UnimplementedImageServiceHandler
	images []*sreportalv1.Image
	err    error
}

func (m *fetchRemoteImagesMockService) ListImages(
	_ context.Context,
	_ *connect.Request[sreportalv1.ListImagesRequest],
) (*connect.Response[sreportalv1.ListImagesResponse], error) {
	if m.err != nil {
		return nil, m.err
	}
	return connect.NewResponse(&sreportalv1.ListImagesResponse{
		Images:     m.images,
		TotalCount: int32(len(m.images)),
	}), nil
}

func newFetchRemoteImagesScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(scheme))
	return scheme
}

func TestFetchRemoteImagesHandlerNoOpWhenLocal(t *testing.T) {
	scheme := newFetchRemoteImagesScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	h := NewFetchRemoteImagesHandler(cli, remoteclient.NewCache())

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "local-inv", Namespace: "default"},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: "main", IsRemote: false},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Nil(t, rc.Data.ByWorkload)
}

func TestFetchRemoteImagesHandlerPopulatesFromRemote(t *testing.T) {
	scheme := newFetchRemoteImagesScheme(t)

	mux := http.NewServeMux()
	mockSvc := &fetchRemoteImagesMockService{
		images: []*sreportalv1.Image{
			{
				Registry:   "ghcr.io",
				Repository: "acme/api",
				Tag:        "1.2.3",
				TagType:    "semver",
				Workloads: []*sreportalv1.WorkloadRef{
					{Kind: "Deployment", Namespace: "default", Name: "api", Container: "main", Source: "spec"},
				},
			},
		},
	}
	mux.Handle(sreportalv1connect.NewImageServiceHandler(mockSvc))
	server := httptest.NewServer(mux)
	defer server.Close()

	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-portal", Namespace: "default"},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:  "Remote",
			Remote: &sreportalv1alpha1.RemotePortalSpec{URL: server.URL, Portal: "main"},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()

	h := NewFetchRemoteImagesHandler(cli, remoteclient.NewCache())
	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-remote-portal", Namespace: "default"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef: "remote-portal",
			IsRemote:  true,
		},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.NotNil(t, rc.Data.ByWorkload)

	wk := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "api"}
	views, ok := rc.Data.ByWorkload[wk]
	require.True(t, ok, "expected workload key to be present")
	require.Len(t, views, 1)
	require.Equal(t, "ghcr.io", views[0].Registry)
	require.Equal(t, "acme/api", views[0].Repository)
	require.Equal(t, domainimage.TagTypeSemver, views[0].TagType)
	require.Equal(t, "remote-portal", views[0].PortalRef)
	require.Len(t, views[0].Workloads, 1)
	require.Equal(t, domainimage.ContainerSourceSpec, views[0].Workloads[0].Source)
}

func TestFetchRemoteImagesHandlerErrorWhenPortalMissing(t *testing.T) {
	scheme := newFetchRemoteImagesScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	h := NewFetchRemoteImagesHandler(cli, remoteclient.NewCache())

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-inv", Namespace: "default"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef: "missing-portal",
			IsRemote:  true,
		},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}
	err := h.Handle(context.Background(), rc)
	require.Error(t, err)
}

func TestFetchRemoteImagesHandlerErrorWhenPortalNotRemote(t *testing.T) {
	scheme := newFetchRemoteImagesScheme(t)
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "main-portal", Namespace: "default"},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Main", Main: true},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()
	h := NewFetchRemoteImagesHandler(cli, remoteclient.NewCache())

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-inv", Namespace: "default"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef: "main-portal",
			IsRemote:  true,
		},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}
	err := h.Handle(context.Background(), rc)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no remote configuration")
}

func TestFetchRemoteImagesHandlerBucketsWorkloadsCorrectly(t *testing.T) {
	scheme := newFetchRemoteImagesScheme(t)

	mux := http.NewServeMux()
	mockSvc := &fetchRemoteImagesMockService{
		images: []*sreportalv1.Image{
			{
				Registry:   "ghcr.io",
				Repository: "acme/api",
				Tag:        "1.2.3",
				TagType:    "semver",
				Workloads: []*sreportalv1.WorkloadRef{
					{Kind: "Deployment", Namespace: "default", Name: "api", Container: "main", Source: "spec"},
					{Kind: "Deployment", Namespace: "default", Name: "web", Container: "main", Source: "spec"},
				},
			},
			{
				Registry:   "ghcr.io",
				Repository: "acme/web",
				Tag:        "2.0.0",
				TagType:    "semver",
				Workloads: []*sreportalv1.WorkloadRef{
					{Kind: "Deployment", Namespace: "default", Name: "api", Container: "sidecar", Source: "spec"},
				},
			},
		},
	}
	mux.Handle(sreportalv1connect.NewImageServiceHandler(mockSvc))
	server := httptest.NewServer(mux)
	defer server.Close()

	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-portal", Namespace: "default"},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:  "Remote",
			Remote: &sreportalv1alpha1.RemotePortalSpec{URL: server.URL, Portal: "main"},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()

	h := NewFetchRemoteImagesHandler(cli, remoteclient.NewCache())
	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-remote-portal", Namespace: "default"},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef: "remote-portal",
			IsRemote:  true,
		},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.NotNil(t, rc.Data.ByWorkload)

	w1 := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "api"}
	w2 := domainimage.WorkloadKey{Kind: "Deployment", Namespace: "default", Name: "web"}

	require.Len(t, rc.Data.ByWorkload[w1], 2, "expected 2 views in bucket W1 (one per image referencing it)")
	require.Len(t, rc.Data.ByWorkload[w2], 1, "expected 1 view in bucket W2")

	for _, v := range rc.Data.ByWorkload[w1] {
		require.Len(t, v.Workloads, 1, "each ImageView must hold exactly one WorkloadRef")
		require.Equal(t, "api", v.Workloads[0].Name)
	}
	for _, v := range rc.Data.ByWorkload[w2] {
		require.Len(t, v.Workloads, 1, "each ImageView must hold exactly one WorkloadRef")
		require.Equal(t, "web", v.Workloads[0].Name)
	}
}
