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

const apiKeyHeader = "X-API-Key"

// APIKeyAuthenticator validates requests using the X-API-Key header.
type APIKeyAuthenticator struct {
	keys []APIKeyEntry
}

// NewAPIKeyAuthenticator creates an APIKeyAuthenticator from config.
func NewAPIKeyAuthenticator(cfg APIKeyConfig) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{keys: cfg.Keys}
}

// Authenticate checks the X-API-Key header against configured keys.
func (a *APIKeyAuthenticator) Authenticate(_ context.Context, headers http.Header) error {
	provided := headers.Get(apiKeyHeader)
	if provided == "" {
		return fmt.Errorf("apikey: %w", ErrUnauthenticated)
	}

	for _, k := range a.keys {
		if subtle.ConstantTimeCompare([]byte(provided), []byte(k.Key)) == 1 {
			return nil
		}
	}

	return fmt.Errorf("apikey: %w", ErrInvalidCredentials)
}
