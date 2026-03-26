/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/auth"
	"github.com/golgoth31/sreportal/internal/config"
)

// staticKeyfunc implements keyfunc.Keyfunc for testing with a known RSA key.
type staticKeyfunc struct {
	key *rsa.PublicKey
}

func (s *staticKeyfunc) Keyfunc(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, errors.New("unexpected signing method")
	}
	return s.key, nil
}

func (s *staticKeyfunc) KeyfuncCtx(_ context.Context) jwt.Keyfunc {
	return s.Keyfunc
}

func (s *staticKeyfunc) Storage() jwkset.Storage {
	return nil
}

func (s *staticKeyfunc) VerificationKeySet(_ context.Context) (jwt.VerificationKeySet, error) {
	return jwt.VerificationKeySet{Keys: []jwt.VerificationKey{s.key}}, nil
}

func mustGenerateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func signToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenStr, err := token.SignedString(key)
	require.NoError(t, err)
	return tokenStr
}

func bearerHeader(token string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+token)
	return h
}

func newTestJWTAuth(t *testing.T, key *rsa.PrivateKey, cfg config.JWTAuthConfig) *auth.JWTAuthenticator {
	t.Helper()
	kf := &staticKeyfunc{key: &key.PublicKey}
	return auth.NewJWTAuthenticatorWithKeyfunc(cfg, kf)
}

func TestJWT_ValidToken(t *testing.T) {
	key := mustGenerateKey(t)
	cfg := config.JWTAuthConfig{
		Issuers: []config.JWTIssuerConfig{{
			Name:      "test",
			IssuerURL: "https://issuer.example.com/",
			Audience:  "sreportal",
		}},
	}
	a := newTestJWTAuth(t, key, cfg)

	token := signToken(t, key, jwt.MapClaims{
		"iss": "https://issuer.example.com/",
		"aud": "sreportal",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	err := a.Authenticate(context.Background(), bearerHeader(token))
	require.NoError(t, err)
}

func TestJWT_ExpiredToken(t *testing.T) {
	key := mustGenerateKey(t)
	cfg := config.JWTAuthConfig{
		Issuers: []config.JWTIssuerConfig{{
			Name:      "test",
			IssuerURL: "https://issuer.example.com/",
		}},
	}
	a := newTestJWTAuth(t, key, cfg)

	token := signToken(t, key, jwt.MapClaims{
		"iss": "https://issuer.example.com/",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})

	err := a.Authenticate(context.Background(), bearerHeader(token))
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidToken))
}

func TestJWT_WrongIssuer(t *testing.T) {
	key := mustGenerateKey(t)
	cfg := config.JWTAuthConfig{
		Issuers: []config.JWTIssuerConfig{{
			Name:      "test",
			IssuerURL: "https://issuer.example.com/",
		}},
	}
	a := newTestJWTAuth(t, key, cfg)

	token := signToken(t, key, jwt.MapClaims{
		"iss": "https://wrong-issuer.example.com/",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	err := a.Authenticate(context.Background(), bearerHeader(token))
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidToken))
}

func TestJWT_WrongAudience(t *testing.T) {
	key := mustGenerateKey(t)
	cfg := config.JWTAuthConfig{
		Issuers: []config.JWTIssuerConfig{{
			Name:      "test",
			IssuerURL: "https://issuer.example.com/",
			Audience:  "sreportal",
		}},
	}
	a := newTestJWTAuth(t, key, cfg)

	token := signToken(t, key, jwt.MapClaims{
		"iss": "https://issuer.example.com/",
		"aud": "other-api",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	err := a.Authenticate(context.Background(), bearerHeader(token))
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidToken))
}

func TestJWT_MissingRequiredClaim(t *testing.T) {
	key := mustGenerateKey(t)
	cfg := config.JWTAuthConfig{
		Issuers: []config.JWTIssuerConfig{{
			Name:           "test",
			IssuerURL:      "https://issuer.example.com/",
			RequiredClaims: map[string]string{"scope": "release:write"},
		}},
	}
	a := newTestJWTAuth(t, key, cfg)

	token := signToken(t, key, jwt.MapClaims{
		"iss": "https://issuer.example.com/",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	err := a.Authenticate(context.Background(), bearerHeader(token))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required claim")
}

func TestJWT_RequiredClaimPresent(t *testing.T) {
	key := mustGenerateKey(t)
	cfg := config.JWTAuthConfig{
		Issuers: []config.JWTIssuerConfig{{
			Name:           "test",
			IssuerURL:      "https://issuer.example.com/",
			RequiredClaims: map[string]string{"scope": "release:write"},
		}},
	}
	a := newTestJWTAuth(t, key, cfg)

	token := signToken(t, key, jwt.MapClaims{
		"iss":   "https://issuer.example.com/",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"scope": "read release:write admin",
	})

	err := a.Authenticate(context.Background(), bearerHeader(token))
	require.NoError(t, err)
}

func TestJWT_RequiredClaimWrongValue(t *testing.T) {
	key := mustGenerateKey(t)
	cfg := config.JWTAuthConfig{
		Issuers: []config.JWTIssuerConfig{{
			Name:           "test",
			IssuerURL:      "https://issuer.example.com/",
			RequiredClaims: map[string]string{"scope": "release:write"},
		}},
	}
	a := newTestJWTAuth(t, key, cfg)

	token := signToken(t, key, jwt.MapClaims{
		"iss":   "https://issuer.example.com/",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"scope": "read admin",
	})

	err := a.Authenticate(context.Background(), bearerHeader(token))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain")
}

func TestJWT_InvalidSignature(t *testing.T) {
	key := mustGenerateKey(t)
	otherKey := mustGenerateKey(t)
	cfg := config.JWTAuthConfig{
		Issuers: []config.JWTIssuerConfig{{
			Name:      "test",
			IssuerURL: "https://issuer.example.com/",
		}},
	}
	a := newTestJWTAuth(t, key, cfg) // keyfunc has key's public key

	// Sign with a different private key
	token := signToken(t, otherKey, jwt.MapClaims{
		"iss": "https://issuer.example.com/",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	err := a.Authenticate(context.Background(), bearerHeader(token))
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidToken))
}

func TestJWT_MissingHeader(t *testing.T) {
	key := mustGenerateKey(t)
	cfg := config.JWTAuthConfig{
		Issuers: []config.JWTIssuerConfig{{
			Name:      "test",
			IssuerURL: "https://issuer.example.com/",
		}},
	}
	a := newTestJWTAuth(t, key, cfg)

	err := a.Authenticate(context.Background(), http.Header{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrUnauthenticated))
}

func TestJWT_NotBearerScheme(t *testing.T) {
	key := mustGenerateKey(t)
	cfg := config.JWTAuthConfig{
		Issuers: []config.JWTIssuerConfig{{
			Name:      "test",
			IssuerURL: "https://issuer.example.com/",
		}},
	}
	a := newTestJWTAuth(t, key, cfg)

	h := http.Header{}
	h.Set("Authorization", "Basic dXNlcjpwYXNz")
	err := a.Authenticate(context.Background(), h)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrUnauthenticated))
}
