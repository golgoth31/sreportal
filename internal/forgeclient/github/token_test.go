package github_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	githubclient "github.com/golgoth31/sreportal/internal/forgeclient/github"
)

// generateTestKey generates a fresh RSA-2048 private key for test use.
func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

// encodePKCS1PEM encodes an RSA key to PKCS#1 PEM (traditional format).
func encodePKCS1PEM(key *rsa.PrivateKey) string {
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
}

// encodePKCS8PEM encodes an RSA key to PKCS#8 PEM format.
func encodePKCS8PEM(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	}))
}

func TestPATTokenSource_ReturnsToken(t *testing.T) {
	src := githubclient.NewPATTokenSource("ghp_testtoken123")
	tok, err := src.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghp_testtoken123", tok)
}

func TestNewAppTokenSource_FailsFastOnInvalidPEM(t *testing.T) {
	_, err := githubclient.NewAppTokenSource(githubclient.AppTokenSourceConfig{
		AppID:          42,
		InstallationID: 1234,
		PrivateKeyPEM:  "not-a-valid-pem",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse private key")
}

func TestAppTokenSource_MintsAndCaches(t *testing.T) {
	key := generateTestKey(t)

	var mintCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the Authorization header carries a Bearer JWT
		auth := r.Header.Get("Authorization")
		assert.True(t, len(auth) > 7 && auth[:7] == "Bearer ", "expected Bearer token, got: %s", auth)

		mintCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		// Return an installation token valid for 1 hour
		resp := map[string]any{
			keyToken:   "ghs_installation_token",
			keyExpires: time.Now().Add(time.Hour).Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	src, err := githubclient.NewAppTokenSource(githubclient.AppTokenSourceConfig{
		AppID:          42,
		InstallationID: 1234,
		PrivateKeyPEM:  encodePKCS1PEM(key),
		BaseURL:        srv.URL,
	})
	require.NoError(t, err)

	// First call — should mint
	tok1, err := src.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghs_installation_token", tok1)

	// Second call — should use cached token (no second mint)
	tok2, err := src.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghs_installation_token", tok2)

	assert.Equal(t, int32(1), mintCount.Load(), "expected exactly one installation token mint")
}

func TestAppTokenSource_RefreshesExpiredToken(t *testing.T) {
	key := generateTestKey(t)

	var mintCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := mintCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		var tok string
		var exp time.Time
		if n == 1 {
			// First token: already expired (so next call will refresh)
			tok = "ghs_expired_token"
			exp = time.Now().Add(-time.Minute)
		} else {
			tok = "ghs_fresh_token"
			exp = time.Now().Add(time.Hour)
		}
		resp := map[string]any{
			keyToken:   tok,
			keyExpires: exp.Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	src, err := githubclient.NewAppTokenSource(githubclient.AppTokenSourceConfig{
		AppID:          42,
		InstallationID: 1234,
		PrivateKeyPEM:  encodePKCS1PEM(key),
		BaseURL:        srv.URL,
	})
	require.NoError(t, err)

	// First call — mints expired token
	tok1, err := src.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghs_expired_token", tok1)

	// Second call — detects expiry, mints again
	tok2, err := src.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghs_fresh_token", tok2)

	assert.Equal(t, int32(2), mintCount.Load())
}

func TestAppTokenSource_PKCS8Key(t *testing.T) {
	key := generateTestKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			keyToken:   "ghs_pkcs8_token",
			keyExpires: time.Now().Add(time.Hour).Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	src, err := githubclient.NewAppTokenSource(githubclient.AppTokenSourceConfig{
		AppID:          42,
		InstallationID: 1234,
		PrivateKeyPEM:  encodePKCS8PEM(t, key),
		BaseURL:        srv.URL,
	})
	require.NoError(t, err)

	tok, err := src.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghs_pkcs8_token", tok)
}
