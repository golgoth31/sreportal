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

package grpc

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/golgoth31/sreportal/internal/config"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	releasev1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
	"github.com/golgoth31/sreportal/internal/log"
	releaseservice "github.com/golgoth31/sreportal/internal/release"
)

// ReleaseService implements the ReleaseServiceHandler interface.
type ReleaseService struct {
	sreportalv1connect.UnimplementedReleaseServiceHandler
	service      *releaseservice.Service
	ttl          time.Duration
	allowedTypes []config.ReleaseTypeConfig
}

// NewReleaseService creates a new ReleaseService.
func NewReleaseService(svc *releaseservice.Service, ttl time.Duration, allowedTypes []config.ReleaseTypeConfig) *ReleaseService {
	return &ReleaseService{service: svc, ttl: ttl, allowedTypes: allowedTypes}
}

// AddRelease appends a release entry to the day's Release CR.
func (s *ReleaseService) AddRelease(
	ctx context.Context,
	req *connect.Request[releasev1.ReleaseEntry],
) (*connect.Response[releasev1.AddReleaseResponse], error) {
	e := req.Msg
	date := e.Date.AsTime()
	entry, err := domainrelease.NewEntry(e.Type, e.Version, e.Origin, date)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	allowedNames := make([]string, len(s.allowedTypes))
	for i, t := range s.allowedTypes {
		allowedNames[i] = t.Name
	}
	if err := domainrelease.ValidateType(e.Type, allowedNames); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	entry.Author = e.Author
	entry.Message = e.Message
	entry.Link = e.Link

	day, count, created, err := s.service.AddEntry(ctx, entry)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if created {
		logger := log.Default().WithName("release-service")
		logger.Info("release CR created", "day", day, "type", e.Type, "version", e.Version, "origin", e.Origin)
	}

	return connect.NewResponse(&releasev1.AddReleaseResponse{
		Day:        day,
		EntryCount: int32(count),
	}), nil
}

// ListReleases returns release entries paginated by day.
func (s *ReleaseService) ListReleases(
	ctx context.Context,
	req *connect.Request[releasev1.ListReleasesRequest],
) (*connect.Response[releasev1.ListReleasesResponse], error) {
	day := req.Msg.Day

	// If no day specified, use the latest available day
	if day == "" {
		days, err := s.service.ListDays(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if len(days) == 0 {
			return connect.NewResponse(&releasev1.ListReleasesResponse{}), nil
		}
		day = days[len(days)-1]
	}

	entries, err := s.service.ListEntries(ctx, day)
	if err != nil {
		if errors.Is(err, domainrelease.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Build proto entries
	protoEntries := make([]*releasev1.ReleaseEntry, 0, len(entries))
	for _, e := range entries {
		protoEntries = append(protoEntries, &releasev1.ReleaseEntry{
			Type:    e.Type,
			Version: e.Version,
			Origin:  e.Origin,
			Date:    timestamppb.New(e.Date.Time),
			Author:  e.Author,
			Message: e.Message,
			Link:    e.Link,
		})
	}

	// Determine previous/next day navigation
	days, err := s.service.ListDays(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var prevDay, nextDay string
	for i, d := range days {
		if d == day {
			if i > 0 {
				prevDay = days[i-1]
			}
			if i < len(days)-1 {
				nextDay = days[i+1]
			}
			break
		}
	}

	return connect.NewResponse(&releasev1.ListReleasesResponse{
		Day:         day,
		Entries:     protoEntries,
		PreviousDay: prevDay,
		NextDay:     nextDay,
	}), nil
}

// ListReleaseDays returns all days that have releases and the TTL window.
func (s *ReleaseService) ListReleaseDays(
	ctx context.Context,
	_ *connect.Request[releasev1.ListReleaseDaysRequest],
) (*connect.Response[releasev1.ListReleaseDaysResponse], error) {
	days, err := s.service.ListDays(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ttlDays := int32(s.ttl.Hours() / 24)

	protoTypes := make([]*releasev1.ReleaseTypeConfig, len(s.allowedTypes))
	for i, t := range s.allowedTypes {
		protoTypes[i] = &releasev1.ReleaseTypeConfig{Name: t.Name, Color: t.Color}
	}

	return connect.NewResponse(&releasev1.ListReleaseDaysResponse{
		Days:    days,
		TtlDays: ttlDays,
		Types:   protoTypes,
	}), nil
}
