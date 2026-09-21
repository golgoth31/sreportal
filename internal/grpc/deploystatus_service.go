package grpc

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	domaindeploystatus "github.com/golgoth31/sreportal/internal/domain/deploystatus"
	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	genv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	"github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1/sreportalv1connect"
)

// DeployStatusService implements DeployStatusServiceHandler.
type DeployStatusService struct {
	sreportalv1connect.UnimplementedDeployStatusServiceHandler
	reader       domaindeploystatus.Reader
	portalReader domainportal.PortalReader
}

// NewDeployStatusService creates a DeployStatusService.
func NewDeployStatusService(reader domaindeploystatus.Reader, portalReader domainportal.PortalReader) *DeployStatusService {
	return &DeployStatusService{reader: reader, portalReader: portalReader}
}

func (s *DeployStatusService) ListDeployStatus(
	ctx context.Context,
	req *connect.Request[genv1.ListDeployStatusRequest],
) (*connect.Response[genv1.ListDeployStatusResponse], error) {
	portal := req.Msg.Portal
	if portal == "" {
		portal = "main"
	}

	if enabled, err := IsFeatureEnabled(ctx, s.portalReader, portal, CheckDeployStatus); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if !enabled {
		return connect.NewResponse(&genv1.ListDeployStatusResponse{}), nil
	}

	entries := s.reader.List(portal)

	out := make([]*genv1.DeployStatusEntry, 0, len(entries))
	for _, e := range entries {
		if !matchesFilters(e, req.Msg.Search, req.Msg.StateFilter) {
			continue
		}
		out = append(out, toProtoDeployStatusEntry(e))
	}

	return connect.NewResponse(&genv1.ListDeployStatusResponse{Entries: out}), nil
}

// matchesFilters returns true when the entry passes both optional filters.
func matchesFilters(e domaindeploystatus.Entry, search, stateFilter string) bool {
	if search != "" {
		if !strings.Contains(e.Image, search) && !strings.Contains(e.SourceRepo, search) {
			return false
		}
	}
	if stateFilter != "" && e.State != stateFilter {
		return false
	}
	return true
}

func toProtoDeployStatusEntry(e domaindeploystatus.Entry) *genv1.DeployStatusEntry {
	commits := make([]*genv1.DeployStatusCommit, 0, len(e.PendingCommits))
	for _, c := range e.PendingCommits {
		commits = append(commits, &genv1.DeployStatusCommit{
			Sha:     c.Sha,
			Message: c.Message,
			Author:  c.Author,
			Date:    timestamppb.New(c.Date),
			Url:     c.URL,
		})
	}

	entry := &genv1.DeployStatusEntry{
		Key: e.Key,
		Workload: &genv1.DeployWorkloadRef{
			Kind:      e.Workload.Kind,
			Namespace: e.Workload.Namespace,
			Name:      e.Workload.Name,
			Container: e.Workload.Container,
		},
		Image:            e.Image,
		SourceRepo:       e.SourceRepo,
		DeployedRef:      e.DeployedRef,
		DefaultBranch:    e.DefaultBranch,
		AheadBy:          int32(e.AheadBy),
		PendingCommits:   commits,
		PendingTruncated: e.PendingTruncated,
		DeployRunUrl:     e.DeployRunURL,
		State:            e.State,
		Error:            e.Error,
	}
	if !e.DeployedAt.IsZero() {
		entry.DeployedAt = timestamppb.New(e.DeployedAt)
	}
	if !e.LastCheckedAt.IsZero() {
		entry.LastCheckedAt = timestamppb.New(e.LastCheckedAt)
	}
	return entry
}
