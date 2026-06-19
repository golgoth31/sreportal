package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/golgoth31/sreportal/internal/domain/forge"
)

const (
	defaultBaseURL        = "https://api.github.com"
	defaultMaxRetries     = 3
	defaultInitialBackoff = 500 * time.Millisecond
	defaultTimeout        = 20 * time.Second
)

// Config configures a GitHub Client.
type Config struct {
	// TokenSource provides the bearer token for each request.
	TokenSource TokenSource
	// BaseURL overrides the API base (default: "https://api.github.com").
	// Override for GHES or tests.
	BaseURL string
	// MaxRetries is the maximum number of retries on 429/5xx/network errors.
	// Default: 3.
	MaxRetries int
	// InitialBackoff is the first backoff duration (doubles each retry).
	// Default: 500ms.
	InitialBackoff time.Duration
	// HTTPClient overrides the HTTP client used for all requests.
	// When nil, defaults to &http.Client{Timeout: 15s}.
	HTTPClient *http.Client
}

// nonRetryableError wraps a 4xx HTTP error. The client stops retrying on these.
type nonRetryableError struct {
	StatusCode int
	Body       string
}

func (e *nonRetryableError) Error() string {
	return fmt.Sprintf("github: HTTP %d: %s", e.StatusCode, e.Body)
}

// Client implements forge.Client via the GitHub REST API.
// All methods are safe for concurrent use.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// Compile-time interface compliance check.
var _ forge.Client = (*Client)(nil)

// NewClient returns a new GitHub Client. cfg.TokenSource must not be nil.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = defaultMaxRetries
	}
	if cfg.InitialBackoff == 0 {
		cfg.InitialBackoff = defaultInitialBackoff
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
	}
}

// DefaultBranch returns the repository's default branch name.
func (c *Client) DefaultBranch(ctx context.Context, ref forge.RepoRef) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.cfg.BaseURL, ref.Owner, ref.Repo)

	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := c.getJSON(ctx, url, &repo); err != nil {
		return "", fmt.Errorf("github DefaultBranch %s/%s: %w", ref.Owner, ref.Repo, err)
	}
	return repo.DefaultBranch, nil
}

// Compare returns commits ahead of base up to head. Merge commits are flagged
// via Commit.Merge when they have more than one parent.
func (c *Client) Compare(ctx context.Context, ref forge.RepoRef, base, head string) (forge.CompareResult, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/compare/%s...%s",
		c.cfg.BaseURL, ref.Owner, ref.Repo, base, head)

	var resp apiCompareResponse
	if err := c.getJSON(ctx, url, &resp); err != nil {
		return forge.CompareResult{}, fmt.Errorf("github Compare %s/%s %s...%s: %w", ref.Owner, ref.Repo, base, head, err)
	}

	commits := make([]forge.Commit, 0, len(resp.Commits))
	for _, rc := range resp.Commits {
		commits = append(commits, forge.Commit{
			SHA:     rc.SHA,
			Message: rc.Commit.Message,
			Author:  rc.Commit.Author.Name,
			Date:    rc.Commit.Author.Date,
			URL:     rc.HTMLURL,
			Merge:   len(rc.Parents) > 1,
		})
	}

	// GitHub marks diverged comparisons with status "diverged" and may truncate.
	truncated := resp.Status == "diverged" && len(resp.Commits) == 0 && resp.AheadBy > 0

	return forge.CompareResult{
		AheadBy:   resp.AheadBy,
		Commits:   commits,
		Truncated: truncated,
	}, nil
}

// LatestWorkflowRun returns the URL of the most recent run of workflowFile on branch.
// Best-effort: returns ("", nil) when not resolvable so the caller may fall back to
// the generic CI page. Errors from the GitHub API are intentionally swallowed.
func (c *Client) LatestWorkflowRun(ctx context.Context, ref forge.RepoRef, workflowFile, branch string) (string, error) {
	if workflowFile == "" {
		return "", nil
	}

	url := fmt.Sprintf("%s/repos/%s/%s/actions/workflows/%s/runs?branch=%s&per_page=1",
		c.cfg.BaseURL, ref.Owner, ref.Repo, workflowFile, branch)

	var resp apiWorkflowRunsResponse
	if err := c.getJSON(ctx, url, &resp); err != nil {
		// Best-effort: swallow the error, caller falls back to the CI page.
		return "", nil
	}
	if len(resp.WorkflowRuns) == 0 {
		return "", nil
	}
	return resp.WorkflowRuns[0].HTMLURL, nil
}

// --------------------------------------------------------------------------
// internal HTTP helpers
// --------------------------------------------------------------------------

// getJSON fetches url with a bearer token, retrying on 429/5xx/network errors.
// Returns immediately (no retry) on 4xx via nonRetryableError.
func (c *Client) getJSON(ctx context.Context, url string, out any) error {
	backoff := c.cfg.InitialBackoff
	var lastErr error

	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}

		err := c.doJSON(ctx, url, out)
		if err == nil {
			return nil
		}

		// Don't retry 4xx.
		var nre *nonRetryableError
		if errors.As(err, &nre) {
			return err
		}

		lastErr = err
		// Continue retrying on network errors or 429/5xx.
	}

	return fmt.Errorf("github: max retries exceeded for %s: %w", url, lastErr)
}

// doJSON performs a single GET request and decodes the JSON body into out.
func (c *Client) doJSON(ctx context.Context, url string, out any) error {
	tok, err := c.cfg.TokenSource.Token(ctx)
	if err != nil {
		return fmt.Errorf("github: get token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Network error — retryable.
		return fmt.Errorf("github: request %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("github: read response body: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		// 429: respect Retry-After if provided, then signal retry.
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(secs) * time.Second):
				}
			}
		}
		return fmt.Errorf("github: rate limited (429)")

	case resp.StatusCode >= 500:
		// 5xx — retryable.
		return fmt.Errorf("github: server error %d: %s", resp.StatusCode, body)

	case resp.StatusCode >= 400:
		// 4xx — not retryable.
		return &nonRetryableError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("github: decode response: %w", err)
	}
	return nil
}

// --------------------------------------------------------------------------
// GitHub API response types
// --------------------------------------------------------------------------

type apiCompareResponse struct {
	AheadBy int         `json:"ahead_by"`
	Status  string      `json:"status"`
	Commits []apiCommit `json:"commits"`
}

type apiCommit struct {
	SHA     string      `json:"sha"`
	Commit  apiGitObj   `json:"commit"`
	HTMLURL string      `json:"html_url"`
	Parents []apiParent `json:"parents"`
}

type apiGitObj struct {
	Message string    `json:"message"`
	Author  apiAuthor `json:"author"`
}

type apiAuthor struct {
	Name string    `json:"name"`
	Date time.Time `json:"date"`
}

type apiParent struct {
	SHA string `json:"sha"`
}

type apiWorkflowRunsResponse struct {
	WorkflowRuns []apiWorkflowRun `json:"workflow_runs"`
}

type apiWorkflowRun struct {
	HTMLURL string `json:"html_url"`
}
