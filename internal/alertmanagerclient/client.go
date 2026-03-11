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
	receiversPath  = "/api/v2/receivers"
	silencesPath   = "/api/v2/silences"
	defaultTimeout = 10 * time.Second
)

// apiAlert is the JSON shape returned by Alertmanager API v2 (simplified, no receivers).
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

// apiAlertFull is the full gettableAlert shape with receivers and silencedBy.
type apiAlertFull struct {
	Fingerprint string             `json:"fingerprint"`
	Labels      map[string]string  `json:"labels"`
	Annotations map[string]string  `json:"annotations"`
	Receivers   []apiReceiver      `json:"receivers"`
	Status      apiAlertStatusFull `json:"status"`
	StartsAt    time.Time          `json:"startsAt"`
	EndsAt      time.Time          `json:"endsAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
}

type apiReceiver struct {
	Name string `json:"name"`
}

type apiAlertStatusFull struct {
	State       string   `json:"state"`
	SilencedBy  []string `json:"silencedBy"`
	InhibitedBy []string `json:"inhibitedBy"`
	MutedBy     []string `json:"mutedBy"`
}

// apiGettableSilence matches the Alertmanager API v2 gettableSilence.
type apiGettableSilence struct {
	ID        string           `json:"id"`
	Status    apiSilenceStatus `json:"status"`
	UpdatedAt time.Time        `json:"updatedAt"`
	apiSilence
}

type apiSilence struct {
	Matchers  []apiMatcher `json:"matchers"`
	StartsAt  time.Time    `json:"startsAt"`
	EndsAt    time.Time    `json:"endsAt"`
	CreatedBy string       `json:"createdBy"`
	Comment   string       `json:"comment"`
}

type apiSilenceStatus struct {
	State string `json:"state"`
}

type apiMatcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
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

// GetReceivers fetches all receivers from the Alertmanager API v2.
func (c *Client) GetReceivers(ctx context.Context, baseURL string) ([]alertmanager.Receiver, error) {
	reqURL := baseURL + receiversPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %v", alertmanager.ErrFetchReceivers, err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", alertmanager.ErrFetchReceivers, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", alertmanager.ErrFetchReceivers, resp.StatusCode)
	}
	var raw []apiReceiver
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", alertmanager.ErrFetchReceivers, err)
	}
	return toDomainReceivers(raw)
}

// GetSilences fetches all silences from the Alertmanager API v2.
func (c *Client) GetSilences(ctx context.Context, baseURL string) ([]alertmanager.Silence, error) {
	reqURL := baseURL + silencesPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %v", alertmanager.ErrFetchSilences, err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", alertmanager.ErrFetchSilences, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", alertmanager.ErrFetchSilences, resp.StatusCode)
	}
	var raw []apiGettableSilence
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", alertmanager.ErrFetchSilences, err)
	}
	return toDomainSilences(raw)
}

// GetAlertmanagerData fetches alerts (with receivers and silencedBy), silences,
// and receivers in one logical operation. Alerts include active, silenced, and inhibited.
func (c *Client) GetAlertmanagerData(ctx context.Context, baseURL string) (*alertmanager.AlertmanagerData, error) {
	// Fetch full alerts: active=true, silenced=true, inhibited=true to get status.silencedBy
	reqURL := baseURL + alertsPath + "?active=true&silenced=true&inhibited=true"
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
	var apiAlerts []apiAlertFull
	if err := json.NewDecoder(resp.Body).Decode(&apiAlerts); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", alertmanager.ErrFetchAlerts, err)
	}

	receivers, err := c.GetReceivers(ctx, baseURL)
	if err != nil {
		return nil, err
	}

	silences, err := c.GetSilences(ctx, baseURL)
	if err != nil {
		return nil, err
	}

	return &alertmanager.AlertmanagerData{
		Alerts:    toDomainAlertsFull(apiAlerts),
		Silences:  silences,
		Receivers: receivers,
	}, nil
}

func toDomainReceivers(raw []apiReceiver) ([]alertmanager.Receiver, error) {
	out := make([]alertmanager.Receiver, 0, len(raw))
	for _, r := range raw {
		rec, err := alertmanager.NewReceiver(r.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

func toDomainSilences(raw []apiGettableSilence) ([]alertmanager.Silence, error) {
	out := make([]alertmanager.Silence, 0, len(raw))
	for _, s := range raw {
		matchers := make([]alertmanager.Matcher, 0, len(s.Matchers))
		for _, m := range s.Matchers {
			matchers = append(matchers, alertmanager.Matcher{
				Name:    m.Name,
				Value:   m.Value,
				IsRegex: m.IsRegex,
			})
		}
		sil, err := alertmanager.NewSilence(
			s.ID,
			matchers,
			s.StartsAt, s.EndsAt,
			alertmanager.SilenceStatus(s.Status.State),
			s.CreatedBy, s.Comment,
			s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("silence %s: %w", s.ID, err)
		}
		out = append(out, sil)
	}
	return out, nil
}

func toDomainAlertsFull(raw []apiAlertFull) []alertmanager.Alert {
	alerts := make([]alertmanager.Alert, 0, len(raw))
	for _, a := range raw {
		receivers := make([]alertmanager.Receiver, 0, len(a.Receivers))
		for _, r := range a.Receivers {
			if rec, err := alertmanager.NewReceiver(r.Name); err == nil {
				receivers = append(receivers, rec)
			}
		}
		da := alertmanager.Alert{
			Fingerprint: a.Fingerprint,
			Labels:      a.Labels,
			Annotations: a.Annotations,
			State:       alertmanager.State(a.Status.State),
			StartsAt:    a.StartsAt,
			UpdatedAt:   a.UpdatedAt,
			Receivers:   receivers,
			SilencedBy:  a.Status.SilencedBy,
		}
		if !a.EndsAt.IsZero() {
			endsAt := a.EndsAt
			da.EndsAt = &endsAt
		}
		alerts = append(alerts, da)
	}
	return alerts
}
