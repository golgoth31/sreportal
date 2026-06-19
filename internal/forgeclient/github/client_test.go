package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	githubclient "github.com/golgoth31/sreportal/internal/forgeclient/github"
	"github.com/golgoth31/sreportal/internal/domain/forge"
)

// staticTokenSource always returns the same token (no network).
type staticTokenSource struct{ tok string }

func (s *staticTokenSource) Token(_ context.Context) (string, error) { return s.tok, nil }

func TestDefaultBranch_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/acme/myapp", r.URL.Path)
		assert.Equal(t, "Bearer test-pat", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"default_branch": "main",
		})
	}))
	defer srv.Close()

	c := githubclient.NewClient(githubclient.Config{
		TokenSource: &staticTokenSource{tok: "test-pat"},
		BaseURL:     srv.URL,
	})
	branch, err := c.DefaultBranch(context.Background(), forge.RepoRef{
		Host:  "github.com",
		Owner: "acme",
		Repo:  "myapp",
	})
	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestCompare_ParsesCommitsAndMergeFlag(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/acme/myapp/compare/abc...HEAD", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ahead_by": 2,
			"commits": []map[string]any{
				{
					"sha": "aaa111",
					"commit": map[string]any{
						"message": "feat: add feature",
						"author":  map[string]any{"name": "Alice", "date": now.Format(time.RFC3339)},
					},
					"html_url": "https://github.com/acme/myapp/commit/aaa111",
					"parents":  []map[string]any{{"sha": "parent1"}},
				},
				{
					"sha": "bbb222",
					"commit": map[string]any{
						"message": "Merge PR #42",
						"author":  map[string]any{"name": "Bot", "date": now.Format(time.RFC3339)},
					},
					"html_url": "https://github.com/acme/myapp/commit/bbb222",
					// two parents => merge commit
					"parents": []map[string]any{{"sha": "parent1"}, {"sha": "parent2"}},
				},
			},
		})
	}))
	defer srv.Close()

	c := githubclient.NewClient(githubclient.Config{
		TokenSource: &staticTokenSource{tok: "test-pat"},
		BaseURL:     srv.URL,
	})
	result, err := c.Compare(context.Background(), forge.RepoRef{
		Host:  "github.com",
		Owner: "acme",
		Repo:  "myapp",
	}, "abc", "HEAD")
	require.NoError(t, err)
	assert.Equal(t, 2, result.AheadBy)
	require.Len(t, result.Commits, 2)
	assert.Equal(t, "aaa111", result.Commits[0].SHA)
	assert.False(t, result.Commits[0].Merge)
	assert.Equal(t, "bbb222", result.Commits[1].SHA)
	assert.True(t, result.Commits[1].Merge, "commit with 2 parents should be flagged as merge")
}

func TestCompare_RetriesOn429ThenParses(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n < 3 {
			// First two calls: 429 with Retry-After: 0
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ahead_by": 1,
			"commits": []map[string]any{
				{
					"sha":      "ccc333",
					"commit":   map[string]any{"message": "fix", "author": map[string]any{"name": "Bob", "date": time.Now().Format(time.RFC3339)}},
					"html_url": "https://github.com/acme/myapp/commit/ccc333",
					"parents":  []map[string]any{{"sha": "p"}},
				},
			},
		})
	}))
	defer srv.Close()

	c := githubclient.NewClient(githubclient.Config{
		TokenSource:     &staticTokenSource{tok: "test-pat"},
		BaseURL:         srv.URL,
		MaxRetries:      5,
		InitialBackoff:  time.Millisecond,
	})
	result, err := c.Compare(context.Background(), forge.RepoRef{Owner: "acme", Repo: "myapp"}, "base", "head")
	require.NoError(t, err)
	assert.Equal(t, 1, result.AheadBy)
	assert.GreaterOrEqual(t, callCount.Load(), int32(3))
}

func TestCompare_DoesNotRetry4xx(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()

	c := githubclient.NewClient(githubclient.Config{
		TokenSource:    &staticTokenSource{tok: "test-pat"},
		BaseURL:        srv.URL,
		MaxRetries:     5,
		InitialBackoff: time.Millisecond,
	})
	_, err := c.Compare(context.Background(), forge.RepoRef{Owner: "acme", Repo: "myapp"}, "base", "head")
	require.Error(t, err)
	assert.Equal(t, int32(1), callCount.Load(), "4xx should not be retried")
}

func TestCompare_Truncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ahead_by": 100,
			"commits":  []map[string]any{},
			"status":   "diverged",
		})
	}))
	defer srv.Close()

	c := githubclient.NewClient(githubclient.Config{
		TokenSource: &staticTokenSource{tok: "test-pat"},
		BaseURL:     srv.URL,
	})
	result, err := c.Compare(context.Background(), forge.RepoRef{Owner: "acme", Repo: "myapp"}, "base", "head")
	require.NoError(t, err)
	// When status == "diverged", commits list may be empty but ahead_by > 0; not truncated per se.
	assert.Equal(t, 100, result.AheadBy)
}
