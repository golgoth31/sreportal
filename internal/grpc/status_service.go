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

	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domaincomponent "github.com/golgoth31/sreportal/internal/domain/component"
	domainincident "github.com/golgoth31/sreportal/internal/domain/incident"
	domainmaint "github.com/golgoth31/sreportal/internal/domain/maintenance"
	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	sreportalv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
	"github.com/golgoth31/sreportal/internal/statuspage"
)

// StatusService implements the StatusServiceHandler interface.
type StatusService struct {
	sreportalv1connect.UnimplementedStatusServiceHandler
	componentReader   domaincomponent.ComponentReader
	maintenanceReader domainmaint.MaintenanceReader
	incidentReader    domainincident.IncidentReader
	writer            *statuspage.Service
	portalReader      domainportal.PortalReader
}

// NewStatusService creates a new StatusService.
func NewStatusService(
	componentReader domaincomponent.ComponentReader,
	maintenanceReader domainmaint.MaintenanceReader,
	incidentReader domainincident.IncidentReader,
	writer *statuspage.Service,
	portalReader domainportal.PortalReader,
) *StatusService {
	return &StatusService{
		componentReader:   componentReader,
		maintenanceReader: maintenanceReader,
		incidentReader:    incidentReader,
		writer:            writer,
		portalReader:      portalReader,
	}
}

