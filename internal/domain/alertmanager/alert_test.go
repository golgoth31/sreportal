package alertmanager_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/golgoth31/sreportal/internal/domain/alertmanager"
)

func TestAlert_AlertName_ReturnsAlertNameLabel(t *testing.T) {
	a := alertmanager.Alert{
		Labels: map[string]string{
			labelAlertname: "HighMemoryUsage",
			"severity":     "critical",
		},
	}

	assert.Equal(t, "HighMemoryUsage", a.AlertName())
}

func TestAlert_AlertName_WhenMissing_ReturnsEmpty(t *testing.T) {
	a := alertmanager.Alert{
		Labels: map[string]string{"severity": "warning"},
	}

	assert.Equal(t, "", a.AlertName())
}

func TestAlert_IsSilenced_WhenSilencedByNotEmpty_ReturnsTrue(t *testing.T) {
	a := alertmanager.Alert{
		Labels:     map[string]string{labelAlertname: alertNameTest},
		SilencedBy: []string{"silence-uuid-1"},
	}
	assert.True(t, a.IsSilenced())
}

func TestAlert_IsSilenced_WhenSilencedByEmpty_ReturnsFalse(t *testing.T) {
	a := alertmanager.Alert{
		Labels:     map[string]string{labelAlertname: alertNameTest},
		SilencedBy: nil,
	}
	assert.False(t, a.IsSilenced())
}
