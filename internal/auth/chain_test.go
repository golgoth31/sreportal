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

type fakeAuthenticator struct {
	err error
}

func (f *fakeAuthenticator) Authenticate(_ context.Context, _ http.Header) error {
	return f.err
}

func TestChain_EmptyChain_ReturnsNoAuthMethod(t *testing.T) {
	chain := auth.NewChain()
	err := chain.Authenticate(context.Background(), http.Header{})
	require.ErrorIs(t, err, auth.ErrNoAuthMethod)
}

func TestChain_FirstSucceeds(t *testing.T) {
	chain := auth.NewChain(
		&fakeAuthenticator{err: nil},
		&fakeAuthenticator{err: auth.ErrInvalidCredentials},
	)
	err := chain.Authenticate(context.Background(), http.Header{})
	require.NoError(t, err)
}

func TestChain_SecondSucceeds(t *testing.T) {
	chain := auth.NewChain(
		&fakeAuthenticator{err: auth.ErrInvalidCredentials},
		&fakeAuthenticator{err: nil},
	)
	err := chain.Authenticate(context.Background(), http.Header{})
	require.NoError(t, err)
}

func TestChain_AllFail(t *testing.T) {
	chain := auth.NewChain(
		&fakeAuthenticator{err: auth.ErrInvalidCredentials},
		&fakeAuthenticator{err: auth.ErrInvalidToken},
	)
	err := chain.Authenticate(context.Background(), http.Header{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrUnauthenticated))
	assert.Contains(t, err.Error(), "invalid credentials")
	assert.Contains(t, err.Error(), "invalid token")
}