// ListComponents returns all platform components with their status.
func (s *StatusService) ListComponents(
	ctx context.Context,
	req *connect.Request[sreportalv1.ListComponentsRequest],
) (*connect.Response[sreportalv1.ListComponentsResponse], error) {
	if enabled, err := IsFeatureEnabled(ctx, s.portalReader, req.Msg.PortalRef, CheckStatusPage); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if !enabled {
		return connect.NewResponse(&sreportalv1.ListComponentsResponse{}), nil
	}

	opts := domaincomponent.ListOptions{
		PortalRef: req.Msg.PortalRef,
		Group:     req.Msg.Group,
	}

	views, err := s.componentReader.List(ctx, opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	components := make([]*sreportalv1.ComponentResource, 0, len(views))
	for _, v := range views {
		comp := &sreportalv1.ComponentResource{
			Name:             v.Name,
			DisplayName:      v.DisplayName,
			Description:      v.Description,
			Group:            v.Group,
			Link:             v.Link,
			PortalRef:        v.PortalRef,
			DeclaredStatus:   componentStatusToProto(v.DeclaredStatus),
			ComputedStatus:   componentStatusToProto(v.ComputedStatus),
			ActiveIncidents:  int32(v.ActiveIncidents),
			LastStatusChange: timestamppb.New(v.LastStatusChange),
		}
		components = append(components, comp)
	}

	sort.Slice(components, func(i, j int) bool {
		if components[i].Group == components[j].Group {
			return components[i].DisplayName < components[j].DisplayName
		}
		return components[i].Group < components[j].Group
	})

	return connect.NewResponse(&sreportalv1.ListComponentsResponse{
		Components: components,
	}), nil
}

// ListMaintenances returns maintenance windows.
func (s *StatusService) ListMaintenances(
	ctx context.Context,
	req *connect.Request[sreportalv1.ListMaintenancesRequest],
) (*connect.Response[sreportalv1.ListMaintenancesResponse], error) {
	if enabled, err := IsFeatureEnabled(ctx, s.portalReader, req.Msg.PortalRef, CheckStatusPage); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if !enabled {
		return connect.NewResponse(&sreportalv1.ListMaintenancesResponse{}), nil
	}

	opts := domainmaint.ListOptions{
		PortalRef: req.Msg.PortalRef,
		Phase:     maintenancePhaseFromProto(req.Msg.Phase),
	}

	views, err := s.maintenanceReader.List(ctx, opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	maintenances := make([]*sreportalv1.MaintenanceResource, 0, len(views))
	for _, v := range views {
		m := &sreportalv1.MaintenanceResource{
			Name:           v.Name,
			Title:          v.Title,
			Description:    v.Description,
			PortalRef:      v.PortalRef,
			Components:     v.Components,
			ScheduledStart: timestamppb.New(v.ScheduledStart),
			ScheduledEnd:   timestamppb.New(v.ScheduledEnd),
			AffectedStatus: v.AffectedStatus,
			Phase:          maintenancePhaseToProto(v.Phase),
		}
		maintenances = append(maintenances, m)
	}

	return connect.NewResponse(&sreportalv1.ListMaintenancesResponse{
		Maintenances: maintenances,
	}), nil
}

// ListIncidents returns declared incidents.
func (s *StatusService) ListIncidents(
	ctx context.Context,
	req *connect.Request[sreportalv1.ListIncidentsRequest],
) (*connect.Response[sreportalv1.ListIncidentsResponse], error) {
	if enabled, err := IsFeatureEnabled(ctx, s.portalReader, req.Msg.PortalRef, CheckStatusPage); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if !enabled {
		return connect.NewResponse(&sreportalv1.ListIncidentsResponse{}), nil
	}

	opts := domainincident.ListOptions{
		PortalRef: req.Msg.PortalRef,
		Phase:     incidentPhaseFromProto(req.Msg.Phase),
	}

	views, err := s.incidentReader.List(ctx, opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	incidents := make([]*sreportalv1.IncidentResource, 0, len(views))
	for _, v := range views {
		inc := &sreportalv1.IncidentResource{
			Name:            v.Name,
			Title:           v.Title,
			PortalRef:       v.PortalRef,
			Components:      v.Components,
			Severity:        incidentSeverityToProto(v.Severity),
			CurrentPhase:    incidentPhaseToProto(v.CurrentPhase),
			DurationMinutes: int32(v.DurationMinutes),
		}
		if !v.StartedAt.IsZero() {
			inc.StartedAt = timestamppb.New(v.StartedAt)
		}
		if !v.ResolvedAt.IsZero() {
			inc.ResolvedAt = timestamppb.New(v.ResolvedAt)
		}

		updates := make([]*sreportalv1.IncidentUpdate, 0, len(v.Updates))
		for _, u := range v.Updates {
			updates = append(updates, &sreportalv1.IncidentUpdate{
				Timestamp: timestamppb.New(u.Timestamp),
				Phase:     incidentPhaseToProto(u.Phase),
				Message:   u.Message,
			})
		}
		inc.Updates = updates

		incidents = append(incidents, inc)
	}

	return connect.NewResponse(&sreportalv1.ListIncidentsResponse{
		Incidents: incidents,
	}), nil
}

// --- Enum converters ---

func componentStatusToProto(s domaincomponent.ComponentStatus) sreportalv1.ComponentStatus {
	switch s {
	case domaincomponent.StatusOperational:
		return sreportalv1.ComponentStatus_COMPONENT_STATUS_OPERATIONAL
	case domaincomponent.StatusDegraded:
		return sreportalv1.ComponentStatus_COMPONENT_STATUS_DEGRADED
	case domaincomponent.StatusPartialOut:
		return sreportalv1.ComponentStatus_COMPONENT_STATUS_PARTIAL_OUTAGE
	case domaincomponent.StatusMajorOutage:
		return sreportalv1.ComponentStatus_COMPONENT_STATUS_MAJOR_OUTAGE
	case domaincomponent.StatusMaintenance:
		return sreportalv1.ComponentStatus_COMPONENT_STATUS_MAINTENANCE
	default:
		return sreportalv1.ComponentStatus_COMPONENT_STATUS_UNKNOWN
	}
}

func maintenancePhaseToProto(p domainmaint.MaintenancePhase) sreportalv1.MaintenancePhase {
	switch p {
	case domainmaint.PhaseUpcoming:
		return sreportalv1.MaintenancePhase_MAINTENANCE_PHASE_UPCOMING
	case domainmaint.PhaseInProgress:
		return sreportalv1.MaintenancePhase_MAINTENANCE_PHASE_IN_PROGRESS
	case domainmaint.PhaseCompleted:
		return sreportalv1.MaintenancePhase_MAINTENANCE_PHASE_COMPLETED
	default:
		return sreportalv1.MaintenancePhase_MAINTENANCE_PHASE_UNSPECIFIED
	}
}

func maintenancePhaseFromProto(p sreportalv1.MaintenancePhase) domainmaint.MaintenancePhase {
	switch p {
	case sreportalv1.MaintenancePhase_MAINTENANCE_PHASE_UPCOMING:
		return domainmaint.PhaseUpcoming
	case sreportalv1.MaintenancePhase_MAINTENANCE_PHASE_IN_PROGRESS:
		return domainmaint.PhaseInProgress
	case sreportalv1.MaintenancePhase_MAINTENANCE_PHASE_COMPLETED:
		return domainmaint.PhaseCompleted
	default:
		return ""
	}
}

func incidentPhaseToProto(p domainincident.IncidentPhase) sreportalv1.IncidentPhase {
	switch p {
	case domainincident.PhaseInvestigating:
		return sreportalv1.IncidentPhase_INCIDENT_PHASE_INVESTIGATING
	case domainincident.PhaseIdentified:
		return sreportalv1.IncidentPhase_INCIDENT_PHASE_IDENTIFIED
	case domainincident.PhaseMonitoring:
		return sreportalv1.IncidentPhase_INCIDENT_PHASE_MONITORING
	case domainincident.PhaseResolved:
		return sreportalv1.IncidentPhase_INCIDENT_PHASE_RESOLVED
	default:
		return sreportalv1.IncidentPhase_INCIDENT_PHASE_UNSPECIFIED
	}
}

func incidentPhaseFromProto(p sreportalv1.IncidentPhase) domainincident.IncidentPhase {
	switch p {
	case sreportalv1.IncidentPhase_INCIDENT_PHASE_INVESTIGATING:
		return domainincident.PhaseInvestigating
	case sreportalv1.IncidentPhase_INCIDENT_PHASE_IDENTIFIED:
		return domainincident.PhaseIdentified
	case sreportalv1.IncidentPhase_INCIDENT_PHASE_MONITORING:
		return domainincident.PhaseMonitoring
	case sreportalv1.IncidentPhase_INCIDENT_PHASE_RESOLVED:
		return domainincident.PhaseResolved
	default:
		return ""
	}
}

func incidentSeverityToProto(s domainincident.IncidentSeverity) sreportalv1.IncidentSeverity {
	switch s {
	case domainincident.SeverityCritical:
		return sreportalv1.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL
	case domainincident.SeverityMajor:
		return sreportalv1.IncidentSeverity_INCIDENT_SEVERITY_MAJOR
	case domainincident.SeverityMinor:
		return sreportalv1.IncidentSeverity_INCIDENT_SEVERITY_MINOR
	default:
		return sreportalv1.IncidentSeverity_INCIDENT_SEVERITY_UNSPECIFIED
	}
}

// --- Write RPCs (auth-protected) ---

// CreateComponent creates a new platform component CR.
func (s *StatusService) CreateComponent(
	ctx context.Context,
	req *connect.Request[sreportalv1.CreateComponentRequest],
) (*connect.Response[sreportalv1.CreateComponentResponse], error) {
	if enabled, err := IsFeatureEnabled(ctx, s.portalReader, req.Msg.PortalRef, CheckStatusPage); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if !enabled {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("statusPage feature is disabled for portal %q", req.Msg.PortalRef))
	}

	status, err := componentStatusFromProto(req.Msg.Status)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	in := statuspage.CreateComponentInput{
		DisplayName: req.Msg.DisplayName,
		Description: req.Msg.Description,
		Group:       req.Msg.Group,
		Link:        req.Msg.Link,
		PortalRef:   req.Msg.PortalRef,
		Status:      status,
	}

	name, err := s.writer.CreateComponent(ctx, in)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&sreportalv1.CreateComponentResponse{
		Name: name,
	}), nil
}

// UpdateComponent updates an existing platform component CR.
func (s *StatusService) UpdateComponent(
	ctx context.Context,
	req *connect.Request[sreportalv1.UpdateComponentRequest],
) (*connect.Response[sreportalv1.UpdateComponentResponse], error) {
	in := statuspage.UpdateComponentInput{
		Name: req.Msg.Name,
	}

	if req.Msg.DisplayName != nil {
		in.DisplayName = req.Msg.DisplayName
	}
	if req.Msg.Description != nil {
		in.Description = req.Msg.Description
	}
	if req.Msg.Group != nil {
		in.Group = req.Msg.Group
	}
	if req.Msg.Link != nil {
		in.Link = req.Msg.Link
	}
	if req.Msg.Status != nil {
		status, err := componentStatusFromProto(*req.Msg.Status)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		in.Status = &status
	}

	name, err := s.writer.UpdateComponent(ctx, in)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&sreportalv1.UpdateComponentResponse{
		Name: name,
	}), nil
}

