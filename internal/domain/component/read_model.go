package component

import "time"

// ComponentStatus represents the operational status of a platform component.
type ComponentStatus string

const (
	StatusOperational ComponentStatus = "operational"
	StatusDegraded    ComponentStatus = "degraded"
	StatusPartialOut  ComponentStatus = "partial_outage"
	StatusMajorOutage ComponentStatus = "major_outage"
	StatusUnknown     ComponentStatus = "unknown"
	StatusMaintenance ComponentStatus = "maintenance"
)

// ComponentView is the read model for a platform component.
type ComponentView struct {
	Name             string
	DisplayName      string
	Description      string
	Group            string
	Link             string
	PortalRef        string
	DeclaredStatus   ComponentStatus
	ComputedStatus   ComponentStatus
	ActiveIncidents  int
	LastStatusChange time.Time
}

// statusSeverityOrder defines the severity hierarchy for global status computation.
var statusSeverityOrder = map[ComponentStatus]int{
	StatusMajorOutage: 6,
	StatusPartialOut:  5,
	StatusDegraded:    4,
	StatusMaintenance: 3,
	StatusUnknown:     2,
	StatusOperational: 1,
}

// ComputeGlobalStatus returns the worst status across all components.
func ComputeGlobalStatus(components []ComponentView) ComponentStatus {
	if len(components) == 0 {
		return StatusUnknown
	}

	worst := StatusOperational
	for _, c := range components {
		s := c.ComputedStatus
		if s == "" {
			s = c.DeclaredStatus
		}
		if statusSeverityOrder[s] > statusSeverityOrder[worst] {
			worst = s
		}
	}

	return worst
}
