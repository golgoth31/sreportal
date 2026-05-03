package alertmanager_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/domain/alertmanager"
)

func TestNewSilence_WhenIDEmpty_ReturnsErrInvalidSilence(t *testing.T) {
	startsAt := time.Now()
	endsAt := startsAt.Add(time.Hour)
	s, err := alertmanager.NewSilence(
		"",
		[]alertmanager.Matcher{{Name: labelAlertname, Value: alertNameTest}},
		startsAt, endsAt,
		alertmanager.SilenceStatusActive,
		"ops@example.com", "maintenance",
		time.Now(),
	)
	require.ErrorIs(t, err, alertmanager.ErrInvalidSilence)
	assert.Equal(t, alertmanager.Silence{}, s)
}

func TestNewSilence_WhenMatchersEmpty_ReturnsErrInvalidSilence(t *testing.T) {
	startsAt := time.Now()
	endsAt := startsAt.Add(time.Hour)
	s, err := alertmanager.NewSilence(
		"abc-123",
		nil,
		startsAt, endsAt,
		alertmanager.SilenceStatusActive,
		"ops@example.com", "maintenance",
		time.Now(),
	)
	require.ErrorIs(t, err, alertmanager.ErrInvalidSilence)
	assert.Equal(t, alertmanager.Silence{}, s)
}

func TestNewSilence_WhenEndsAtBeforeStartsAt_ReturnsErrInvalidSilence(t *testing.T) {
	startsAt := time.Now()
	endsAt := startsAt.Add(-time.Hour)
	s, err := alertmanager.NewSilence(
		"abc-123",
		[]alertmanager.Matcher{{Name: labelAlertname, Value: alertNameTest}},
		startsAt, endsAt,
		alertmanager.SilenceStatusActive,
		"ops", "comment",
		time.Now(),
	)
	require.ErrorIs(t, err, alertmanager.ErrInvalidSilence)
	assert.Equal(t, alertmanager.Silence{}, s)
}

func TestNewSilence_WhenValid_ReturnsSilence(t *testing.T) {
	startsAt := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
	endsAt := startsAt.Add(2 * time.Hour)
	updatedAt := time.Now()
	matchers := []alertmanager.Matcher{
		{Name: "alertname", Value: "HighCPU", IsRegex: false},
	}

	s, err := alertmanager.NewSilence(
		"silence-uuid-123",
		matchers,
		startsAt, endsAt,
		alertmanager.SilenceStatusActive,
		"ops@example.com",
		"Planned maintenance",
		updatedAt,
	)
	require.NoError(t, err)
	assert.Equal(t, "silence-uuid-123", s.ID())
	assert.Equal(t, matchers, s.Matchers())
	assert.Equal(t, startsAt, s.StartsAt())
	assert.Equal(t, endsAt, s.EndsAt())
	assert.Equal(t, alertmanager.SilenceStatusActive, s.Status())
	assert.Equal(t, "ops@example.com", s.CreatedBy())
	assert.Equal(t, "Planned maintenance", s.Comment())
	assert.Equal(t, updatedAt, s.UpdatedAt())
}
