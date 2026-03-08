package alertmanagerclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/alertmanagerclient"
	"github.com/golgoth31/sreportal/internal/domain/alertmanager"
)

func TestGetActiveAlerts_ParsesResponse(t *testing.T) {
	body := `[
		{
			"fingerprint": "abc123",
			"labels": {"alertname": "HighCPU", "severity": "critical"},
			"annotations": {"summary": "CPU is high"},
			"status": {"state": "active"},
			"startsAt": "2026-03-08T10:00:00Z",
			"endsAt": "0001-01-01T00:00:00Z",
			"updatedAt": "2026-03-08T10:05:00Z"
		},
		{
			"fingerprint": "def456",
			"labels": {"alertname": "DiskFull", "severity": "warning"},
			"annotations": {},
			"status": {"state": "active"},
			"startsAt": "2026-03-08T09:00:00Z",
			"endsAt": "2026-03-08T12:00:00Z",
			"updatedAt": "2026-03-08T09:30:00Z"
		}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/alerts", r.URL.Path)
		assert.Equal(t, "true", r.URL.Query().Get("active"))
		assert.Equal(t, "false", r.URL.Query().Get("silenced"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := alertmanagerclient.NewClient()
	alerts, err := c.GetActiveAlerts(context.Background(), srv.URL)
	require.NoError(t, err)
	require.Len(t, alerts, 2)

	assert.Equal(t, "abc123", alerts[0].Fingerprint)
	assert.Equal(t, "HighCPU", alerts[0].AlertName())
	assert.Equal(t, alertmanager.StateActive, alerts[0].State)
	assert.Equal(t, "CPU is high", alerts[0].Annotations["summary"])

	assert.Equal(t, "def456", alerts[1].Fingerprint)
	assert.NotNil(t, alerts[1].EndsAt)
}

func TestGetActiveAlerts_WhenEmptyResponse_ReturnsEmptySlice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c := alertmanagerclient.NewClient()
	alerts, err := c.GetActiveAlerts(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Empty(t, alerts)
}

func TestGetActiveAlerts_WhenHTTPError_ReturnsErrFetchAlerts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := alertmanagerclient.NewClient()
	_, err := c.GetActiveAlerts(context.Background(), srv.URL)
	require.ErrorIs(t, err, alertmanager.ErrFetchAlerts)
}

func TestGetActiveAlerts_WhenInvalidJSON_ReturnsErrFetchAlerts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{bad json"))
	}))
	defer srv.Close()

	c := alertmanagerclient.NewClient()
	_, err := c.GetActiveAlerts(context.Background(), srv.URL)
	require.ErrorIs(t, err, alertmanager.ErrFetchAlerts)
}

func TestGetActiveAlerts_WhenServerUnreachable_ReturnsErrFetchAlerts(t *testing.T) {
	c := alertmanagerclient.NewClient()
	_, err := c.GetActiveAlerts(context.Background(), "http://127.0.0.1:1")
	require.ErrorIs(t, err, alertmanager.ErrFetchAlerts)
}

func TestGetActiveAlerts_WithCustomHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode([]any{}))
	}))
	defer srv.Close()

	custom := &http.Client{}
	c := alertmanagerclient.NewClient(alertmanagerclient.WithHTTPClient(custom))
	alerts, err := c.GetActiveAlerts(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Empty(t, alerts)
}
