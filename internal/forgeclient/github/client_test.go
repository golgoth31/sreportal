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

	"github.com/golgoth31/sreportal/internal/domain/forge"
	githubclient "github.com/golgoth31/sreportal/internal/forgeclient/github"
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
		TokenSource: &staticTokenSource{tok: testPAT},
		BaseURL:     srv.URL,
	})
	branch, err := c.DefaultBranch(context.Background(), forge.RepoRef{
		Host:  "github.com",
		Owner: testOwner,
		Repo:  testRepo,
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
			keyAheadBy: 2,
			keyCommits: []map[string]any{
				{
					keySHA: "aaa111",
					keyCommit: map[string]any{
						keyMessage: "feat: add feature",
						keyAuthor:  map[string]any{keyName: "Alice", keyDate: now.Format(time.RFC3339)},
					},
					keyHTMLURL: "https://github.com/acme/myapp/commit/aaa111",
					keyParents: []map[string]any{{keySHA: "parent1"}},
				},
				{
					keySHA: "bbb222",
					keyCommit: map[string]any{
						keyMessage: "Merge PR #42",
						keyAuthor:  map[string]any{keyName: "Bot", keyDate: now.Format(time.RFC3339)},
					},
					keyHTMLURL: "https://github.com/acme/myapp/commit/bbb222",
					// two parents => merge commit
					keyParents: []map[string]any{{keySHA: "parent1"}, {keySHA: "parent2"}},
				},
			},
		})
	}))
	defer srv.Close()

	c := githubclient.NewClient(githubclient.Config{
		TokenSource: &staticTokenSource{tok: testPAT},
		BaseURL:     srv.URL,
	})
	result, err := c.Compare(context.Background(), forge.RepoRef{
		Host:  "github.com",
		Owner: testOwner,
		Repo:  testRepo,
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
			keyAheadBy: 1,
			keyCommits: []map[string]any{
				{
					keySHA:     "ccc333",
					keyCommit:  map[string]any{keyMessage: "fix", keyAuthor: map[string]any{keyName: "Bob", keyDate: time.Now().Format(time.RFC3339)}},
					keyHTMLURL: "https://github.com/acme/myapp/commit/ccc333",
					keyParents: []map[string]any{{keySHA: "p"}},
				},
			},
		})
	}))
	defer srv.Close()

	c := githubclient.NewClient(githubclient.Config{
		TokenSource:    &staticTokenSource{tok: testPAT},
		BaseURL:        srv.URL,
		MaxRetries:     5,
		InitialBackoff: time.Millisecond,
	})
	result, err := c.Compare(context.Background(), forge.RepoRef{Owner: testOwner, Repo: testRepo}, "base", "head")
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
		TokenSource:    &staticTokenSource{tok: testPAT},
		BaseURL:        srv.URL,
		MaxRetries:     5,
		InitialBackoff: time.Millisecond,
	})
	_, err := c.Compare(context.Background(), forge.RepoRef{Owner: testOwner, Repo: testRepo}, "base", "head")
	require.Error(t, err)
	assert.Equal(t, int32(1), callCount.Load(), "4xx should not be retried")
}

func TestCompare_Truncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			keyAheadBy: 100,
			keyCommits: []map[string]any{},
			"status":   "diverged",
		})
	}))
	defer srv.Close()

	c := githubclient.NewClient(githubclient.Config{
		TokenSource: &staticTokenSource{tok: testPAT},
		BaseURL:     srv.URL,
	})
	result, err := c.Compare(context.Background(), forge.RepoRef{Owner: testOwner, Repo: testRepo}, "base", "head")
	require.NoError(t, err)
	// When status == "diverged", commits list may be empty but ahead_by > 0; not truncated per se.
	assert.Equal(t, 100, result.AheadBy)
}

func TestLatestWorkflowRun_ReturnsHTMLURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/acme/myapp/actions/workflows/deploy.yml/runs", r.URL.Path)
		assert.Equal(t, "mybranch", r.URL.Query().Get("branch"))
		assert.Equal(t, "1", r.URL.Query().Get("per_page"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"workflow_runs": []map[string]any{
				{keyHTMLURL: "https://github.com/acme/myapp/actions/runs/12345"},
			},
		})
	}))
	defer srv.Close()

	c := githubclient.NewClient(githubclient.Config{
		TokenSource: &staticTokenSource{tok: testPAT},
		BaseURL:     srv.URL,
	})
	got, err := c.LatestWorkflowRun(context.Background(), forge.RepoRef{Owner: testOwner, Repo: testRepo}, "deploy.yml", "mybranch")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/acme/myapp/actions/runs/12345", got)
}

func TestLatestWorkflowRun_EmptyWorkflowFile(t *testing.T) {
	// No server needed — should return early without any HTTP call.
	c := githubclient.NewClient(githubclient.Config{
		TokenSource: &staticTokenSource{tok: testPAT},
		BaseURL:     "http://127.0.0.1:0", // unreachable; call must not reach it
	})
	got, err := c.LatestWorkflowRun(context.Background(), forge.RepoRef{Owner: testOwner, Repo: testRepo}, "", "main")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestLatestWorkflowRun_ServerErrorReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()

	c := githubclient.NewClient(githubclient.Config{
		TokenSource:    &staticTokenSource{tok: testPAT},
		BaseURL:        srv.URL,
		MaxRetries:     0, // no retries, fail fast
		InitialBackoff: time.Millisecond,
	})
	got, err := c.LatestWorkflowRun(context.Background(), forge.RepoRef{Owner: testOwner, Repo: testRepo}, "deploy.yml", "main")
	require.NoError(t, err, "LatestWorkflowRun must swallow errors (best-effort)")
	assert.Equal(t, "", got)
}

func TestLatestWorkflowRun_EmptyRunsReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"workflow_runs": []map[string]any{},
		})
	}))
	defer srv.Close()

	c := githubclient.NewClient(githubclient.Config{
		TokenSource: &staticTokenSource{tok: testPAT},
		BaseURL:     srv.URL,
	})
	got, err := c.LatestWorkflowRun(context.Background(), forge.RepoRef{Owner: testOwner, Repo: testRepo}, "ci.yml", "main")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}
