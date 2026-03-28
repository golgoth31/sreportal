package incident

import (
	"slices"
	"sort"
	"time"
)

// IncidentPhase describes the resolution stage of an incident.
type IncidentPhase string

const (
	PhaseInvestigating IncidentPhase = "investigating"
	PhaseIdentified    IncidentPhase = "identified"
	PhaseMonitoring    IncidentPhase = "monitoring"
	PhaseResolved      IncidentPhase = "resolved"
)

// IncidentSeverity describes how severe an incident is.
type IncidentSeverity string

const (
	SeverityCritical IncidentSeverity = "critical"
	SeverityMajor    IncidentSeverity = "major"
	SeverityMinor    IncidentSeverity = "minor"
)

// UpdateView is the read model for a single incident timeline entry.
type UpdateView struct {
	Timestamp time.Time
	Phase     IncidentPhase
	Message   string
}

// IncidentView is the read model for an incident.
type IncidentView struct {
	Name            string
	Title           string
	PortalRef       string
	Components      []string
	Severity        IncidentSeverity
	CurrentPhase    IncidentPhase
	Updates         []UpdateView
	StartedAt       time.Time
	ResolvedAt      time.Time
	DurationMinutes int
}

// severityToStatus maps incident severity to the component status it implies.
var severityToStatus = map[IncidentSeverity]string{
	SeverityCritical: "major_outage",
	SeverityMajor:    "partial_outage",
	SeverityMinor:    "degraded",
}

// SeverityToComponentStatus returns the component status implied by the given severity.
// Unknown severities default to "degraded".
func SeverityToComponentStatus(severity IncidentSeverity) string {
	if s, ok := severityToStatus[severity]; ok {
		return s
	}
	return "degraded"
}

// AffectsComponent returns true if the incident is active (non-resolved) and
// targets the given component name for the given portal.
func (v IncidentView) AffectsComponent(portalRef, componentName string) bool {
	if v.PortalRef != portalRef {
		return false
	}
	if v.CurrentPhase == PhaseResolved {
		return false
	}
	return slices.Contains(v.Components, componentName)
}

// ComponentStatus returns the component status implied by this incident's severity.
func (v IncidentView) ComponentStatus() string {
	return SeverityToComponentStatus(v.Severity)
}

// ComputedStatus holds the derived fields from the updates timeline.
type ComputedStatus struct {
	CurrentPhase    IncidentPhase
	StartedAt       time.Time
	ResolvedAt      time.Time
	DurationMinutes int
}

// ComputeStatus derives the current phase, timestamps, and duration from
// a list of updates. Updates are sorted by timestamp ascending internally.
// Returns an error if the updates slice is empty.
func ComputeStatus(updates []UpdateView) (ComputedStatus, error) {
	if len(updates) == 0 {
		return ComputedStatus{}, ErrNoUpdates
	}

	sorted := make([]UpdateView, len(updates))
	copy(sorted, updates)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	first := sorted[0]
	last := sorted[len(sorted)-1]

	cs := ComputedStatus{
		CurrentPhase: last.Phase,
		StartedAt:    first.Timestamp,
	}

	if last.Phase == PhaseResolved {
		cs.ResolvedAt = last.Timestamp
		cs.DurationMinutes = int(last.Timestamp.Sub(first.Timestamp).Minutes())
	}

	return cs, nil
}
