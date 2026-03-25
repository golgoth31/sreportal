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
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// BasicAuthenticator validates HTTP Basic Authentication credentials.
type BasicAuthenticator struct {
	username string
	password string
}

// NewBasicAuthenticator creates a BasicAuthenticator from config.
func NewBasicAuthenticator(cfg BasicAuthConfig) *BasicAuthenticator {
	return &BasicAuthenticator{
		username: cfg.Username,
		password: cfg.Password,
	}
}

// Authenticate checks the Authorization header for valid Basic credentials.
func (a *BasicAuthenticator) Authenticate(_ context.Context, headers http.Header) error {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("basic: %w", ErrUnauthenticated)
	}

	if !strings.HasPrefix(authHeader, "Basic ") {
		return fmt.Errorf("basic: %w: not a Basic scheme", ErrUnauthenticated)
	}

	decoded, err := base64.StdEncoding.DecodeString(authHeader[len("Basic "):])
	if err != nil {
		return fmt.Errorf("basic: %w: malformed credentials", ErrInvalidCredentials)
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("basic: %w: malformed credentials", ErrInvalidCredentials)
	}

	usernameMatch := subtle.ConstantTimeCompare([]byte(parts[0]), []byte(a.username))
	passwordMatch := subtle.ConstantTimeCompare([]byte(parts[1]), []byte(a.password))
	if usernameMatch&passwordMatch != 1 {
		return fmt.Errorf("basic: %w", ErrInvalidCredentials)
	}

	return nil
}
