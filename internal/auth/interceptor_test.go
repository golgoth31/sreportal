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

package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/auth"
	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	releasev1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
	readstorerelease "github.com/golgoth31/sreportal/internal/readstore/release"
	releaseservice "github.com/golgoth31/sreportal/internal/release"
)

type fakePortalReader struct {
	views []domainportal.PortalView
}

func (f *fakePortalReader) List(_ context.Context, filters domainportal.PortalFilters) ([]domainportal.PortalView, error) {
	var out []domainportal.PortalView
	for _, v := range f.views {
		if filters.Namespace != "" && v.Namespace != filters.Namespace {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

func (f *fakePortalReader) Subscribe() <-chan struct{} {
	return make(chan struct{})
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

func setupReleaseServer(t *testing.T, k8sClient client.Client, resolver *auth.PortalResolver) sreportalv1connect.ReleaseServiceClient {
	t.Helper()
	svc := releaseservice.NewService(k8sClient, "default", "main")
	reader := readstorerelease.NewReleaseStore()
	grpcSvc := svcgrpc.NewReleaseService(reader, svc, 30*24*time.Hour, nil, nil)

	var opts []connect.HandlerOption
	if resolver != nil {
		opts = append(opts, connect.WithInterceptors(auth.PortalAuthInterceptor(resolver)))
	}

	mux := http.NewServeMux()
	path, handler := sreportalv1connect.NewReleaseServiceHandler(grpcSvc, opts...)
	mux.Handle(path, handler)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return sreportalv1connect.NewReleaseServiceClient(server.Client(), server.URL)
}

func mainPortalWithAPIKeyAuth() []domainportal.PortalView {
	return []domainportal.PortalView{
		{
			Name:         "main",
			Namespace:    "default",
			Main:         true,
			AuthExplicit: true,
			Auth: &domainportal.PortalAuthView{
				APIKey: &domainportal.PortalAPIKeyAuthView{
					Enabled:    true,
					HeaderName: "X-API-Key",
					SecretName: "key",
					SecretKey:  "api-key",
				},
			},
		},
	}
}

func TestInterceptor_ProtectedEndpoint_NoAuth_Rejected(t *testing.T) {
	scheme := testScheme(t)
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Main", Main: true},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "key", Namespace: "default"},
		Data:       map[string][]byte{"api-key": []byte("my-secret-key")},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, secret).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()
	r := auth.NewPortalResolver(k8s, &fakePortalReader{views: mainPortalWithAPIKeyAuth()}, "default")
	defer r.Close()

	releaseClient := setupReleaseServer(t, k8s, r)

	_, err := releaseClient.AddRelease(context.Background(), connect.NewRequest(&releasev1.ReleaseEntry{
		Type:    "deployment",
		Version: "v1.0.0",
		Origin:  "ci",
		Date:    timestamppb.New(time.Now()),
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestInterceptor_ProtectedEndpoint_ValidAuth_Passes(t *testing.T) {
	scheme := testScheme(t)
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Main", Main: true},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "key", Namespace: "default"},
		Data:       map[string][]byte{"api-key": []byte("my-secret-key")},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, secret).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()
	r := auth.NewPortalResolver(k8s, &fakePortalReader{views: mainPortalWithAPIKeyAuth()}, "default")
	defer r.Close()

	releaseClient := setupReleaseServer(t, k8s, r)

	req := connect.NewRequest(&releasev1.ReleaseEntry{
		Type:    "deployment",
		Version: "v1.0.0",
		Origin:  "ci",
		Date:    timestamppb.New(time.Now()),
	})
	req.Header().Set("X-API-Key", "my-secret-key")

	resp, err := releaseClient.AddRelease(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Day)
}

func TestInterceptor_ReadEndpoint_NoAuth_Passes(t *testing.T) {
	scheme := testScheme(t)
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Main", Main: true},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "key", Namespace: "default"},
		Data:       map[string][]byte{"api-key": []byte("my-secret-key")},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, secret).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()
	r := auth.NewPortalResolver(k8s, &fakePortalReader{views: mainPortalWithAPIKeyAuth()}, "default")
	defer r.Close()

	releaseClient := setupReleaseServer(t, k8s, r)

	resp, err := releaseClient.ListReleaseDays(context.Background(), connect.NewRequest(&releasev1.ListReleaseDaysRequest{}))
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
}

func TestInterceptor_NoResolver_NoProcedureProtection(t *testing.T) {
	scheme := testScheme(t)
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Main", Main: true},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()

	releaseClient := setupReleaseServer(t, k8s, nil)

	date := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	resp, err := releaseClient.AddRelease(context.Background(), connect.NewRequest(&releasev1.ReleaseEntry{
		Type:    "deployment",
		Version: "v1.0.0",
		Origin:  "ci",
		Date:    timestamppb.New(date),
	}))
	require.NoError(t, err)
	assert.Equal(t, "2026-03-25", resp.Msg.Day)
}

func TestInterceptor_APIKey_OnProtectedEndpoint(t *testing.T) {
	scheme := testScheme(t)
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Main", Main: true},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "key", Namespace: "default"},
		Data:       map[string][]byte{"api-key": []byte("my-secret-key")},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, secret).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()

	views := []domainportal.PortalView{
		{
			Name:         "main",
			Namespace:    "default",
			Main:         true,
			AuthExplicit: true,
			Auth: &domainportal.PortalAuthView{
				APIKey: &domainportal.PortalAPIKeyAuthView{
					Enabled:    true,
					HeaderName: "X-Custom-Auth",
					SecretName: "key",
					SecretKey:  "api-key",
				},
			},
		},
	}
	r := auth.NewPortalResolver(k8s, &fakePortalReader{views: views}, "default")
	defer r.Close()

	releaseClient := setupReleaseServer(t, k8s, r)

	req := connect.NewRequest(&releasev1.ReleaseEntry{
		Type:    "deployment",
		Version: "v1.0.0",
		Origin:  "ci",
		Date:    timestamppb.New(time.Now()),
	})
	req.Header().Set("X-Custom-Auth", "my-secret-key")

	resp, err := releaseClient.AddRelease(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Day)
}

func TestInterceptor_ListReleases_NoAuth_Passes(t *testing.T) {
	scheme := testScheme(t)
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Main", Main: true},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "key", Namespace: "default"},
		Data:       map[string][]byte{"api-key": []byte("my-secret-key")},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, secret).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()
	r := auth.NewPortalResolver(k8s, &fakePortalReader{views: mainPortalWithAPIKeyAuth()}, "default")
	defer r.Close()

	releaseClient := setupReleaseServer(t, k8s, r)

	resp, err := releaseClient.ListReleases(context.Background(), connect.NewRequest(&releasev1.ListReleasesRequest{
		Day: "2026-03-25",
	}))
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
}
