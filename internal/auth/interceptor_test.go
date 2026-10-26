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

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/auth"
	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	releasev1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
	readstorerelease "github.com/golgoth31/sreportal/internal/readstore/release"
	releaseservice "github.com/golgoth31/sreportal/internal/release"
)

func setupReleaseServer(t *testing.T, chain *auth.Chain) sreportalv1connect.ReleaseServiceClient {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = sreportalv1alpha1.AddToScheme(scheme)
	portal := &sreportalv1alpha1.Portal{
		Name: "main", Namespace: "default",
		Spec: sreportalv1alpha1.PortalSpec{Title: "Main"},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()
	svc := releaseservice.NewService(k8sClient, "default", "main")
	reader := readstorerelease.NewReleaseStore()
	grpcSvc := svcgrpc.NewReleaseService(reader, svc, 30*24*time.Hour, nil, nil)

	var interceptors []connect.ServerInterceptor
	if chain != nil {
		interceptors = []connect.ServerInterceptor{auth.AuthInterceptor(chain)}
	}

	mux := http.NewServeMux()
	srv := connect.NewServer(interceptors...)
	sreportalv1connect.RegisterReleaseServiceHandler(srv, grpcSvc)
	connecthttp.Mount(mux, srv, connecthttp.WithReadMaxBytes(0))

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return sreportalv1connect.NewReleaseServiceClient(connect.NewClient(
		connecthttp.NewTransport(server.Client(), server.URL, connecthttp.WithReadMaxBytes(0)),
	))
}

func clientCtxWithHeader(key, value string) context.Context {
	ctx, info := connect.NewClientContext(context.Background())
	info.RequestHeader().Set(key, value)
	return ctx
}

func TestInterceptor_ProtectedEndpoint_NoAuth_Rejected(t *testing.T) {
	chain := auth.NewChain(
		auth.NewAPIKeyAuthenticator("", "my-secret-key"),
	)
	client := setupReleaseServer(t, chain)

	_, err := client.AddRelease(context.Background(), &releasev1.ReleaseEntry{
		Type:    tKindDeployment,
		Version: tVerV1,
		Origin:  "ci",
		Date:    timestamppb.New(time.Now()),
	})

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestInterceptor_ProtectedEndpoint_ValidAuth_Passes(t *testing.T) {
	chain := auth.NewChain(
		auth.NewAPIKeyAuthenticator("", "my-secret-key"),
	)
	client := setupReleaseServer(t, chain)

	resp, err := client.AddRelease(clientCtxWithHeader("X-API-Key", "my-secret-key"), &releasev1.ReleaseEntry{
		Type:    tKindDeployment,
		Version: tVerV1,
		Origin:  "ci",
		Date:    timestamppb.New(time.Now()),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Day)
}

func TestInterceptor_ReadEndpoint_NoAuth_Passes(t *testing.T) {
	chain := auth.NewChain(
		auth.NewAPIKeyAuthenticator("", "my-secret-key"),
	)
	client := setupReleaseServer(t, chain)

	resp, err := client.ListReleaseDays(context.Background(), &releasev1.ListReleaseDaysRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestInterceptor_NoChain_NoProcedureProtection(t *testing.T) {
	client := setupReleaseServer(t, nil)

	date := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	resp, err := client.AddRelease(context.Background(), &releasev1.ReleaseEntry{
		Type:    tKindDeployment,
		Version: tVerV1,
		Origin:  "ci",
		Date:    timestamppb.New(date),
	})
	require.NoError(t, err)
	assert.Equal(t, "2026-03-25", resp.Day)
}

func TestInterceptor_APIKey_OnProtectedEndpoint(t *testing.T) {
	chain := auth.NewChain(
		auth.NewAPIKeyAuthenticator("X-Custom-Auth", "my-secret-key"),
	)
	client := setupReleaseServer(t, chain)

	resp, err := client.AddRelease(clientCtxWithHeader("X-Custom-Auth", "my-secret-key"), &releasev1.ReleaseEntry{
		Type:    tKindDeployment,
		Version: tVerV1,
		Origin:  "ci",
		Date:    timestamppb.New(time.Now()),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Day)
}

func TestInterceptor_ListReleases_NoAuth_Passes(t *testing.T) {
	chain := auth.NewChain(
		auth.NewAPIKeyAuthenticator("", "my-secret-key"),
	)
	client := setupReleaseServer(t, chain)

	resp, err := client.ListReleases(context.Background(), &releasev1.ListReleasesRequest{
		Day: "2026-03-25",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}
