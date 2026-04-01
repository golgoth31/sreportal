package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

func TestMergeDailyStatusHandler_CreatesEntryForToday(t *testing.T) {
	comp := &sreportalv1alpha1.Component{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Component, ChainData]{
		Resource: comp,
		Data:     ChainData{ComputedStatus: sreportalv1alpha1.ComputedStatusDegraded},
	}

	fixedNow := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	h := NewMergeDailyStatusHandler(func() time.Time { return fixedNow })

	err := h.Handle(context.Background(), rc)

	require.NoError(t, err)
	require.Len(t, comp.Status.DailyWorstStatus, 1)
	assert.Equal(t, "2026-04-01", comp.Status.DailyWorstStatus[0].Date)
	assert.Equal(t, sreportalv1alpha1.ComputedStatusDegraded, comp.Status.DailyWorstStatus[0].WorstStatus)
}

func TestMergeDailyStatusHandler_MergesWorstForSameDay(t *testing.T) {
	comp := &sreportalv1alpha1.Component{}
	comp.Status.DailyWorstStatus = []sreportalv1alpha1.DailyComponentStatus{
		{Date: "2026-04-01", WorstStatus: sreportalv1alpha1.ComputedStatusDegraded},
	}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Component, ChainData]{
		Resource: comp,
		Data:     ChainData{ComputedStatus: sreportalv1alpha1.ComputedStatusMajorOutage},
	}

	fixedNow := time.Date(2026, 4, 1, 14, 0, 0, 0, time.UTC)
	h := NewMergeDailyStatusHandler(func() time.Time { return fixedNow })

	err := h.Handle(context.Background(), rc)

	require.NoError(t, err)
	require.Len(t, comp.Status.DailyWorstStatus, 1)
	assert.Equal(t, sreportalv1alpha1.ComputedStatusMajorOutage, comp.Status.DailyWorstStatus[0].WorstStatus)
}

func TestMergeDailyStatusHandler_DoesNotSetRequeueAfter(t *testing.T) {
	comp := &sreportalv1alpha1.Component{}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Component, ChainData]{
		Resource: comp,
		Data:     ChainData{ComputedStatus: sreportalv1alpha1.ComputedStatusOperational},
	}

	fixedNow := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	h := NewMergeDailyStatusHandler(func() time.Time { return fixedNow })

	_ = h.Handle(context.Background(), rc)

	assert.Zero(t, rc.Result.RequeueAfter, "MergeDailyStatusHandler must not set RequeueAfter")
}

func TestUpdateStatusHandler_SetsRequeueAfterToMidnight(t *testing.T) {
	// This test only verifies the requeue calculation, not K8s interactions.
	// Full integration is tested in the envtest suite.
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	h := &UpdateStatusHandler{
		now: func() time.Time { return fixedNow },
	}

	// Directly test the requeue duration
	want := 12 * time.Hour
	got := h.requeueDuration()
	assert.Equal(t, want, got)
}
