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

	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	portalv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// PortalService implements the PortalServiceHandler interface
type PortalService struct {
	sreportalv1connect.UnimplementedPortalServiceHandler
	reader domainportal.PortalReader
}

// NewPortalService creates a new PortalService
func NewPortalService(reader domainportal.PortalReader) *PortalService {
	return &PortalService{reader: reader}
}

// ListPortals returns all available portals
func (s *PortalService) ListPortals(
	ctx context.Context,
	req *connect.Request[portalv1.ListPortalsRequest],
) (*connect.Response[portalv1.ListPortalsResponse], error) {
	filters := domainportal.PortalFilters{
		Namespace: req.Msg.Namespace,
	}

	views, err := s.reader.List(ctx, filters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	portals := make([]*portalv1.Portal, 0, len(views))
	for _, v := range views {
		portals = append(portals, portalViewToProto(v))
	}

	return connect.NewResponse(&portalv1.ListPortalsResponse{
		Portals: portals,
	}), nil
}

func portalViewToProto(v domainportal.PortalView) *portalv1.Portal {
	subPath := v.SubPath
	if subPath == "" {
		subPath = v.Name
	}

	portal := &portalv1.Portal{
		Name:      v.Name,
		Title:     v.Title,
		Main:      v.Main,
		SubPath:   subPath,
		Namespace: v.Namespace,
		Ready:     v.Ready,
		IsRemote:  v.IsRemote,
		Url:       v.URL,
	}

	if v.RemoteSync != nil {
		portal.RemoteSync = &portalv1.RemoteSyncStatus{
			LastSyncTime:  v.RemoteSync.LastSyncTime,
			LastSyncError: v.RemoteSync.LastSyncError,
			RemoteTitle:   v.RemoteSync.RemoteTitle,
			FqdnCount:     int32(v.RemoteSync.FQDNCount),
		}
	}

	return portal
}
