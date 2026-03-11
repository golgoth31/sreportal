// Package alertmanager contains pure domain types for Alertmanager alert management.
// This package has no external dependencies and contains only business logic.
package alertmanager

import (
	"context"
	"time"
)

// State represents the state of an alert.
type State string

const (
	StateActive      State = "active"
	StateSuppressed  State = "suppressed"
	StateUnprocessed State = "unprocessed"
)

// Alert represents an active alert from Alertmanager with its labels.
// Receivers are the notification integrations this alert is routed to.
// SilencedBy contains the IDs of silences that suppress this alert.
type Alert struct {
	Fingerprint string
	Labels      map[string]string
	Annotations map[string]string
	State       State
	StartsAt    time.Time
	EndsAt      *time.Time
	UpdatedAt   time.Time
	Receivers   []Receiver
	SilencedBy  []string
}

// AlertName returns the alertname label, which is the conventional alert identifier.
func (a Alert) AlertName() string {
	return a.Labels["alertname"]
}

// IsSilenced returns true when the alert is suppressed by one or more silences.
func (a Alert) IsSilenced() bool {
	return len(a.SilencedBy) > 0
}

// Fetcher retrieves active alerts from an Alertmanager instance.
type Fetcher interface {
	GetActiveAlerts(ctx context.Context, baseURL string) ([]Alert, error)
}

// AlertmanagerData aggregates alerts (with receiver linkage and silence info),
// silences, and receivers from a single fetch. Used when the consumer needs
// to link alerts to receivers and identify silenced alerts.
type AlertmanagerData struct {
	Alerts    []Alert
	Silences  []Silence
	Receivers []Receiver
}

// DataFetcher retrieves full Alertmanager data: alerts with receivers and
// silencedBy info, plus silences and receivers.
type DataFetcher interface {
	GetAlertmanagerData(ctx context.Context, baseURL string) (*AlertmanagerData, error)
}
