// Package alertmanager contains pure domain types for Alertmanager alert management.
// This package has no external dependencies and contains only business logic.
package alertmanager

import (
	"context"
	"errors"
	"time"
)

// State represents the state of an alert.
type State string

const (
	StateActive      State = "active"
	StateSuppressed  State = "suppressed"
	StateUnprocessed State = "unprocessed"
)

var (
	ErrFetchAlerts  = errors.New("failed to fetch alerts from alertmanager")
	ErrInvalidURL   = errors.New("invalid alertmanager URL")
	ErrInvalidState = errors.New("invalid alert state")
)

// Alert represents an active alert from Alertmanager with its labels.
type Alert struct {
	Fingerprint string
	Labels      map[string]string
	Annotations map[string]string
	State       State
	StartsAt    time.Time
	EndsAt      *time.Time
	UpdatedAt   time.Time
}

// AlertName returns the alertname label, which is the conventional alert identifier.
func (a Alert) AlertName() string {
	return a.Labels["alertname"]
}

// Fetcher retrieves active alerts from an Alertmanager instance.
type Fetcher interface {
	GetActiveAlerts(ctx context.Context, baseURL string) ([]Alert, error)
}
