package alertmanager_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainam "github.com/golgoth31/sreportal/internal/domain/alertmanagerreadmodel"
	amstore "github.com/golgoth31/sreportal/internal/readstore/alertmanager"
)

const portalMain = "main"

func seedStore(t *testing.T) *amstore.AlertmanagerStore {
	t.Helper()
	store := amstore.NewAlertmanagerStore()
	ctx := context.Background()
	now := time.Now()

	_ = store.Replace(ctx, "monitoring/am-prod", domainam.AlertmanagerView{
		Name: "am-prod", Namespace: "monitoring", PortalRef: portalMain,
		LocalURL: "http://am:9093", RemoteURL: "https://am.example.com",
		Ready: true, LastReconcileTime: &now,
		Alerts: []domainam.AlertView{
			{Fingerprint: "aaa", Labels: map[string]string{"alertname": "HighCPU", "severity": "critical"},
				State: "active", StartsAt: now, UpdatedAt: now},
			{Fingerprint: "bbb", Labels: map[string]string{"alertname": "DiskFull", "severity": "warning"},
				State: "active", StartsAt: now, UpdatedAt: now},
		},
	})

	_ = store.Replace(ctx, "default/am-other", domainam.AlertmanagerView{
		Name: "am-other", Namespace: "default", PortalRef: "other",
		LocalURL: "http://am2:9093", Ready: false,
	})

	return store
}

func TestAlertmanagerStore_List_ReturnsAllSorted(t *testing.T) {
	store := seedStore(t)
	views, err := store.List(context.Background(), domainam.AlertmanagerFilters{})

	require.NoError(t, err)
	require.Len(t, views, 2)
	assert.Equal(t, "am-other", views[0].Name) // default < monitoring
	assert.Equal(t, "am-prod", views[1].Name)
}

func TestAlertmanagerStore_List_FiltersByPortal(t *testing.T) {
	store := seedStore(t)
	views, err := store.List(context.Background(), domainam.AlertmanagerFilters{Portal: portalMain})

	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, "am-prod", views[0].Name)
}

func TestAlertmanagerStore_List_FiltersByNamespace(t *testing.T) {
	store := seedStore(t)
	views, err := store.List(context.Background(), domainam.AlertmanagerFilters{Namespace: "monitoring"})

	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, "am-prod", views[0].Name)
}

func TestAlertmanagerStore_List_CombinesFilters(t *testing.T) {
	store := seedStore(t)
	views, err := store.List(context.Background(), domainam.AlertmanagerFilters{
		Portal: portalMain, Namespace: "default",
	})

	require.NoError(t, err)
	assert.Empty(t, views)
}

func TestAlertmanagerStore_Delete_RemovesEntry(t *testing.T) {
	store := seedStore(t)
	err := store.Delete(context.Background(), "monitoring/am-prod")
	require.NoError(t, err)

	views, err := store.List(context.Background(), domainam.AlertmanagerFilters{})
	require.NoError(t, err)
	assert.Len(t, views, 1)
}

func TestAlertmanagerStore_AlertsPreserved(t *testing.T) {
	store := seedStore(t)
	views, err := store.List(context.Background(), domainam.AlertmanagerFilters{Portal: portalMain})

	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Len(t, views[0].Alerts, 2)
	assert.Equal(t, "HighCPU", views[0].Alerts[0].Labels["alertname"])
}

func TestAlertmanagerStore_Subscribe_NotifiesOnChange(t *testing.T) {
	store := amstore.NewAlertmanagerStore()
	ch := store.Subscribe()

	_ = store.Replace(context.Background(), "ns/test", domainam.AlertmanagerView{Name: "test"})

	select {
	case <-ch:
		// expected
	default:
		t.Fatal("expected notification after Replace")
	}
}
