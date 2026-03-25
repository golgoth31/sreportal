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

package grpc_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	releasev1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	releasereadstore "github.com/golgoth31/sreportal/internal/readstore/release"
	releaseservice "github.com/golgoth31/sreportal/internal/release"
)

func releaseScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = sreportalv1alpha1.AddToScheme(s)
	return s
}

func TestAddRelease_CreatesEntry(t *testing.T) {
	ctx := context.Background()
	scheme := releaseScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()
	svc := releaseservice.NewService(k8sClient, "default")
	store := releasereadstore.NewReleaseStore()
	grpcSvc := svcgrpc.NewReleaseService(store, svc, 30*24*time.Hour, nil)

	date := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	resp, err := grpcSvc.AddRelease(ctx, connect.NewRequest(&releasev1.ReleaseEntry{
		Type:    "deployment",
		Version: "v1.0.0",
		Origin:  "ci/cd",
		Date:    timestamppb.New(date),
	}))

	require.NoError(t, err)
	assert.Equal(t, "2026-03-21", resp.Msg.Day)
	assert.Equal(t, int32(1), resp.Msg.EntryCount)
}

func TestAddRelease_EmptyType(t *testing.T) {
	ctx := context.Background()
	scheme := releaseScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	svc := releaseservice.NewService(k8sClient, "default")
	store := releasereadstore.NewReleaseStore()
	grpcSvc := svcgrpc.NewReleaseService(store, svc, 30*24*time.Hour, nil)

	_, err := grpcSvc.AddRelease(ctx, connect.NewRequest(&releasev1.ReleaseEntry{
		Type:    "",
		Version: "v1.0.0",
		Origin:  "ci/cd",
		Date:    timestamppb.New(time.Now()),
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAddRelease_WithOptionalFields(t *testing.T) {
	ctx := context.Background()
	scheme := releaseScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()
	svc := releaseservice.NewService(k8sClient, "default")
	store := releasereadstore.NewReleaseStore()
	grpcSvc := svcgrpc.NewReleaseService(store, svc, 30*24*time.Hour, nil)

	date := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	resp, err := grpcSvc.AddRelease(ctx, connect.NewRequest(&releasev1.ReleaseEntry{
		Type:    "deployment",
		Version: "v1.0.0",
		Origin:  "ci/cd",
		Date:    timestamppb.New(date),
		Author:  "alice",
		Message: "fix login bug",
		Link:    "https://github.com/example/repo/pull/42",
	}))

	require.NoError(t, err)
	assert.Equal(t, "2026-03-21", resp.Msg.Day)
	assert.Equal(t, int32(1), resp.Msg.EntryCount)
}

func TestListReleases_ReturnsEntries(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()
	_ = store.Replace(ctx, "2026-03-21", []domainrelease.EntryView{
		{Type: "deployment", Version: "v1.0.0", Origin: "ci/cd", Date: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)},
		{Type: "rollback", Version: "v0.9.0", Origin: "manual", Date: time.Date(2026, 3, 21, 14, 0, 0, 0, time.UTC)},
	})
	grpcSvc := svcgrpc.NewReleaseService(store, nil, 30*24*time.Hour, nil)

	resp, err := grpcSvc.ListReleases(ctx, connect.NewRequest(&releasev1.ListReleasesRequest{
		Day: "2026-03-21",
	}))

	require.NoError(t, err)
	assert.Equal(t, "2026-03-21", resp.Msg.Day)
	assert.Len(t, resp.Msg.Entries, 2)
	assert.Equal(t, "deployment", resp.Msg.Entries[0].Type)
	assert.Equal(t, "rollback", resp.Msg.Entries[1].Type)
}

func TestListReleases_EmptyDayReturnsLatest(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()
	_ = store.Replace(ctx, "2026-03-20", []domainrelease.EntryView{
		{Type: "deployment", Version: "v1.0.0", Origin: "ci", Date: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)},
	})
	_ = store.Replace(ctx, "2026-03-21", []domainrelease.EntryView{
		{Type: "hotfix", Version: "v1.0.1", Origin: "manual", Date: time.Date(2026, 3, 21, 8, 0, 0, 0, time.UTC)},
	})
	grpcSvc := svcgrpc.NewReleaseService(store, nil, 30*24*time.Hour, nil)

	resp, err := grpcSvc.ListReleases(ctx, connect.NewRequest(&releasev1.ListReleasesRequest{}))

	require.NoError(t, err)
	assert.Equal(t, "2026-03-21", resp.Msg.Day)
	assert.Len(t, resp.Msg.Entries, 1)
	assert.Equal(t, "2026-03-20", resp.Msg.PreviousDay)
	assert.Empty(t, resp.Msg.NextDay)
}

func TestListReleases_DayNavigation(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()
	_ = store.Replace(ctx, "2026-03-19", []domainrelease.EntryView{
		{Type: "deploy", Version: "v1", Origin: "ci", Date: time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)},
	})
	_ = store.Replace(ctx, "2026-03-20", []domainrelease.EntryView{
		{Type: "deploy", Version: "v2", Origin: "ci", Date: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)},
	})
	_ = store.Replace(ctx, "2026-03-21", []domainrelease.EntryView{
		{Type: "deploy", Version: "v3", Origin: "ci", Date: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)},
	})
	grpcSvc := svcgrpc.NewReleaseService(store, nil, 30*24*time.Hour, nil)

	resp, err := grpcSvc.ListReleases(ctx, connect.NewRequest(&releasev1.ListReleasesRequest{
		Day: "2026-03-20",
	}))

	require.NoError(t, err)
	assert.Equal(t, "2026-03-20", resp.Msg.Day)
	assert.Equal(t, "2026-03-19", resp.Msg.PreviousDay)
	assert.Equal(t, "2026-03-21", resp.Msg.NextDay)
}

