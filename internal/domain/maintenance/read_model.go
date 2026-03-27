package maintenance

import "time"

// MaintenancePhase describes the lifecycle phase of a maintenance window.
type MaintenancePhase string

const (
	PhaseUpcoming   MaintenancePhase = "upcoming"
	PhaseInProgress MaintenancePhase = "in_progress"
	PhaseCompleted  MaintenancePhase = "completed"
)

// MaintenanceView is the read model for a maintenance window.
type MaintenanceView struct {
	Name           string
	Title          string
	Description    string
	PortalRef      string
	Components     []string
	ScheduledStart time.Time
	ScheduledEnd   time.Time
	AffectedStatus string
	Phase          MaintenancePhase
}

// ComputePhase returns the phase based on the given time.
func ComputePhase(now, scheduledStart, scheduledEnd time.Time) MaintenancePhase {
	if now.Before(scheduledStart) {
		return PhaseUpcoming
	}
	if !now.After(scheduledEnd) {
		return PhaseInProgress
	}
	return PhaseCompleted
}

// ComputeRequeue returns the duration until the next phase transition.
// Returns 0 if no requeue is needed (already completed).
func ComputeRequeue(now, scheduledStart, scheduledEnd time.Time) time.Duration {
	if now.Before(scheduledStart) {
		return scheduledStart.Sub(now)
	}
	if !now.After(scheduledEnd) {
		return scheduledEnd.Sub(now)
	}
	return 0
}

// AffectsComponent returns true if the maintenance is in progress and
// targets the given component name for the given portal.
func (v MaintenanceView) AffectsComponent(portalRef, componentName string) bool {
	if v.PortalRef != portalRef {
		return false
	}
	if v.Phase != PhaseInProgress {
		return false
	}
	for _, c := range v.Components {
		if c == componentName {
			return true
		}
	}
	return false
}
