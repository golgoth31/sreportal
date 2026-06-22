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

	store.ReplaceForNamespace(tPortalMain, tNsDefault, []domaindeploystatus.Entry{
		{
			Key:   tKeyAPIApp,
			Image: "ghcr.io/acme/api:abc1234",
			Workload: domaindeploystatus.WorkloadRef{
				Kind:      tWorkloadKindDeployment,
				Namespace: tNsDefault,
				Name:      "api",
				Container: "app",
			},
			SourceRepo:    tRepoACMEAPI,
			DeployedRef:   "abc1234",
			DefaultBranch: "main",
			AheadBy:       2,
			PendingCommits: []domaindeploystatus.Commit{
				{Sha: "def5678", Message: "fix: login bug", Author: "alice", Date: commitDate, URL: "https://github.com/acme/api/commit/def5678"},
			},
			PendingTruncated: false,
			DeployedAt:       deployedAt,
			DeployRunURL:     "https://github.com/acme/api/actions/runs/1",
			State:            tStateBehind,
			Error:            "",
			LastCheckedAt:    checkedAt,
		},
		{
			Key:   tKeyWorkerWorker,
			Image: "ghcr.io/acme/worker:xyz9999",
			Workload: domaindeploystatus.WorkloadRef{
				Kind:      tWorkloadKindDeployment,
				Namespace: tNsDefault,
				Name:      tNameWorker,
				Container: tNameWorker,
			},
			SourceRepo:    tRepoACMEWorker,
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
		if e.Key == tKeyAPIApp {
			apiEntry = e
			break
		}
	}
	require.NotNil(t, apiEntry, "expected entry with key default/api/app")

	assert.Equal(t, "ghcr.io/acme/api:abc1234", apiEntry.Image)
	assert.Equal(t, tRepoACMEAPI, apiEntry.SourceRepo)
	assert.Equal(t, "abc1234", apiEntry.DeployedRef)
	assert.Equal(t, "main", apiEntry.DefaultBranch)
	assert.Equal(t, int32(2), apiEntry.AheadBy)
	assert.Equal(t, tStateBehind, apiEntry.State)
	assert.Equal(t, "https://github.com/acme/api/actions/runs/1", apiEntry.DeployRunUrl)
	assert.False(t, apiEntry.PendingTruncated)

	require.NotNil(t, apiEntry.Workload)
	assert.Equal(t, "Deployment", apiEntry.Workload.Kind)
	assert.Equal(t, tNsDefault, apiEntry.Workload.Namespace)
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
	store.ReplaceForNamespace(tPortalMain, tNsDefault, []domaindeploystatus.Entry{
		{Key: tKeyAPIApp, Image: tImageACMEAPIv1, SourceRepo: tRepoACMEAPI, State: "ok"},
		{Key: tKeyWorkerWorker, Image: "ghcr.io/acme/worker:v1", SourceRepo: tRepoACMEWorker, State: "ok"},
	})

	svc := svcgrpc.NewDeployStatusService(store, nil)
	resp, err := svc.ListDeployStatus(context.Background(), connect.NewRequest(&genv1.ListDeployStatusRequest{
		Portal: tPortalMain,
		Search: tNameWorker,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Entries, 1)
	assert.Equal(t, tKeyWorkerWorker, resp.Msg.Entries[0].Key)
}

func TestListDeployStatus_StateFilter(t *testing.T) {
	store := readstoredeploystatus.NewStore()
	store.ReplaceForNamespace(tPortalMain, tNsDefault, []domaindeploystatus.Entry{
		{Key: tKeyAPIApp, Image: tImageACMEAPIv1, SourceRepo: tRepoACMEAPI, State: "ok"},
		{Key: tKeyWorkerWorker, Image: "ghcr.io/acme/worker:v1", SourceRepo: tRepoACMEWorker, State: tStateBehind},
	})

	svc := svcgrpc.NewDeployStatusService(store, nil)
	resp, err := svc.ListDeployStatus(context.Background(), connect.NewRequest(&genv1.ListDeployStatusRequest{
		Portal:      tPortalMain,
		StateFilter: tStateBehind,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Entries, 1)
	assert.Equal(t, tKeyWorkerWorker, resp.Msg.Entries[0].Key)
}

func TestListDeployStatus_FeatureDisabled(t *testing.T) {
	store := readstoredeploystatus.NewStore()
	store.ReplaceForNamespace(tPortalMain, tNsDefault, []domaindeploystatus.Entry{
		{Key: tKeyAPIApp, Image: tImageACMEAPIv1, SourceRepo: tRepoACMEAPI, State: "ok"},
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
	store.ReplaceForNamespace("main", tNsDefault, []domaindeploystatus.Entry{
		{Key: tKeyAPIApp, Image: tImageACMEAPIv1, SourceRepo: tRepoACMEAPI, State: "ok"},
	})

	svc := svcgrpc.NewDeployStatusService(store, nil)
	// Empty portal defaults to "main".
	resp, err := svc.ListDeployStatus(context.Background(), connect.NewRequest(&genv1.ListDeployStatusRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Entries, 1)
}
