package portal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	portalstore "github.com/golgoth31/sreportal/internal/readstore/portal"
)

func seedStore(t *testing.T) *portalstore.PortalStore {
	t.Helper()
	store := portalstore.NewPortalStore()
	ctx := context.Background()

	_ = store.Replace(ctx, "ns/main", domainportal.PortalView{
		Name: "main", Title: "Main Portal", Main: true,
		Namespace: "sreportal-system", Ready: true,
	})
	_ = store.Replace(ctx, "ns/dev", domainportal.PortalView{
		Name: "dev", Title: "Dev Portal", SubPath: "dev",
		Namespace: "sreportal-system", Ready: false,
	})
	_ = store.Replace(ctx, "ns/remote", domainportal.PortalView{
		Name: "remote", Title: "Remote Portal", IsRemote: true,
		URL: "https://remote.example.com", Namespace: "other-ns", Ready: true,
		RemoteSync: &domainportal.RemoteSyncView{
			LastSyncTime: "2026-01-15T10:00:00Z", RemoteTitle: "Remote", FQDNCount: 42,
		},
	})

	return store
}

func TestPortalStore_List_ReturnsAllSorted(t *testing.T) {
	store := seedStore(t)
	views, err := store.List(context.Background(), domainportal.PortalFilters{})

	require.NoError(t, err)
	require.Len(t, views, 3)
	assert.Equal(t, "dev", views[0].Name)
	assert.Equal(t, "main", views[1].Name)
	assert.Equal(t, "remote", views[2].Name)
}

func TestPortalStore_List_FiltersByNamespace(t *testing.T) {
	store := seedStore(t)
	views, err := store.List(context.Background(), domainportal.PortalFilters{
		Namespace: "sreportal-system",
	})

	require.NoError(t, err)
	require.Len(t, views, 2)
	assert.Equal(t, "dev", views[0].Name)
	assert.Equal(t, "main", views[1].Name)
}

func TestPortalStore_Delete_RemovesPortal(t *testing.T) {
	store := seedStore(t)
	err := store.Delete(context.Background(), "ns/dev")
	require.NoError(t, err)

	views, err := store.List(context.Background(), domainportal.PortalFilters{})
	require.NoError(t, err)
	assert.Len(t, views, 2)
}

func TestPortalStore_Subscribe_NotifiesOnChange(t *testing.T) {
	store := portalstore.NewPortalStore()
	ch := store.Subscribe()

	_ = store.Replace(context.Background(), "ns/test", domainportal.PortalView{Name: "test"})

	select {
	case <-ch:
		// expected
	default:
		t.Fatal("expected notification after Replace")
	}
}

func TestPortalStore_RemoteSyncPreserved(t *testing.T) {
	store := seedStore(t)
	views, err := store.List(context.Background(), domainportal.PortalFilters{Namespace: "other-ns"})

	require.NoError(t, err)
	require.Len(t, views, 1)
	require.NotNil(t, views[0].RemoteSync)
	assert.Equal(t, 42, views[0].RemoteSync.FQDNCount)
	assert.Equal(t, "Remote", views[0].RemoteSync.RemoteTitle)
}
