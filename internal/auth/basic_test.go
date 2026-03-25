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
	"encoding/base64"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/auth"
)

func basicHeader(username, password string) http.Header {
	h := http.Header{}
	creds := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	h.Set("Authorization", "Basic "+creds)
	return h
}

func TestBasicAuth_ValidCredentials(t *testing.T) {
	a := auth.NewBasicAuthenticator(auth.BasicAuthConfig{
		Username: "admin", Password: "secret",
	})
	err := a.Authenticate(context.Background(), basicHeader("admin", "secret"))
	require.NoError(t, err)
}

func TestBasicAuth_InvalidPassword(t *testing.T) {
	a := auth.NewBasicAuthenticator(auth.BasicAuthConfig{
		Username: "admin", Password: "secret",
	})
	err := a.Authenticate(context.Background(), basicHeader("admin", "wrong"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidCredentials))
}

func TestBasicAuth_InvalidUsername(t *testing.T) {
	a := auth.NewBasicAuthenticator(auth.BasicAuthConfig{
		Username: "admin", Password: "secret",
	})
	err := a.Authenticate(context.Background(), basicHeader("wrong", "secret"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidCredentials))
}

func TestBasicAuth_MissingHeader(t *testing.T) {
	a := auth.NewBasicAuthenticator(auth.BasicAuthConfig{
		Username: "admin", Password: "secret",
	})
	err := a.Authenticate(context.Background(), http.Header{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrUnauthenticated))
}

func TestBasicAuth_WrongScheme(t *testing.T) {
	a := auth.NewBasicAuthenticator(auth.BasicAuthConfig{
		Username: "admin", Password: "secret",
	})
	h := http.Header{}
	h.Set("Authorization", "Bearer some-token")
	err := a.Authenticate(context.Background(), h)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrUnauthenticated))
}

func TestBasicAuth_MalformedBase64(t *testing.T) {
	a := auth.NewBasicAuthenticator(auth.BasicAuthConfig{
		Username: "admin", Password: "secret",
	})
	h := http.Header{}
	h.Set("Authorization", "Basic !!!invalid!!!")
	err := a.Authenticate(context.Background(), h)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidCredentials))
}

func TestBasicAuth_MissingColon(t *testing.T) {
	a := auth.NewBasicAuthenticator(auth.BasicAuthConfig{
		Username: "admin", Password: "secret",
	})
	h := http.Header{}
	h.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("nocolon")))
	err := a.Authenticate(context.Background(), h)
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrInvalidCredentials))
}
