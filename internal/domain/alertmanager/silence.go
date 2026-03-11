package alertmanager

import "time"

// SilenceStatus is the state of a silence in Alertmanager.
type SilenceStatus string

const (
	SilenceStatusActive  SilenceStatus = "active"
	SilenceStatusPending SilenceStatus = "pending"
	SilenceStatusExpired SilenceStatus = "expired"
)

// Matcher represents a label matcher used in a silence.
type Matcher struct {
	Name    string
	Value   string
	IsRegex bool
}

// Silence represents a mute rule in Alertmanager.
// Alerts matching the matchers are silenced for the given time window.
type Silence struct {
	id        string
	matchers  []Matcher
	startsAt  time.Time
	endsAt    time.Time
	status    SilenceStatus
	createdBy string
	comment   string
	updatedAt time.Time
}

// NewSilence creates a Silence. Validates that ID, matchers and times are valid.
func NewSilence(
	id string,
	matchers []Matcher,
	startsAt, endsAt time.Time,
	status SilenceStatus,
	createdBy, comment string,
	updatedAt time.Time,
) (Silence, error) {
	if id == "" {
		return Silence{}, ErrInvalidSilence
	}
	if len(matchers) == 0 {
		return Silence{}, ErrInvalidSilence
	}
	if !endsAt.After(startsAt) {
		return Silence{}, ErrInvalidSilence
	}
	return Silence{
		id:        id,
		matchers:  matchers,
		startsAt:  startsAt,
		endsAt:    endsAt,
		status:    status,
		createdBy: createdBy,
		comment:   comment,
		updatedAt: updatedAt,
	}, nil
}

// ID returns the silence UUID.
func (s Silence) ID() string { return s.id }

// Matchers returns the label matchers.
func (s Silence) Matchers() []Matcher { return s.matchers }

// StartsAt returns when the silence becomes active.
func (s Silence) StartsAt() time.Time { return s.startsAt }

// EndsAt returns when the silence expires.
func (s Silence) EndsAt() time.Time { return s.endsAt }

// Status returns the silence state.
func (s Silence) Status() SilenceStatus { return s.status }

// CreatedBy returns the creator identifier.
func (s Silence) CreatedBy() string { return s.createdBy }

// Comment returns the silence comment.
func (s Silence) Comment() string { return s.comment }

// UpdatedAt returns the last update timestamp.
func (s Silence) UpdatedAt() time.Time { return s.updatedAt }
