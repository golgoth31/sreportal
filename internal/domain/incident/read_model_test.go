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
		{"critical maps to major_outage", SeverityCritical, statusMajorOutage},
		{"major maps to partial_outage", SeverityMajor, statusPartialOut},
		{"minor maps to degraded", SeverityMinor, statusDegraded},
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
	assert.Equal(t, statusDegraded, got, "unknown severity should default to degraded")
}

func TestIncidentView_AffectsComponent(t *testing.T) {
	view := IncidentView{
		PortalRef:    tPortalA,
		Components:   []string{tCompID, "comp-2"},
		CurrentPhase: PhaseInvestigating,
	}

	cases := []struct {
		name          string
		portalRef     string
		componentName string
		want          bool
	}{
		{"matching portal and component", tPortalA, tCompID, true},
		{"matching portal, different component", tPortalA, "comp-3", false},
		{"different portal, matching component", "portal-b", tCompID, false},
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
		PortalRef:    tPortalA,
		Components:   []string{tCompID},
		CurrentPhase: PhaseResolved,
	}

	require.False(t, view.AffectsComponent(tPortalA, tCompID),
		"resolved incident should not affect any component")
}

func TestIncidentView_ComponentStatus(t *testing.T) {
	cases := []struct {
		name     string
		severity IncidentSeverity
		want     string
	}{
		{"critical", SeverityCritical, statusMajorOutage},
		{"major", SeverityMajor, statusPartialOut},
		{"minor", SeverityMinor, statusDegraded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			view := IncidentView{Severity: tc.severity}
			assert.Equal(t, tc.want, view.ComponentStatus())
		})
	}
}
