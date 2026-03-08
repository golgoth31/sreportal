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

	"connectrpc.com/connect"

	portalv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
	"github.com/golgoth31/sreportal/internal/version"
)

// VersionService implements the VersionServiceHandler interface
type VersionService struct {
	sreportalv1connect.UnimplementedVersionServiceHandler
}

// NewVersionService creates a new VersionService
func NewVersionService() *VersionService {
	return &VersionService{}
}

// GetVersion returns the current build version information
func (s *VersionService) GetVersion(
	_ context.Context,
	_ *connect.Request[portalv1.GetVersionRequest],
) (*connect.Response[portalv1.GetVersionResponse], error) {
	return connect.NewResponse(&portalv1.GetVersionResponse{
		Version: version.Version(),
		Commit:  version.Commit(),
		Date:    version.Date(),
	}), nil
}
