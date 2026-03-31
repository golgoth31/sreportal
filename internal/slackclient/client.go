// Package slackclient provides an HTTP client for the Slack API emoji.list endpoint.
package slackclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	emojiListURL   = "https://slack.com/api/emoji.list"
	defaultTimeout = 10 * time.Second
	maxAliasDepth  = 10
	aliasPrefix    = "alias:"
)

// ErrFetchEmojis is returned when the Slack API call fails at any stage.
var ErrFetchEmojis = errors.New("failed to fetch custom emojis")

// apiResponse is the JSON shape returned by the Slack emoji.list API.
type apiResponse struct {
	OK    bool              `json:"ok"`
	Error string            `json:"error,omitempty"`
	Emoji map[string]string `json:"emoji"`
}

// Client fetches custom emojis from a Slack workspace.
type Client struct {
	token      string
	baseURL    string
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

// WithBaseURL overrides the Slack API base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(cl *Client) {
		cl.baseURL = url
	}
}

// NewClient creates a new Slack API client.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		token:      token,
		baseURL:    emojiListURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// GetCustomEmojis fetches all custom emojis from the Slack workspace and resolves aliases.
func (c *Client) GetCustomEmojis(ctx context.Context) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %w", ErrFetchEmojis, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetchEmojis, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", ErrFetchEmojis, resp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("%w: decode response: %w", ErrFetchEmojis, err)
	}

	if !apiResp.OK {
		return nil, fmt.Errorf("%w: slack error: %s", ErrFetchEmojis, apiResp.Error)
	}

	return resolveAliases(apiResp.Emoji), nil
}

// resolveAliases resolves Slack emoji aliases (values starting with "alias:") to their
// target image URLs. Circular or excessively deep alias chains are dropped.
func resolveAliases(raw map[string]string) map[string]string {
	resolved := make(map[string]string, len(raw))

	for name, value := range raw {
		if url := resolve(raw, name, value, 0); url != "" {
			resolved[name] = url
		}
	}

	return resolved
}

// resolve follows an alias chain. name is the root emoji being resolved (not the
// current alias target), so target == name detects cycles back to the origin.
func resolve(raw map[string]string, name, value string, depth int) string {
	if depth >= maxAliasDepth {
		return ""
	}

	if !strings.HasPrefix(value, aliasPrefix) {
		return value
	}

	target := strings.TrimPrefix(value, aliasPrefix)
	if target == name {
		return ""
	}

	targetValue, ok := raw[target]
	if !ok {
		return ""
	}

	return resolve(raw, name, targetValue, depth+1)
}
