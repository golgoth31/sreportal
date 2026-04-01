package component

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeDailyWorst_EmptyHistory_CreatesEntry(t *testing.T) {
	got := MergeDailyWorst(nil, "2026-04-01", StatusOperational, 30)

	require.Len(t, got, 1)
	assert.Equal(t, "2026-04-01", got[0].Date)
	assert.Equal(t, StatusOperational, got[0].WorstStatus)
}

func TestMergeDailyWorst_SameDay_KeepsWorst(t *testing.T) {
	history := []DailyStatus{
		{Date: "2026-04-01", WorstStatus: StatusDegraded},
	}

	got := MergeDailyWorst(history, "2026-04-01", StatusMajorOutage, 30)

	require.Len(t, got, 1)
	assert.Equal(t, StatusMajorOutage, got[0].WorstStatus)
}

func TestMergeDailyWorst_SameDay_DoesNotDowngrade(t *testing.T) {
	history := []DailyStatus{
		{Date: "2026-04-01", WorstStatus: StatusMajorOutage},
	}

	got := MergeDailyWorst(history, "2026-04-01", StatusOperational, 30)

	require.Len(t, got, 1)
	assert.Equal(t, StatusMajorOutage, got[0].WorstStatus)
}

func TestMergeDailyWorst_NewDay_AppendsEntry(t *testing.T) {
	history := []DailyStatus{
		{Date: "2026-03-31", WorstStatus: StatusOperational},
	}

	got := MergeDailyWorst(history, "2026-04-01", StatusDegraded, 30)

	require.Len(t, got, 2)
	assert.Equal(t, "2026-03-31", got[0].Date)
	assert.Equal(t, "2026-04-01", got[1].Date)
	assert.Equal(t, StatusDegraded, got[1].WorstStatus)
}

func TestMergeDailyWorst_PrunesOldEntries(t *testing.T) {
	// Build 31 entries (one too many for a 30-day window)
	history := make([]DailyStatus, 30)
	for i := range history {
		day := time.Date(2026, 3, 2+i, 0, 0, 0, 0, time.UTC)
		history[i] = DailyStatus{
			Date:        day.Format("2006-01-02"),
			WorstStatus: StatusOperational,
		}
	}
	// history[0] = "2026-03-02", history[29] = "2026-03-31"

	got := MergeDailyWorst(history, "2026-04-01", StatusOperational, 30)

	require.Len(t, got, 30)
	assert.Equal(t, "2026-03-03", got[0].Date, "oldest entry should be pruned")
	assert.Equal(t, "2026-04-01", got[29].Date)
}

func TestMergeDailyWorst_MaintainsAscendingOrder(t *testing.T) {
	history := []DailyStatus{
		{Date: "2026-03-28", WorstStatus: StatusOperational},
		{Date: "2026-03-30", WorstStatus: StatusDegraded},
	}

	got := MergeDailyWorst(history, "2026-03-31", StatusOperational, 30)

	require.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		assert.Greater(t, got[i].Date, got[i-1].Date, "dates must be ascending")
	}
}

func TestMergeDailyWorst_UnknownStatus_TreatedAsRankZero(t *testing.T) {
	history := []DailyStatus{
		{Date: "2026-04-01", WorstStatus: StatusOperational},
	}

	got := MergeDailyWorst(history, "2026-04-01", ComponentStatus("bogus"), 30)

	require.Len(t, got, 1)
	// Operational (rank 1) > bogus (rank 0), so operational is kept
	assert.Equal(t, StatusOperational, got[0].WorstStatus)
}

func TestDurationUntilNextMidnightUTC(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want time.Duration
	}{
		{
			name: "noon UTC",
			now:  time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			want: 12 * time.Hour,
		},
		{
			name: "23:59:59 UTC",
			now:  time.Date(2026, 4, 1, 23, 59, 59, 0, time.UTC),
			want: 1 * time.Second,
		},
		{
			name: "exactly midnight — clamp to 1s",
			now:  time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			want: 1 * time.Second,
		},
		{
			name: "one nanosecond after midnight",
			now:  time.Date(2026, 4, 1, 0, 0, 0, 1, time.UTC),
			want: 24*time.Hour - 1*time.Nanosecond,
		},
		{
			name: "non-UTC input is converted",
			now:  time.Date(2026, 4, 1, 2, 0, 0, 0, time.FixedZone("CET", 2*3600)),
			// 02:00 CET = 00:00 UTC → should clamp to 1s
			want: 1 * time.Second,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DurationUntilNextMidnightUTC(tc.now)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDurationUntilNextMidnightUTC_AlwaysPositive(t *testing.T) {
	// Fuzz a few times across the day to ensure always > 0
	for h := range 24 {
		for m := range 60 {
			now := time.Date(2026, 4, 1, h, m, 0, 0, time.UTC)
			d := DurationUntilNextMidnightUTC(now)
			assert.Greater(t, d, time.Duration(0), "must always be positive at %02d:%02d", h, m)
		}
	}
}
