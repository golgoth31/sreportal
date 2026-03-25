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

func TestReleaseStore_ReplaceAndListEntries(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()

	entries := []domainrelease.EntryView{
		{Type: "deployment", Version: "v1.0.0", Origin: "ci", Date: time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)},
		{Type: "rollback", Version: "v0.9.0", Origin: "manual", Date: time.Date(2026, 3, 25, 14, 0, 0, 0, time.UTC)},
	}

	err := store.Replace(ctx, "2026-03-25", entries)
	require.NoError(t, err)

	got, err := store.ListEntries(ctx, "2026-03-25")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, "deployment", got[0].Type)
	assert.Equal(t, "rollback", got[1].Type)
}

func TestReleaseStore_ListEntries_UnknownDay(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()

	got, err := store.ListEntries(ctx, "2026-01-01")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestReleaseStore_ListDays_Sorted(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()

	_ = store.Replace(ctx, "2026-03-25", []domainrelease.EntryView{{Type: "deploy", Origin: "ci", Date: time.Now()}})
	_ = store.Replace(ctx, "2026-03-20", []domainrelease.EntryView{{Type: "deploy", Origin: "ci", Date: time.Now()}})
	_ = store.Replace(ctx, "2026-03-22", []domainrelease.EntryView{{Type: "deploy", Origin: "ci", Date: time.Now()}})

	days, err := store.ListDays(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-03-20", "2026-03-22", "2026-03-25"}, days)
}

func TestReleaseStore_Delete(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()

	_ = store.Replace(ctx, "2026-03-25", []domainrelease.EntryView{{Type: "deploy", Origin: "ci", Date: time.Now()}})

	err := store.Delete(ctx, "2026-03-25")
	require.NoError(t, err)

	got, err := store.ListEntries(ctx, "2026-03-25")
	require.NoError(t, err)
	assert.Empty(t, got)

	days, err := store.ListDays(ctx)
	require.NoError(t, err)
	assert.Empty(t, days)
}

func TestReleaseStore_Subscribe(t *testing.T) {
	ctx := context.Background()
	store := releasereadstore.NewReleaseStore()

	ch := store.Subscribe()

	// Should not be closed yet
	select {
	case <-ch:
		t.Fatal("channel should not be closed before mutation")
	default:
	}

	_ = store.Replace(ctx, "2026-03-25", []domainrelease.EntryView{{Type: "deploy", Origin: "ci", Date: time.Now()}})

	// Should be closed after mutation
	select {
	case <-ch:
		// OK
	case <-time.After(time.Second):
		t.Fatal("channel should be closed after mutation")
	}
}