func TestAddRelease_WithoutVersion(t *testing.T) {
	ctx := context.Background()
	scheme := releaseScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()
	svc := releaseservice.NewService(k8sClient, "default")
	store := releasereadstore.NewReleaseStore()
	grpcSvc := svcgrpc.NewReleaseService(store, svc, 30*24*time.Hour, nil)

	date := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	resp, err := grpcSvc.AddRelease(ctx, connect.NewRequest(&releasev1.ReleaseEntry{
		Type:   "deployment",
		Origin: "ci/cd",
		Date:   timestamppb.New(date),
	}))

	require.NoError(t, err)
	assert.Equal(t, "2026-03-21", resp.Msg.Day)
	assert.Equal(t, int32(1), resp.Msg.EntryCount)
}

func TestAddRelease_TypeNotAllowed(t *testing.T) {
	ctx := context.Background()
	scheme := releaseScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	svc := releaseservice.NewService(k8sClient, "default")
	store := releasereadstore.NewReleaseStore()
	grpcSvc := svcgrpc.NewReleaseService(store, svc, 30*24*time.Hour, []config.ReleaseTypeConfig{
		{Name: "deployment"}, {Name: "rollback"},
	})

	_, err := grpcSvc.AddRelease(ctx, connect.NewRequest(&releasev1.ReleaseEntry{
		Type:    "hotfix",
		Version: "v1.0.0",
		Origin:  "ci/cd",
		Date:    timestamppb.New(time.Now()),
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "not allowed")
}

func TestAddRelease_TypeAllowed(t *testing.T) {
	ctx := context.Background()
	scheme := releaseScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()
	svc := releaseservice.NewService(k8sClient, "default")
	store := releasereadstore.NewReleaseStore()
	grpcSvc := svcgrpc.NewReleaseService(store, svc, 30*24*time.Hour, []config.ReleaseTypeConfig{
		{Name: "deployment"}, {Name: "rollback"},
	})

	date := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	resp, err := grpcSvc.AddRelease(ctx, connect.NewRequest(&releasev1.ReleaseEntry{
		Type:    "deployment",
		Version: "v1.0.0",
		Origin:  "ci/cd",
		Date:    timestamppb.New(date),
	}))

	require.NoError(t, err)
	assert.Equal(t, "2026-03-21", resp.Msg.Day)
}

func TestListReleaseDays_ReturnsDaysAndTTL(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()
	_ = store.Replace(ctx, "2026-03-19", []domainrelease.EntryView{
		{Type: "deploy", Origin: "ci", Date: time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)},
	})
	_ = store.Replace(ctx, "2026-03-21", []domainrelease.EntryView{
		{Type: "deploy", Origin: "ci", Date: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)},
	})
	grpcSvc := svcgrpc.NewReleaseService(store, nil, 30*24*time.Hour, nil)

	resp, err := grpcSvc.ListReleaseDays(ctx, connect.NewRequest(&releasev1.ListReleaseDaysRequest{}))

	require.NoError(t, err)
	assert.Equal(t, []string{"2026-03-19", "2026-03-21"}, resp.Msg.Days)
	assert.Equal(t, int32(30), resp.Msg.TtlDays)
}
