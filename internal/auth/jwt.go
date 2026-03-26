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

package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/golgoth31/sreportal/internal/config"
)

// JWTAuthenticator validates JWT Bearer tokens against multiple issuers.
type JWTAuthenticator struct {
	issuers []issuerProvider
	cancels []context.CancelFunc
}

type issuerProvider struct {
	cfg  config.JWTIssuerConfig
	jwks keyfunc.Keyfunc
}

// NewJWTAuthenticator creates a JWTAuthenticator that fetches JWKS from each issuer.
// The parent context controls the lifetime of background JWKS refresh goroutines.
func NewJWTAuthenticator(ctx context.Context, cfg config.JWTAuthConfig) (*JWTAuthenticator, error) {
	providers := make([]issuerProvider, 0, len(cfg.Issuers))
	cancels := make([]context.CancelFunc, 0, len(cfg.Issuers))
	for _, iss := range cfg.Issuers {
		issCtx, cancel := context.WithCancel(ctx)
		jwks, err := keyfunc.NewDefaultCtx(issCtx, []string{iss.JWKSURL})
		if err != nil {
			cancel()
			return nil, fmt.Errorf("init JWKS for issuer %q: %w", iss.Name, err)
		}
		providers = append(providers, issuerProvider{cfg: iss, jwks: jwks})
		cancels = append(cancels, cancel)
	}
	return &JWTAuthenticator{issuers: providers, cancels: cancels}, nil
}

// NewJWTAuthenticatorWithKeyfunc creates a JWTAuthenticator with a pre-built keyfunc
// (useful for testing).
func NewJWTAuthenticatorWithKeyfunc(cfg config.JWTAuthConfig, kf keyfunc.Keyfunc) *JWTAuthenticator {
	providers := make([]issuerProvider, 0, len(cfg.Issuers))
	for _, iss := range cfg.Issuers {
		providers = append(providers, issuerProvider{cfg: iss, jwks: kf})
	}
	return &JWTAuthenticator{issuers: providers}
}

// Authenticate extracts a Bearer token and validates it against configured issuers.
func (a *JWTAuthenticator) Authenticate(_ context.Context, headers http.Header) error {
	tokenStr, err := extractBearerToken(headers)
	if err != nil {
		return err
	}

	var lastErr error
	for _, iss := range a.issuers {
		lastErr = a.validateToken(tokenStr, iss)
		if lastErr == nil {
			return nil
		}
	}

	return lastErr
}

// Close stops background JWKS refresh goroutines.
func (a *JWTAuthenticator) Close() {
	for _, cancel := range a.cancels {
		cancel()
	}
}

func extractBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("jwt: %w", ErrUnauthenticated)
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("jwt: %w: not a Bearer scheme", ErrUnauthenticated)
	}
	return authHeader[len("Bearer "):], nil
}

func (a *JWTAuthenticator) validateToken(tokenStr string, iss issuerProvider) error {
	parserOpts := []jwt.ParserOption{
		jwt.WithIssuer(iss.cfg.IssuerURL),
		jwt.WithExpirationRequired(),
	}
	if iss.cfg.Audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(iss.cfg.Audience))
	}

	token, err := jwt.Parse(tokenStr, iss.jwks.KeyfuncCtx(context.Background()), parserOpts...)
	if err != nil {
		return fmt.Errorf("jwt: %w: %w", ErrInvalidToken, err)
	}

	if !token.Valid {
		return fmt.Errorf("jwt: %w", ErrInvalidToken)
	}

	if err := a.validateClaims(token, iss.cfg); err != nil {
		return err
	}

	return nil
}

func (a *JWTAuthenticator) validateClaims(token *jwt.Token, cfg config.JWTIssuerConfig) error {
	if len(cfg.RequiredClaims) == 0 {
		return nil
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("jwt: %w: unexpected claims type", ErrInvalidToken)
	}

	for key, required := range cfg.RequiredClaims {
		val, exists := claims[key]
		if !exists {
			return fmt.Errorf("jwt: %w: missing required claim %q", ErrInvalidToken, key)
		}

		strVal, ok := val.(string)
		if !ok {
			return fmt.Errorf("jwt: %w: claim %q is not a string", ErrInvalidToken, key)
		}

		// For space-separated claims (like "scope"), check if the required value
		// is contained within the space-separated list.
		if !containsValue(strVal, required) {
			return fmt.Errorf("jwt: %w: claim %q does not contain %q", ErrInvalidToken, key, required)
		}
	}

	return nil
}

// containsValue checks whether required is present in a space-separated value string.
func containsValue(value, required string) bool {
	for part := range strings.SplitSeq(value, " ") {
		if part == required {
			return true
		}
	}
	return false
}
