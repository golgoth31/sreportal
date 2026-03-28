package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusSeverityRank(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   int
	}{
		{"operational", "operational", 1},
		{"unknown", "unknown", 2},
		{"maintenance", "maintenance", 3},
		{"degraded", "degraded", 4},
		{"partial_outage", "partial_outage", 5},
		{"major_outage", "major_outage", 6},
		{"unrecognized defaults to 0", "bogus", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, StatusSeverityRank(tc.status))
		})
	}
}

func TestStatusSeverityRank_Ordering(t *testing.T) {
	assert.Less(t, StatusSeverityRank("operational"), StatusSeverityRank("degraded"))
	assert.Less(t, StatusSeverityRank("degraded"), StatusSeverityRank("partial_outage"))
	assert.Less(t, StatusSeverityRank("partial_outage"), StatusSeverityRank("major_outage"))
	assert.Less(t, StatusSeverityRank("maintenance"), StatusSeverityRank("degraded"))
}
