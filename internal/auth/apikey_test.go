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
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/auth"
)

func apiKeyHeader(header, key string) http.Header {
	h := http.Header{}
	h.Set(header, key)
	return h
}

func TestAPIKey_ValidKey(t *testing.T) {
	a := auth.NewAPIKeyAuthenticator("", "abc123")
	err := a.Authenticate(context.Background(), apiKeyHeader("X-API-Key", "abc123"))
	require.NoError(t, err)
}

func TestAPIKey_CustomHeader(t *testing.T) {
	a := auth.NewAPIKeyAuthenticator("X-Custom-Auth", "my-key")
	err := a.Authenticate(context.Background(), apiKeyHeader("X-Custom-Auth", "my-key"))
	require.NoError(t, err)
}

func TestAPIKey_InvalidKey(t *testing.T) {
	a := auth.NewAPIKeyAuthenticator("", "abc123")
	err := a.Authenticate(context.Background(), apiKeyHeader("X-API-Key", "wrong"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidCredentials))
}

func TestAPIKey_MissingHeader(t *testing.T) {
	a := auth.NewAPIKeyAuthenticator("", "abc123")
	err := a.Authenticate(context.Background(), http.Header{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrUnauthenticated))
}

func TestAPIKey_DefaultHeader(t *testing.T) {
	a := auth.NewAPIKeyAuthenticator("", "secret")
	err := a.Authenticate(context.Background(), apiKeyHeader("X-API-Key", "secret"))
	require.NoError(t, err)
}
