package forge

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseSourceURL parses an OCI org.opencontainers.image.source label value into a
// RepoRef. Supports:
//   - https://host/owner/repo[.git]
//   - git@host:owner/repo[.git]  (scp-style SSH)
func ParseSourceURL(raw string) (RepoRef, error) {
	if raw == "" {
		return RepoRef{}, fmt.Errorf("forge: empty source URL")
	}

	// scp-style ssh: git@host:owner/repo[.git]
	if strings.HasPrefix(raw, "git@") {
		return parseScpURL(raw)
	}

	return parseHTTPSURL(raw)
}

func parseScpURL(raw string) (RepoRef, error) {
	// Format: git@<host>:<owner>/<repo>[.git]
	withoutPrefix := strings.TrimPrefix(raw, "git@")
	colonIdx := strings.Index(withoutPrefix, ":")
	if colonIdx < 0 {
		return RepoRef{}, fmt.Errorf("forge: malformed scp URL (no colon): %q", raw)
	}
	host := withoutPrefix[:colonIdx]
	path := withoutPrefix[colonIdx+1:]
	owner, repo, err := splitOwnerRepo(path)
	if err != nil {
		return RepoRef{}, fmt.Errorf("forge: %w in %q", err, raw)
	}
	return RepoRef{Host: host, Owner: owner, Repo: repo}, nil
}

func parseHTTPSURL(raw string) (RepoRef, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return RepoRef{}, fmt.Errorf("forge: invalid URL %q: %w", raw, err)
	}
	host := u.Hostname()
	// Strip leading slash from path
	path := strings.TrimPrefix(u.Path, "/")
	owner, repo, err := splitOwnerRepo(path)
	if err != nil {
		return RepoRef{}, fmt.Errorf("forge: %w in %q", err, raw)
	}
	return RepoRef{Host: host, Owner: owner, Repo: repo}, nil
}

// splitOwnerRepo splits "owner/repo[.git]" into (owner, repo).
// Returns an error if the path doesn't have exactly two non-empty segments.
func splitOwnerRepo(path string) (owner, repo string, err error) {
	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected owner/repo path, got %q", path)
	}
	return parts[0], parts[1], nil
}
