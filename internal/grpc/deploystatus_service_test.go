package grpc_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaindeploystatus "github.com/golgoth31/sreportal/internal/domain/deploystatus"
	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	genv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
	readstoredeploystatus "github.com/golgoth31/sreportal/internal/readstore/deploystatus"
)

// fakePortalReader is a minimal PortalReader that returns a fixed portal list.
type fakePortalReader struct {
	portals []domainportal.PortalView
}

func (f *fakePortalReader) List(_ context.Context, _ domainportal.PortalFilters) ([]domainportal.PortalView, error) {
	return f.portals, nil
}

func (f *fakePortalReader) Subscribe() <-chan struct{} {
	return make(chan struct{})
}

func TestListDeployStatus(t *testing.T) {
	store := readstoredeploystatus.NewStore()

	deployedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	checkedAt := time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)
	commitDate := time.Date(2025, 12, 31, 10, 0, 0, 0, time.UTC)

	store.ReplaceForNamespace(tPortalMain, "default", []domaindeploystatus.Entry{
		{
			Key:   "default/api/app",
			Image: "ghcr.io/acme/api:abc1234",
			Workload: domaindeploystatus.WorkloadRef{
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "api",
				Container: "app",
			},
			SourceRepo:    "acme/api",
			DeployedRef:   "abc1234",
			DefaultBranch: "main",
			AheadBy:       2,
			PendingCommits: []domaindeploystatus.Commit{
				{Sha: "def5678", Message: "fix: login bug", Author: "alice", Date: commitDate, URL: "https://github.com/acme/api/commit/def5678"},
			},
			PendingTruncated: false,
			DeployedAt:       deployedAt,
			DeployRunURL:     "https://github.com/acme/api/actions/runs/1",
			State:            "behind",
			Error:            "",
			LastCheckedAt:    checkedAt,
		},
		{
			Key:   "default/worker/worker",
			Image: "ghcr.io/acme/worker:xyz9999",
			Workload: domaindeploystatus.WorkloadRef{
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "worker",
				Container: "worker",
			},
			SourceRepo:    "acme/worker",
			DeployedRef:   "xyz9999",
			DefaultBranch: "main",
			AheadBy:       0,
			State:         "ok",
			DeployedAt:    deployedAt,
			LastCheckedAt: checkedAt,
		},
	})

	svc := svcgrpc.NewDeployStatusService(store, nil)
	resp, err := svc.ListDeployStatus(context.Background(), connect.NewRequest(&genv1.ListDeployStatusRequest{
		Portal: tPortalMain,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Entries, 2)

	// Find the api entry for detailed field assertions.
	var apiEntry *genv1.DeployStatusEntry
	for _, e := range resp.Msg.Entries {
		if e.Key == "default/api/app" {
			apiEntry = e
			break
		}
	}
	require.NotNil(t, apiEntry, "expected entry with key default/api/app")

	assert.Equal(t, "ghcr.io/acme/api:abc1234", apiEntry.Image)
	assert.Equal(t, "acme/api", apiEntry.SourceRepo)
	assert.Equal(t, "abc1234", apiEntry.DeployedRef)
	assert.Equal(t, "main", apiEntry.DefaultBranch)
	assert.Equal(t, int32(2), apiEntry.AheadBy)
	assert.Equal(t, "behind", apiEntry.State)
	assert.Equal(t, "https://github.com/acme/api/actions/runs/1", apiEntry.DeployRunUrl)
	assert.False(t, apiEntry.PendingTruncated)

	require.NotNil(t, apiEntry.Workload)
	assert.Equal(t, "Deployment", apiEntry.Workload.Kind)
	assert.Equal(t, "default", apiEntry.Workload.Namespace)
	assert.Equal(t, "api", apiEntry.Workload.Name)
	assert.Equal(t, "app", apiEntry.Workload.Container)

	require.Len(t, apiEntry.PendingCommits, 1)
	c := apiEntry.PendingCommits[0]
	assert.Equal(t, "def5678", c.Sha)
	assert.Equal(t, "fix: login bug", c.Message)
	assert.Equal(t, "alice", c.Author)
	assert.Equal(t, "https://github.com/acme/api/commit/def5678", c.Url)
	require.NotNil(t, c.Date)
	assert.Equal(t, commitDate.Unix(), c.Date.AsTime().Unix())

	require.NotNil(t, apiEntry.DeployedAt)
	assert.Equal(t, deployedAt.Unix(), apiEntry.DeployedAt.AsTime().Unix())
	require.NotNil(t, apiEntry.LastCheckedAt)
	assert.Equal(t, checkedAt.Unix(), apiEntry.LastCheckedAt.AsTime().Unix())
}

func TestListDeployStatus_SearchFilter(t *testing.T) {
	store := readstoredeploystatus.NewStore()
	store.ReplaceForNamespace(tPortalMain, "default", []domaindeploystatus.Entry{
		{Key: "default/api/app", Image: "ghcr.io/acme/api:v1", SourceRepo: "acme/api", State: "ok"},
		{Key: "default/worker/worker", Image: "ghcr.io/acme/worker:v1", SourceRepo: "acme/worker", State: "ok"},
	})

	svc := svcgrpc.NewDeployStatusService(store, nil)
	resp, err := svc.ListDeployStatus(context.Background(), connect.NewRequest(&genv1.ListDeployStatusRequest{
		Portal: tPortalMain,
		Search: "worker",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Entries, 1)
	assert.Equal(t, "default/worker/worker", resp.Msg.Entries[0].Key)
}

func TestListDeployStatus_StateFilter(t *testing.T) {
	store := readstoredeploystatus.NewStore()
	store.ReplaceForNamespace(tPortalMain, "default", []domaindeploystatus.Entry{
		{Key: "default/api/app", Image: "ghcr.io/acme/api:v1", SourceRepo: "acme/api", State: "ok"},
		{Key: "default/worker/worker", Image: "ghcr.io/acme/worker:v1", SourceRepo: "acme/worker", State: "behind"},
	})

	svc := svcgrpc.NewDeployStatusService(store, nil)
	resp, err := svc.ListDeployStatus(context.Background(), connect.NewRequest(&genv1.ListDeployStatusRequest{
		Portal:      tPortalMain,
		StateFilter: "behind",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Entries, 1)
	assert.Equal(t, "default/worker/worker", resp.Msg.Entries[0].Key)
}

func TestListDeployStatus_FeatureDisabled(t *testing.T) {
	store := readstoredeploystatus.NewStore()
	store.ReplaceForNamespace(tPortalMain, "default", []domaindeploystatus.Entry{
		{Key: "default/api/app", Image: "ghcr.io/acme/api:v1", SourceRepo: "acme/api", State: "ok"},
	})

	disabled := false
	portalReader := &fakePortalReader{
		portals: []domainportal.PortalView{
			{
				Name: tPortalMain,
				Features: domainportal.PortalFeatures{
					DeployStatus: disabled,
				},
			},
		},
	}

	svc := svcgrpc.NewDeployStatusService(store, portalReader)
	resp, err := svc.ListDeployStatus(context.Background(), connect.NewRequest(&genv1.ListDeployStatusRequest{
		Portal: tPortalMain,
	}))
	require.NoError(t, err)
	// Feature disabled: empty response, no error.
	assert.Empty(t, resp.Msg.Entries)
}

func TestListDeployStatus_DefaultPortalMain(t *testing.T) {
	store := readstoredeploystatus.NewStore()
	store.ReplaceForNamespace("main", "default", []domaindeploystatus.Entry{
		{Key: "default/api/app", Image: "ghcr.io/acme/api:v1", SourceRepo: "acme/api", State: "ok"},
	})

	svc := svcgrpc.NewDeployStatusService(store, nil)
	// Empty portal defaults to "main".
	resp, err := svc.ListDeployStatus(context.Background(), connect.NewRequest(&genv1.ListDeployStatusRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Entries, 1)
}
