package incident

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeverityToComponentStatus(t *testing.T) {
	cases := []struct {
		name     string
		severity IncidentSeverity
		want     string
	}{
		{"critical maps to major_outage", SeverityCritical, "major_outage"},
		{"major maps to partial_outage", SeverityMajor, "partial_outage"},
		{"minor maps to degraded", SeverityMinor, "degraded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SeverityToComponentStatus(tc.severity)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSeverityToComponentStatus_UnknownSeverity(t *testing.T) {
	got := SeverityToComponentStatus("unknown")
	assert.Equal(t, "degraded", got, "unknown severity should default to degraded")
}

func TestIncidentView_AffectsComponent(t *testing.T) {
	view := IncidentView{
		PortalRef:    "portal-a",
		Components:   []string{"comp-1", "comp-2"},
		CurrentPhase: PhaseInvestigating,
	}

	cases := []struct {
		name          string
		portalRef     string
		componentName string
		want          bool
	}{
		{"matching portal and component", "portal-a", "comp-1", true},
		{"matching portal, different component", "portal-a", "comp-3", false},
		{"different portal, matching component", "portal-b", "comp-1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := view.AffectsComponent(tc.portalRef, tc.componentName)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIncidentView_AffectsComponent_ResolvedIncident(t *testing.T) {
	view := IncidentView{
		PortalRef:    "portal-a",
		Components:   []string{"comp-1"},
		CurrentPhase: PhaseResolved,
	}

	require.False(t, view.AffectsComponent("portal-a", "comp-1"),
		"resolved incident should not affect any component")
}

func TestIncidentView_ComponentStatus(t *testing.T) {
	cases := []struct {
		name     string
		severity IncidentSeverity
		want     string
	}{
		{"critical", SeverityCritical, "major_outage"},
		{"major", SeverityMajor, "partial_outage"},
		{"minor", SeverityMinor, "degraded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			view := IncidentView{Severity: tc.severity}
			assert.Equal(t, tc.want, view.ComponentStatus())
		})
	}
}