// DeleteComponent deletes a platform component CR.
func (s *StatusService) DeleteComponent(
	ctx context.Context,
	req *connect.Request[sreportalv1.DeleteComponentRequest],
) (*connect.Response[sreportalv1.DeleteComponentResponse], error) {
	if err := s.writer.DeleteComponent(ctx, req.Msg.Name); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&sreportalv1.DeleteComponentResponse{}), nil
}

// CreateMaintenance creates a new maintenance window CR.
func (s *StatusService) CreateMaintenance(
	ctx context.Context,
	req *connect.Request[sreportalv1.CreateMaintenanceRequest],
) (*connect.Response[sreportalv1.CreateMaintenanceResponse], error) {
	if enabled, err := IsFeatureEnabled(ctx, s.portalReader, req.Msg.PortalRef, CheckStatusPage); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if !enabled {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("statusPage feature is disabled for portal %q", req.Msg.PortalRef))
	}

	var scheduledStart, scheduledEnd metav1.Time
	if req.Msg.ScheduledStart != nil {
		scheduledStart = metav1.NewTime(req.Msg.ScheduledStart.AsTime())
	}
	if req.Msg.ScheduledEnd != nil {
		scheduledEnd = metav1.NewTime(req.Msg.ScheduledEnd.AsTime())
	}

	affectedStatus, err := maintenanceAffectedStatusFromProto(req.Msg.AffectedStatus)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	in := statuspage.CreateMaintenanceInput{
		Title:          req.Msg.Title,
		Description:    req.Msg.Description,
		PortalRef:      req.Msg.PortalRef,
		Components:     req.Msg.Components,
		ScheduledStart: scheduledStart,
		ScheduledEnd:   scheduledEnd,
		AffectedStatus: affectedStatus,
	}

	name, err := s.writer.CreateMaintenance(ctx, in)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&sreportalv1.CreateMaintenanceResponse{
		Name: name,
	}), nil
}

