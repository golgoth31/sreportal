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
	"sync"
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

// fakeImageWriter records calls to ReplaceForNamespace / RemoveForNamespace so
// tests can assert which scopes were written and with which views.
type fakeImageWriter struct {
	mu       sync.Mutex
	scopes   map[fakeScopeKey][]domainimage.ImageView
	removes  []fakeScopeKey
	replaces []fakeScopeKey // ordered record of replace calls
}

type fakeScopeKey struct {
	Portal, Host, Namespace string
}

func newFakeImageWriter() *fakeImageWriter {
	return &fakeImageWriter{scopes: map[fakeScopeKey][]domainimage.ImageView{}}
}

func (w *fakeImageWriter) ReplaceForNamespace(_ context.Context, portal, host, namespace string, views []domainimage.ImageView) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	k := fakeScopeKey{Portal: portal, Host: host, Namespace: namespace}
	w.scopes[k] = append([]domainimage.ImageView(nil), views...)
	w.replaces = append(w.replaces, k)
	return nil
}

func (w *fakeImageWriter) RemoveForNamespace(_ context.Context, portal, host, namespace string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	k := fakeScopeKey{Portal: portal, Host: host, Namespace: namespace}
	delete(w.scopes, k)
	w.removes = append(w.removes, k)
	return nil
}

func newFetchRemoteImagesScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(scheme))
	return scheme
}

func TestFetchRemoteImagesHandlerNoOpWhenLocal(t *testing.T) {
	t.Parallel()
	scheme := newFetchRemoteImagesScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	writer := newFakeImageWriter()
	h := NewFetchRemoteImagesHandler(cli, remoteclient.NewCache(), writer)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "local-inv", Namespace: tNsDefault},
		Spec:       sreportalv1alpha1.ImageInventorySpec{PortalRef: tPortalMain, IsRemote: false},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}
	require.NoError(t, h.Handle(context.Background(), rc))
	require.Empty(t, writer.replaces, "local-path inventory must not call the writer")
	require.Empty(t, writer.removes)
}

func TestFetchRemoteImagesHandlerPopulatesFromRemote(t *testing.T) {
	t.Parallel()
	scheme := newFetchRemoteImagesScheme(t)

	mux := http.NewServeMux()
	mockSvc := &fetchRemoteImagesMockService{
		images: []*sreportalv1.Image{
			{
				Registry:   tRegistryGhcr,
				Repository: tRepoAcmeAPI,
				Tag:        tVersion123,
				TagType:    tTagTypeSemver,
				Workloads: []*sreportalv1.WorkloadRef{
					{Kind: tKindDeploy, Namespace: tNsDefault, Name: tNameAPI, Container: tPortalMain, Source: tFieldSpec},
				},
			},
		},
	}
	mux.Handle(sreportalv1connect.NewImageServiceHandler(mockSvc))
	server := httptest.NewServer(mux)
	defer server.Close()

	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalRemote, Namespace: tNsDefault},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:  tTypeRemote,
			Remote: &sreportalv1alpha1.RemotePortalSpec{URL: server.URL, Portal: tPortalMain},
		},
	}
	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalRemoteInv, Namespace: tNsDefault},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef: tPortalRemote,
			IsRemote:  true,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, inv).WithStatusSubresource(inv).Build()
	writer := newFakeImageWriter()

	h := NewFetchRemoteImagesHandler(cli, remoteclient.NewCache(), writer)
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}
	require.NoError(t, h.Handle(context.Background(), rc))

	// One scope: (tPortalRemote, ghcr.io, default).
	wantKey := fakeScopeKey{Portal: tPortalRemote, Host: tRegistryGhcr, Namespace: tNsDefault}
	views, ok := writer.scopes[wantKey]
	require.True(t, ok, "expected scope %+v in writer", wantKey)
	require.Len(t, views, 1)
	require.Equal(t, tRegistryGhcr, views[0].Registry)
	require.Equal(t, tRepoAcmeAPI, views[0].Repository)
	require.Equal(t, domainimage.TagTypeSemver, views[0].TagType)
	require.Equal(t, tPortalRemote, views[0].PortalRef)
	require.Len(t, views[0].Workloads, 1)
	require.Equal(t, domainimage.ContainerSourceSpec, views[0].Workloads[0].Source)

	// inv.Status.Registries should record the scope (Hash empty for remote-path).
	require.Len(t, inv.Status.Registries, 1)
	require.Equal(t, "", inv.Status.Registries[0].Hash)
	require.Equal(t, tRegistryGhcr, inv.Status.Registries[0].Host)
	require.Equal(t, tNsDefault, inv.Status.Registries[0].Namespace)
}

func TestFetchRemoteImagesHandlerErrorWhenPortalMissing(t *testing.T) {
	t.Parallel()
	scheme := newFetchRemoteImagesScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	writer := newFakeImageWriter()
	h := NewFetchRemoteImagesHandler(cli, remoteclient.NewCache(), writer)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-inv", Namespace: tNsDefault},
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
	t.Parallel()
	scheme := newFetchRemoteImagesScheme(t)
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "main-portal", Namespace: tNsDefault},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Main", Main: true},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()
	writer := newFakeImageWriter()
	h := NewFetchRemoteImagesHandler(cli, remoteclient.NewCache(), writer)

	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-inv", Namespace: tNsDefault},
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

