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
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	alertmanagerv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// AlertmanagerService implements the AlertmanagerServiceHandler interface.
type AlertmanagerService struct {
	sreportalv1connect.UnimplementedAlertmanagerServiceHandler
	client client.Client
}

// NewAlertmanagerService creates a new AlertmanagerService.
func NewAlertmanagerService(c client.Client) *AlertmanagerService {
	return &AlertmanagerService{client: c}
}

// ListAlerts returns all active alerts from Alertmanager resources.
func (s *AlertmanagerService) ListAlerts(
	ctx context.Context,
	req *connect.Request[alertmanagerv1.ListAlertsRequest],
) (*connect.Response[alertmanagerv1.ListAlertsResponse], error) {
	var amList sreportalv1alpha1.AlertmanagerList
	listOpts := []client.ListOption{}

	if req.Msg.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(req.Msg.Namespace))
	}

	if err := s.client.List(ctx, &amList, listOpts...); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resources := make([]*alertmanagerv1.AlertmanagerResource, 0, len(amList.Items))
	for _, am := range amList.Items {
		if req.Msg.Portal != "" && am.Spec.PortalRef != req.Msg.Portal {
			continue
		}

		alerts := toProtoAlerts(am.Status.ActiveAlerts, req.Msg.Search, req.Msg.State)

		ready := false
		for _, c := range am.Status.Conditions {
			if c.Type == "Ready" && c.Status == "True" {
				ready = true
				break
			}
		}

		resource := &alertmanagerv1.AlertmanagerResource{
			Name:      am.Name,
			Namespace: am.Namespace,
			PortalRef: am.Spec.PortalRef,
			LocalUrl:  am.Spec.URL.Local,
			RemoteUrl: am.Spec.URL.Remote,
			Alerts:    alerts,
			Ready:     ready,
		}

		if am.Status.LastReconcileTime != nil {
			resource.LastReconcileTime = timestamppb.New(am.Status.LastReconcileTime.Time)
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

func toProtoAlerts(statuses []sreportalv1alpha1.AlertStatus, search, stateFilter string) []*alertmanagerv1.Alert {
	alerts := make([]*alertmanagerv1.Alert, 0, len(statuses))
	searchLower := strings.ToLower(search)

	for _, s := range statuses {
		if stateFilter != "" && s.State != stateFilter {
			continue
		}

		if search != "" && !matchesSearch(s, searchLower) {
			continue
		}

		a := &alertmanagerv1.Alert{
			Fingerprint: s.Fingerprint,
			Labels:      s.Labels,
			Annotations: s.Annotations,
			State:       s.State,
			StartsAt:    timestamppb.New(s.StartsAt.Time),
			UpdatedAt:   timestamppb.New(s.UpdatedAt.Time),
		}
		if s.EndsAt != nil {
			a.EndsAt = timestamppb.New(s.EndsAt.Time)
		}
		alerts = append(alerts, a)
	}

	return alerts
}

func matchesSearch(s sreportalv1alpha1.AlertStatus, searchLower string) bool {
	for _, v := range s.Labels {
		if strings.Contains(strings.ToLower(v), searchLower) {
			return true
		}
	}
	for _, v := range s.Annotations {
		if strings.Contains(strings.ToLower(v), searchLower) {
			return true
		}
	}
	return false
}
