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
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	portalv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// PortalService implements the PortalServiceHandler interface
type PortalService struct {
	sreportalv1connect.UnimplementedPortalServiceHandler
	client client.Client
}

// NewPortalService creates a new PortalService
func NewPortalService(c client.Client) *PortalService {
	return &PortalService{client: c}
}

// ListPortals returns all available portals
func (s *PortalService) ListPortals(
	ctx context.Context,
	req *connect.Request[portalv1.ListPortalsRequest],
) (*connect.Response[portalv1.ListPortalsResponse], error) {
	var portalList sreportalv1alpha1.PortalList
	listOpts := []client.ListOption{}

	if req.Msg.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(req.Msg.Namespace))
	}

	if err := s.client.List(ctx, &portalList, listOpts...); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	portals := make([]*portalv1.Portal, 0, len(portalList.Items))
	for _, p := range portalList.Items {
		subPath := p.Spec.SubPath
		if subPath == "" {
			subPath = p.Name
		}

		portals = append(portals, &portalv1.Portal{
			Name:      p.Name,
			Title:     p.Spec.Title,
			Main:      p.Spec.Main,
			SubPath:   subPath,
			Namespace: p.Namespace,
			Ready:     p.Status.Ready,
		})
	}

	return connect.NewResponse(&portalv1.ListPortalsResponse{
		Portals: portals,
	}), nil
}
