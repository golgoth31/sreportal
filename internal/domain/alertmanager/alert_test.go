package alertmanager_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/golgoth31/sreportal/internal/domain/alertmanager"
)

func TestAlert_AlertName_ReturnsAlertNameLabel(t *testing.T) {
	a := alertmanager.Alert{
		Labels: map[string]string{
			"alertname": "HighMemoryUsage",
			"severity":  "critical",
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