// UpdateMaintenance updates an existing maintenance window CR.
func (s *StatusService) UpdateMaintenance(
	ctx context.Context,
	req *connect.Request[sreportalv1.UpdateMaintenanceRequest],
) (*connect.Response[sreportalv1.UpdateMaintenanceResponse], error) {
	in := statuspage.UpdateMaintenanceInput{
		Name:       req.Msg.Name,
		Components: req.Msg.Components,
	}

	if req.Msg.Title != nil {
		in.Title = req.Msg.Title
	}
	if req.Msg.Description != nil {
		in.Description = req.Msg.Description
	}
	if req.Msg.ScheduledStart != nil {
		ts := metav1.NewTime(req.Msg.ScheduledStart.AsTime())
		in.ScheduledStart = &ts
	}
	if req.Msg.ScheduledEnd != nil {
		ts := metav1.NewTime(req.Msg.ScheduledEnd.AsTime())
		in.ScheduledEnd = &ts
	}
	if req.Msg.AffectedStatus != nil {
		status, err := maintenanceAffectedStatusFromProto(*req.Msg.AffectedStatus)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		in.AffectedStatus = &status
	}

	name, err := s.writer.UpdateMaintenance(ctx, in)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&sreportalv1.UpdateMaintenanceResponse{
		Name: name,
	}), nil
}

// DeleteMaintenance deletes a maintenance window CR.
func (s *StatusService) DeleteMaintenance(
	ctx context.Context,
	req *connect.Request[sreportalv1.DeleteMaintenanceRequest],
) (*connect.Response[sreportalv1.DeleteMaintenanceResponse], error) {
	if err := s.writer.DeleteMaintenance(ctx, req.Msg.Name); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&sreportalv1.DeleteMaintenanceResponse{}), nil
}

// CreateIncident creates a new incident CR.
func (s *StatusService) CreateIncident(
	ctx context.Context,
	req *connect.Request[sreportalv1.CreateIncidentRequest],
) (*connect.Response[sreportalv1.CreateIncidentResponse], error) {
	if enabled, err := IsFeatureEnabled(ctx, s.portalReader, req.Msg.PortalRef, CheckStatusPage); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if !enabled {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("statusPage feature is disabled for portal %q", req.Msg.PortalRef))
	}

	severity, err := incidentSeverityFromProto(req.Msg.Severity)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if req.Msg.InitialUpdate == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("initial_update is required"))
	}

	update, err := protoToIncidentUpdate(req.Msg.InitialUpdate, "initial_update")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	in := statuspage.CreateIncidentInput{
		Title:         req.Msg.Title,
		PortalRef:     req.Msg.PortalRef,
		Components:    req.Msg.Components,
		Severity:      severity,
		InitialUpdate: update,
	}

	name, err := s.writer.CreateIncident(ctx, in)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&sreportalv1.CreateIncidentResponse{
		Name: name,
	}), nil
}

// UpdateIncident updates an existing incident CR, appending a timeline entry.
func (s *StatusService) UpdateIncident(
	ctx context.Context,
	req *connect.Request[sreportalv1.UpdateIncidentRequest],
) (*connect.Response[sreportalv1.UpdateIncidentResponse], error) {
	if req.Msg.Update == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("update is required"))
	}

	update, err := protoToIncidentUpdate(req.Msg.Update, "update")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	in := statuspage.UpdateIncidentInput{
		Name:       req.Msg.Name,
		Components: req.Msg.Components,
		Update:     update,
	}

	if req.Msg.Title != nil {
		in.Title = req.Msg.Title
	}
	if req.Msg.Severity != nil {
		sev, err := incidentSeverityFromProto(*req.Msg.Severity)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		in.Severity = &sev
	}

	name, err := s.writer.UpdateIncident(ctx, in)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&sreportalv1.UpdateIncidentResponse{
		Name: name,
	}), nil
}

