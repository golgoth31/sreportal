package release_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	releasereadstore "github.com/golgoth31/sreportal/internal/readstore/release"
)

func entry(portal, day, typ string, d time.Time) domainrelease.EntryView {
	return domainrelease.EntryView{
		PortalRef: portal,
		Day:       day,
		Type:      typ,
		Origin:    "ci",
		Date:      d,
	}
}

func TestReleaseStore_ReplaceAndListEntries(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()

	d := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	entries := []domainrelease.EntryView{
		entry("main", "2026-03-25", "deployment", d),
		{PortalRef: "main", Day: "2026-03-25", Type: "rollback", Version: "v0.9.0", Origin: "manual", Date: time.Date(2026, 3, 25, 14, 0, 0, 0, time.UTC)},
	}

	err := store.Replace(ctx, "default/release-2026-03-25", entries)
	require.NoError(t, err)

	got, err := store.ListEntries(ctx, "2026-03-25", "main")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "deployment", got[0].Type)
	assert.Equal(t, "rollback", got[1].Type)
}

func TestReleaseStore_ListEntries_FiltersByPortal(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()
	d := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	require.NoError(t, store.Replace(ctx, "default/r1", []domainrelease.EntryView{
		entry("portalA", "2026-03-25", "deploy", d),
	}))
	require.NoError(t, store.Replace(ctx, "default/r2", []domainrelease.EntryView{
		entry("portalB", "2026-03-25", "hotfix", d),
	}))

	gotA, err := store.ListEntries(ctx, "2026-03-25", "portalA")
	require.NoError(t, err)
	require.Len(t, gotA, 1)
	assert.Equal(t, "deploy", gotA[0].Type)

	gotB, err := store.ListEntries(ctx, "2026-03-25", "portalB")
	require.NoError(t, err)
	require.Len(t, gotB, 1)
	assert.Equal(t, "hotfix", gotB[0].Type)
}

func TestReleaseStore_ListEntries_UnknownDay(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()

	got, err := store.ListEntries(ctx, "2026-01-01", "main")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestReleaseStore_ListDays_Sorted(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()

	_ = store.Replace(ctx, "default/a", []domainrelease.EntryView{entry("main", "2026-03-25", "deploy", time.Now())})
	_ = store.Replace(ctx, "default/b", []domainrelease.EntryView{entry("main", "2026-03-20", "deploy", time.Now())})
	_ = store.Replace(ctx, "default/c", []domainrelease.EntryView{entry("main", "2026-03-22", "deploy", time.Now())})

	days, err := store.ListDays(ctx, "main")
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-03-20", "2026-03-22", "2026-03-25"}, days)
}

func TestReleaseStore_ListDays_FiltersByPortal(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()
	_ = store.Replace(ctx, "default/x", []domainrelease.EntryView{entry("portalA", "2026-03-10", "d", time.Now())})
	_ = store.Replace(ctx, "default/y", []domainrelease.EntryView{entry("portalB", "2026-03-11", "d", time.Now())})

	days, err := store.ListDays(ctx, "portalA")
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-03-10"}, days)
}

func TestReleaseStore_Delete(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()

	_ = store.Replace(ctx, "default/release-2026-03-25", []domainrelease.EntryView{
		entry("main", "2026-03-25", "deploy", time.Now()),
	})

	err := store.Delete(ctx, "default/release-2026-03-25")
	require.NoError(t, err)

	got, err := store.ListEntries(ctx, "2026-03-25", "main")
	require.NoError(t, err)
	assert.Empty(t, got)

	days, err := store.ListDays(ctx, "main")
	require.NoError(t, err)
	assert.Empty(t, days)
}

func TestReleaseStore_Subscribe(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()

	ch := store.Subscribe()

	select {
	case <-ch:
		t.Fatal("channel should not be closed before mutation")
	default:
	}

	_ = store.Replace(ctx, "default/r", []domainrelease.EntryView{entry("main", "2026-03-25", "deploy", time.Now())})

	select {
	case <-ch:
		// OK
	case <-time.After(time.Second):
		t.Fatal("channel should be closed after mutation")
	}
}
