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
	"fmt"
	"net/http"
)

const defaultAPIKeyHeader = "X-API-Key"

// APIKeyAuthenticator validates requests using a configurable header and a single secret key.
type APIKeyAuthenticator struct {
	headerName string
	key        string
}

// NewAPIKeyAuthenticator creates an APIKeyAuthenticator.
// headerName is the HTTP header to check (falls back to "X-API-Key" when empty).
// key is the expected secret value.
func NewAPIKeyAuthenticator(headerName, key string) *APIKeyAuthenticator {
	if headerName == "" {
		headerName = defaultAPIKeyHeader
	}
	return &APIKeyAuthenticator{headerName: headerName, key: key}
}

// Authenticate checks the configured header against the expected key.
func (a *APIKeyAuthenticator) Authenticate(_ context.Context, headers http.Header) error {
	provided := headers.Get(a.headerName)
	if provided == "" {
		return fmt.Errorf("apikey: %w", ErrUnauthenticated)
	}

	if subtle.ConstantTimeCompare([]byte(provided), []byte(a.key)) != 1 {
		return fmt.Errorf("apikey: %w", ErrInvalidCredentials)
	}

	return nil
}
