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
	"sort"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	domainalertmanager "github.com/golgoth31/sreportal/internal/domain/alertmanagerreadmodel"
	alertmanagerv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// AlertmanagerService implements the AlertmanagerServiceHandler interface.
type AlertmanagerService struct {
	sreportalv1connect.UnimplementedAlertmanagerServiceHandler
	reader domainalertmanager.AlertmanagerReader
}

// NewAlertmanagerService creates a new AlertmanagerService.
func NewAlertmanagerService(reader domainalertmanager.AlertmanagerReader) *AlertmanagerService {
	return &AlertmanagerService{reader: reader}
}

// ListAlerts returns all active alerts from Alertmanager resources.
func (s *AlertmanagerService) ListAlerts(
	ctx context.Context,
	req *connect.Request[alertmanagerv1.ListAlertsRequest],
) (*connect.Response[alertmanagerv1.ListAlertsResponse], error) {
	filters := domainalertmanager.AlertmanagerFilters{
		Portal:    req.Msg.Portal,
		Namespace: req.Msg.Namespace,
	}

	views, err := s.reader.List(ctx, filters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resources := make([]*alertmanagerv1.AlertmanagerResource, 0, len(views))
	for _, v := range views {
		alerts := alertViewsToProto(v.Alerts, req.Msg.Search, req.Msg.State)

		resource := &alertmanagerv1.AlertmanagerResource{
			Name:      v.Name,
			Namespace: v.Namespace,
			PortalRef: v.PortalRef,
			LocalUrl:  v.LocalURL,
			RemoteUrl: v.RemoteURL,
			Alerts:    alerts,
			Ready:     v.Ready,
			Silences:  silenceViewsToProto(v.Silences),
		}

		if v.LastReconcileTime != nil {
			resource.LastReconcileTime = timestamppb.New(*v.LastReconcileTime)
		}

		resources = append(resources, resource)
	}

	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Namespace == resources[j].Namespace {
			return resources[i].Name < resources[j].Name
		}
		return resources[i].Namespace < resources[j].Namespace
	})

	return connect.NewResponse(&alertmanagerv1.ListAlertsResponse{
		Alertmanagers: resources,
	}), nil
}

func alertViewsToProto(views []domainalertmanager.AlertView, search, stateFilter string) []*alertmanagerv1.Alert {
	alerts := make([]*alertmanagerv1.Alert, 0, len(views))
	searchLower := strings.ToLower(search)

	for _, v := range views {
		if stateFilter != "" && v.State != stateFilter {
			continue
		}

		if search != "" && !matchesAlertViewSearch(v, searchLower) {
			continue
		}

		a := &alertmanagerv1.Alert{
			Fingerprint: v.Fingerprint,
			Labels:      v.Labels,
			Annotations: v.Annotations,
			State:       v.State,
			StartsAt:    timestamppb.New(v.StartsAt),
			UpdatedAt:   timestamppb.New(v.UpdatedAt),
			Receivers:   v.Receivers,
			SilencedBy:  v.SilencedBy,
		}
		if v.EndsAt != nil {
			a.EndsAt = timestamppb.New(*v.EndsAt)
		}
		alerts = append(alerts, a)
	}

	return alerts
}

func matchesAlertViewSearch(v domainalertmanager.AlertView, searchLower string) bool {
	for _, val := range v.Labels {
		if strings.Contains(strings.ToLower(val), searchLower) {
			return true
		}
	}
	for _, val := range v.Annotations {
		if strings.Contains(strings.ToLower(val), searchLower) {
			return true
		}
	}
	return false
}

func silenceViewsToProto(views []domainalertmanager.SilenceView) []*alertmanagerv1.Silence {
	if len(views) == 0 {
		return nil
	}
	silences := make([]*alertmanagerv1.Silence, 0, len(views))
	for _, v := range views {
		matchers := make([]*alertmanagerv1.Matcher, 0, len(v.Matchers))
		for _, m := range v.Matchers {
			matchers = append(matchers, &alertmanagerv1.Matcher{
				Name:    m.Name,
				Value:   m.Value,
				IsRegex: m.IsRegex,
			})
		}
		silences = append(silences, &alertmanagerv1.Silence{
			Id:        v.ID,
			Matchers:  matchers,
			StartsAt:  timestamppb.New(v.StartsAt),
			EndsAt:    timestamppb.New(v.EndsAt),
			Status:    v.Status,
			CreatedBy: v.CreatedBy,
			Comment:   v.Comment,
			UpdatedAt: timestamppb.New(v.UpdatedAt),
		})
	}
	return silences
}
