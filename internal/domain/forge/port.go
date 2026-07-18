// Package forge defines the forge-agnostic port (interface + value types) for
// communicating with a git forge (GitHub, GHES, …). Infrastructure packages
// implement the Client interface; domain packages depend only on this package.
package forge

import (
	"context"
	"time"
)

// RepoRef identifies a git repository on a given host.
type RepoRef struct {
	Host  string // e.g. "github.com", "ghe.example.com"
	Owner string // org or user
	Repo  string // repository name (no ".git" suffix)
}

// Commit is a single git commit value.
type Commit struct {
	SHA     string
	Message string
	Author  string
	Date    time.Time
	URL     string
	Merge   bool // true when len(parents) > 1
}

// CompareResult is the response to a Compare call.
type CompareResult struct {
	AheadBy   int
	Commits   []Commit // oldest-first, merge commits included (flagged by Merge=true)
	Truncated bool     // true when the forge capped the list
}

// Client is the forge-agnostic port. Infrastructure packages implement this.
// All methods are safe to call concurrently.
type Client interface {
	// DefaultBranch returns the repository's default branch name.
	DefaultBranch(ctx context.Context, ref RepoRef) (string, error)

	// Compare returns commits reachable from head but not from base (head is
	// ahead-of base). The result may be truncated by the forge.
	Compare(ctx context.Context, ref RepoRef, base, head string) (CompareResult, error)

	// LatestWorkflowRun returns the URL of the most recent run of workflowFile on branch.
	// Best-effort: returns ("", nil) when not resolvable (caller falls back to the CI page).
	LatestWorkflowRun(ctx context.Context, ref RepoRef, workflowFile, branch string) (string, error)
}
