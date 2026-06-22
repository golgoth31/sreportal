// Package github provides a forge.Client implementation backed by the GitHub REST API.
package github

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TokenSource obtains a bearer token for authenticating GitHub API requests.
// Implementations must be safe for concurrent use.
type TokenSource interface {
	// Token returns a valid bearer token. It may refresh internally.
	Token(ctx context.Context) (string, error)
}

// --------------------------------------------------------------------------
// PATTokenSource — fine-grained or classic Personal Access Token
// --------------------------------------------------------------------------

// PATTokenSource is a static token source backed by a pre-configured PAT.
type PATTokenSource struct {
	token string
}

// NewPATTokenSource returns a TokenSource that always returns pat.
func NewPATTokenSource(pat string) *PATTokenSource {
	return &PATTokenSource{token: pat}
}

// Token implements TokenSource.
func (p *PATTokenSource) Token(_ context.Context) (string, error) {
	return p.token, nil
}

// --------------------------------------------------------------------------
// AppTokenSource — GitHub App installation token (RS256 JWT → installation token)
// --------------------------------------------------------------------------

// AppTokenSourceConfig configures an AppTokenSource.
type AppTokenSourceConfig struct {
	AppID          int64
	InstallationID int64
	// PrivateKeyPEM is the raw PEM content of the GitHub App private key
	// (PKCS#1 "RSA PRIVATE KEY" or PKCS#8 "PRIVATE KEY").
	PrivateKeyPEM string
	// BaseURL overrides the GitHub API base (default: "https://api.github.com").
	// Useful for GHES and tests.
	BaseURL string
	// HTTPClient overrides the HTTP client used for installation token requests.
	// When nil, defaults to &http.Client{Timeout: 15s}.
	HTTPClient *http.Client
}

// AppTokenSource mints a GitHub App JWT, exchanges it for an installation access
// token, and caches the token until ~1 min before expiry. All methods are safe
// for concurrent use.
type AppTokenSource struct {
	cfg        AppTokenSourceConfig
	privateKey *rsa.PrivateKey
	httpClient *http.Client
	baseURL    string

	mu        sync.Mutex
	cached    string
	expiresAt time.Time
}

// NewAppTokenSource parses the PEM key and returns an AppTokenSource ready to use.
// Returns an error immediately if the private key PEM cannot be parsed (fail-fast at
// operator startup rather than deferring to the first Token() call).
func NewAppTokenSource(cfg AppTokenSourceConfig) (*AppTokenSource, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	key, err := parseRSAKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("github app: parse private key: %w", err)
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &AppTokenSource{
		cfg:        cfg,
		privateKey: key,
		httpClient: httpClient,
		baseURL:    baseURL,
	}, nil
}

// Token returns a valid installation access token, refreshing if needed.
func (a *AppTokenSource) Token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Refresh ~1 min before expiry.
	if a.cached != "" && time.Now().Before(a.expiresAt.Add(-time.Minute)) {
		return a.cached, nil
	}

	tok, exp, err := a.mintInstallationToken(ctx)
	if err != nil {
		return "", err
	}
	a.cached = tok
	a.expiresAt = exp
	return tok, nil
}

// mintInstallationToken signs an App JWT and exchanges it for an installation token.
func (a *AppTokenSource) mintInstallationToken(ctx context.Context) (string, time.Time, error) {
	jwt, err := a.signAppJWT()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("github app: sign JWT: %w", err)
	}

	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", a.baseURL, a.cfg.InstallationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("github app: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("github app: installation token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("github app: read response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("github app: installation token: status %d: %s", resp.StatusCode, body)
	}

	var payload struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", time.Time{}, fmt.Errorf("github app: decode token response: %w", err)
	}
	exp, err := time.Parse(time.RFC3339, payload.ExpiresAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("github app: parse expires_at %q: %w", payload.ExpiresAt, err)
	}
	return payload.Token, exp, nil
}

// signAppJWT produces a signed RS256 JWT for the GitHub App.
// Uses stdlib only: no external JWT library.
func (a *AppTokenSource) signAppJWT() (string, error) {
	now := time.Now()
	// iat slightly in the past to account for clock skew
	iat := now.Add(-30 * time.Second).Unix()
	// exp max 10 minutes from now
	exp := now.Add(9 * time.Minute).Unix()

	header := base64url(mustJSON(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	}))
	claims := base64url(mustJSON(map[string]any{
		"iat": iat,
		"exp": exp,
		"iss": fmt.Sprintf("%d", a.cfg.AppID),
	}))

	signingInput := header + "." + claims
	digest := sha256.Sum256([]byte(signingInput))

	sig, err := rsa.SignPKCS1v15(rand.Reader, a.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("RSA sign: %w", err)
	}
	return signingInput + "." + base64url(sig), nil
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

// base64url encodes b using base64 URL encoding without padding.
func base64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// mustJSON marshals v to JSON; panics on error (only used with static map literals).
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustJSON: %v", err))
	}
	return b
}

// parseRSAKey parses a PEM-encoded RSA private key (PKCS#1 or PKCS#8).
func parseRSAKey(pemData string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemData)))
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is not RSA")
		}
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %q", block.Type)
	}
}
