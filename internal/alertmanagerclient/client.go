// Package alertmanagerclient provides an HTTP client for the Alertmanager API v2.
package alertmanagerclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golgoth31/sreportal/internal/domain/alertmanager"
)

const (
	alertsPath     = "/api/v2/alerts"
	defaultTimeout = 10 * time.Second
)

// apiAlert is the JSON shape returned by Alertmanager API v2.
type apiAlert struct {
	Fingerprint string            `json:"fingerprint"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Status      apiAlertStatus    `json:"status"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

type apiAlertStatus struct {
	State string `json:"state"`
}

// Client fetches alerts from an Alertmanager instance over HTTP.
type Client struct {
	httpClient *http.Client
}

// Option configures the Client.
type Option func(*Client)

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) {
		cl.httpClient = c
	}
}

// NewClient creates a new Alertmanager HTTP client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// GetActiveAlerts fetches active alerts from the Alertmanager API v2.
func (c *Client) GetActiveAlerts(ctx context.Context, baseURL string) ([]alertmanager.Alert, error) {
	reqURL := baseURL + alertsPath + "?active=true&silenced=false&inhibited=false"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %v", alertmanager.ErrFetchAlerts, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", alertmanager.ErrFetchAlerts, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", alertmanager.ErrFetchAlerts, resp.StatusCode)
	}

	var apiAlerts []apiAlert
	if err := json.NewDecoder(resp.Body).Decode(&apiAlerts); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", alertmanager.ErrFetchAlerts, err)
	}

	return toDomain(apiAlerts), nil
}

func toDomain(raw []apiAlert) []alertmanager.Alert {
	alerts := make([]alertmanager.Alert, 0, len(raw))
	for _, a := range raw {
		da := alertmanager.Alert{
			Fingerprint: a.Fingerprint,
			Labels:      a.Labels,
			Annotations: a.Annotations,
			State:       alertmanager.State(a.Status.State),
			StartsAt:    a.StartsAt,
			UpdatedAt:   a.UpdatedAt,
		}
		if !a.EndsAt.IsZero() {
			endsAt := a.EndsAt
			da.EndsAt = &endsAt
		}
		alerts = append(alerts, da)
	}
	return alerts
}