func TestFetchRemoteImagesHandlerBucketsByHostAndNamespace(t *testing.T) {
	t.Parallel()
	scheme := newFetchRemoteImagesScheme(t)

	mux := http.NewServeMux()
	mockSvc := &fetchRemoteImagesMockService{
		images: []*sreportalv1.Image{
			{
				Registry:   tRegistryGhcr,
				Repository: tRepoAcmeAPI,
				Tag:        tVersion123,
				TagType:    tTagTypeSemver,
				Workloads: []*sreportalv1.WorkloadRef{
					// Same image referenced from two namespaces -> two scopes.
					{Kind: tKindDeploy, Namespace: tNsDefault, Name: tNameAPI, Container: tPortalMain, Source: tFieldSpec},
					{Kind: tKindDeploy, Namespace: tNsKubeSystem, Name: tNameWeb, Container: tPortalMain, Source: tFieldSpec},
				},
			},
			{
				// Different host -> independent scope.
				Registry:   tRegistryDocker,
				Repository: "acme/web",
				Tag:        "2.0.0",
				TagType:    tTagTypeSemver,
				Workloads: []*sreportalv1.WorkloadRef{
					{Kind: tKindDeploy, Namespace: tNsDefault, Name: tNameAPI, Container: tContainerSidecar, Source: tFieldSpec},
				},
			},
		},
	}
	mux.Handle(sreportalv1connect.NewImageServiceHandler(mockSvc))
	server := httptest.NewServer(mux)
	defer server.Close()

	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalRemote, Namespace: tNsDefault},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:  tTypeRemote,
			Remote: &sreportalv1alpha1.RemotePortalSpec{URL: server.URL, Portal: tPortalMain},
		},
	}
	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalRemoteInv, Namespace: tNsDefault},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef: tPortalRemote,
			IsRemote:  true,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, inv).WithStatusSubresource(inv).Build()
	writer := newFakeImageWriter()

	h := NewFetchRemoteImagesHandler(cli, remoteclient.NewCache(), writer)
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}
	require.NoError(t, h.Handle(context.Background(), rc))

	// Three distinct (host, namespace) scopes are expected.
	require.Len(t, writer.scopes, 3)

	ghcrDefault := writer.scopes[fakeScopeKey{Portal: tPortalRemote, Host: tRegistryGhcr, Namespace: tNsDefault}]
	require.Len(t, ghcrDefault, 1, "ghcr.io/default scope must hold the API image")
	require.Equal(t, tRepoAcmeAPI, ghcrDefault[0].Repository)
	require.Len(t, ghcrDefault[0].Workloads, 1)
	require.Equal(t, tNsDefault, ghcrDefault[0].Workloads[0].Namespace)

	ghcrKubeSystem := writer.scopes[fakeScopeKey{Portal: tPortalRemote, Host: tRegistryGhcr, Namespace: tNsKubeSystem}]
	require.Len(t, ghcrKubeSystem, 1, "ghcr.io/kube-system scope must hold the API image bucketed for kube-system only")
	require.Equal(t, tRepoAcmeAPI, ghcrKubeSystem[0].Repository)
	require.Len(t, ghcrKubeSystem[0].Workloads, 1)
	require.Equal(t, tNsKubeSystem, ghcrKubeSystem[0].Workloads[0].Namespace)

	dockerDefault := writer.scopes[fakeScopeKey{Portal: tPortalRemote, Host: tRegistryDocker, Namespace: tNsDefault}]
	require.Len(t, dockerDefault, 1, "docker.io/default scope must hold the web image")
	require.Equal(t, "acme/web", dockerDefault[0].Repository)
}

func TestFetchRemoteImagesHandlerDropsMissingScopes(t *testing.T) {
	t.Parallel()
	scheme := newFetchRemoteImagesScheme(t)

	// Mock returns only the ghcr.io / default scope. The previous reconcile
	// (recorded in inv.Status.Registries) had an extra docker.io / kube-system
	// scope that must be dropped.
	mux := http.NewServeMux()
	mockSvc := &fetchRemoteImagesMockService{
		images: []*sreportalv1.Image{
			{
				Registry:   tRegistryGhcr,
				Repository: tRepoAcmeAPI,
				Tag:        tVersion123,
				TagType:    tTagTypeSemver,
				Workloads: []*sreportalv1.WorkloadRef{
					{Kind: tKindDeploy, Namespace: tNsDefault, Name: tNameAPI, Container: tPortalMain, Source: tFieldSpec},
				},
			},
		},
	}
	mux.Handle(sreportalv1connect.NewImageServiceHandler(mockSvc))
	server := httptest.NewServer(mux)
	defer server.Close()

	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalRemote, Namespace: tNsDefault},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:  tTypeRemote,
			Remote: &sreportalv1alpha1.RemotePortalSpec{URL: server.URL, Portal: tPortalMain},
		},
	}
	inv := &sreportalv1alpha1.ImageInventory{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalRemoteInv, Namespace: tNsDefault},
		Spec: sreportalv1alpha1.ImageInventorySpec{
			PortalRef: tPortalRemote,
			IsRemote:  true,
		},
		Status: sreportalv1alpha1.ImageInventoryStatus{
			Registries: []sreportalv1alpha1.ImageRegistryRef{
				{Host: tRegistryGhcr, Namespace: tNsDefault},      // still present
				{Host: tRegistryDocker, Namespace: tNsKubeSystem}, // gone -> must be removed
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, inv).WithStatusSubresource(inv).Build()
	writer := newFakeImageWriter()

	h := NewFetchRemoteImagesHandler(cli, remoteclient.NewCache(), writer)
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]{Resource: inv}
	require.NoError(t, h.Handle(context.Background(), rc))

	require.Len(t, writer.removes, 1, "the orphan scope must be removed once")
	require.Equal(t, fakeScopeKey{Portal: tPortalRemote, Host: tRegistryDocker, Namespace: tNsKubeSystem}, writer.removes[0])

	// The kept scope was rewritten via Replace, the orphan was Removed.
	_, hasGhcr := writer.scopes[fakeScopeKey{Portal: tPortalRemote, Host: tRegistryGhcr, Namespace: tNsDefault}]
	require.True(t, hasGhcr)
}