func protoToIncidentUpdate(u *sreportalv1.IncidentUpdate, field string) (sreportalv1alpha1.IncidentUpdate, error) {
	phase := incidentPhaseFromProto(u.Phase)
	if phase == "" {
		return sreportalv1alpha1.IncidentUpdate{}, fmt.Errorf("%s.phase: unsupported value %q", field, u.Phase.String())
	}
	var ts metav1.Time
	if u.Timestamp != nil {
		ts = metav1.NewTime(u.Timestamp.AsTime())
	}
	return sreportalv1alpha1.IncidentUpdate{
		Timestamp: ts,
		Phase:     sreportalv1alpha1.IncidentPhase(phase),
		Message:   u.Message,
	}, nil
}

// DeleteIncident deletes an incident CR.
func (s *StatusService) DeleteIncident(
	ctx context.Context,
	req *connect.Request[sreportalv1.DeleteIncidentRequest],
) (*connect.Response[sreportalv1.DeleteIncidentResponse], error) {
	if err := s.writer.DeleteIncident(ctx, req.Msg.Name); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&sreportalv1.DeleteIncidentResponse{}), nil
}

// --- Proto → CRD enum converters ---

func componentStatusFromProto(s sreportalv1.ComponentStatus) (sreportalv1alpha1.ComponentStatusValue, error) {
	switch s {
	case sreportalv1.ComponentStatus_COMPONENT_STATUS_OPERATIONAL:
		return sreportalv1alpha1.ComponentStatusOperational, nil
	case sreportalv1.ComponentStatus_COMPONENT_STATUS_DEGRADED:
		return sreportalv1alpha1.ComponentStatusDegraded, nil
	case sreportalv1.ComponentStatus_COMPONENT_STATUS_PARTIAL_OUTAGE:
		return sreportalv1alpha1.ComponentStatusPartialOut, nil
	case sreportalv1.ComponentStatus_COMPONENT_STATUS_MAJOR_OUTAGE:
		return sreportalv1alpha1.ComponentStatusMajorOutage, nil
	case sreportalv1.ComponentStatus_COMPONENT_STATUS_UNKNOWN:
		return sreportalv1alpha1.ComponentStatusUnknown, nil
	default:
		return "", fmt.Errorf("status: unsupported value %q", s.String())
	}
}

func incidentSeverityFromProto(s sreportalv1.IncidentSeverity) (sreportalv1alpha1.IncidentSeverity, error) {
	switch s {
	case sreportalv1.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL:
		return sreportalv1alpha1.IncidentSeverityCritical, nil
	case sreportalv1.IncidentSeverity_INCIDENT_SEVERITY_MAJOR:
		return sreportalv1alpha1.IncidentSeverityMajor, nil
	case sreportalv1.IncidentSeverity_INCIDENT_SEVERITY_MINOR:
		return sreportalv1alpha1.IncidentSeverityMinor, nil
	default:
		return "", fmt.Errorf("severity: unsupported value %q", s.String())
	}
}

func maintenanceAffectedStatusFromProto(s string) (sreportalv1alpha1.MaintenanceAffectedStatus, error) {
	v := sreportalv1alpha1.MaintenanceAffectedStatus(s)
	switch v {
	case sreportalv1alpha1.MaintenanceAffectedMaintenance,
		sreportalv1alpha1.MaintenanceAffectedDegraded,
		sreportalv1alpha1.MaintenanceAffectedPartialOut,
		sreportalv1alpha1.MaintenanceAffectedMajorOutage:
		return v, nil
	default:
		return "", fmt.Errorf("affected_status: unsupported value %q", s)
	}
}

// toConnectError maps service-layer errors to Connect error codes.
func toConnectError(err error) *connect.Error {
	switch {
	case errors.Is(err, statuspage.ErrNameRequired),
		errors.Is(err, statuspage.ErrPortalRefRequired),
		errors.Is(err, statuspage.ErrGroupRequired),
		errors.Is(err, statuspage.ErrTitleRequired),
		errors.Is(err, statuspage.ErrSeverityRequired):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, statuspage.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, statuspage.ErrAlreadyExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
