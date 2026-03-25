package alertmanagerreadmodel

import "time"

// AlertmanagerView is the read-side projection of an Alertmanager resource.
type AlertmanagerView struct {
	Name              string
	Namespace         string
	PortalRef         string
	LocalURL          string
	RemoteURL         string
	Ready             bool
	LastReconcileTime *time.Time
	Alerts            []AlertView
	Silences          []SilenceView
}

// AlertView is the read-side projection of an alert.
type AlertView struct {
	Fingerprint string
	Labels      map[string]string
	Annotations map[string]string
	State       string
	StartsAt    time.Time
	EndsAt      *time.Time
	UpdatedAt   time.Time
	Receivers   []string
	SilencedBy  []string
}

// SilenceView is the read-side projection of a silence.
type SilenceView struct {
	ID        string
	Matchers  []MatcherView
	StartsAt  time.Time
	EndsAt    time.Time
	Status    string
	CreatedBy string
	Comment   string
	UpdatedAt time.Time
}

// MatcherView is a label matcher within a silence.
type MatcherView struct {
	Name    string
	Value   string
	IsRegex bool
}

// AlertmanagerFilters are the criteria for listing Alertmanager resources.
type AlertmanagerFilters struct {
	Portal    string
	Namespace string
}
