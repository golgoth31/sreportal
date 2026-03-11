package alertmanager_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/domain/alertmanager"
)

func TestNewReceiver_WhenNameEmpty_ReturnsErrInvalidReceiver(t *testing.T) {
	r, err := alertmanager.NewReceiver("")
	require.ErrorIs(t, err, alertmanager.ErrInvalidReceiver)
	assert.Equal(t, alertmanager.Receiver{}, r)
}

func TestNewReceiver_WhenNameValid_ReturnsReceiver(t *testing.T) {
	r, err := alertmanager.NewReceiver("team-oncall")
	require.NoError(t, err)
	assert.Equal(t, "team-oncall", r.Name())
}
