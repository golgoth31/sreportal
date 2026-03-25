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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/auth"
)

func TestAuthConfig_Enabled(t *testing.T) {
	cases := []struct {
		name string
		cfg  auth.AuthConfig
		want bool
	}{
		{"empty config", auth.AuthConfig{}, false},
		{"basic disabled", auth.AuthConfig{BasicAuth: &auth.BasicAuthConfig{Enabled: false}}, false},
		{"basic enabled", auth.AuthConfig{BasicAuth: &auth.BasicAuthConfig{Enabled: true}}, true},
		{"apikey enabled", auth.AuthConfig{APIKey: &auth.APIKeyConfig{Enabled: true}}, true},
		{"jwt enabled", auth.AuthConfig{JWT: &auth.JWTConfig{Enabled: true}}, true},
		{"multiple enabled", auth.AuthConfig{
			BasicAuth: &auth.BasicAuthConfig{Enabled: true},
			APIKey:    &auth.APIKeyConfig{Enabled: true},
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.cfg.Enabled())
		})
	}
}

func TestAuthConfig_Validate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     auth.AuthConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid basic auth",
			cfg: auth.AuthConfig{BasicAuth: &auth.BasicAuthConfig{
				Enabled: true, Username: "admin", Password: "secret",
			}},
		},
		{
			name: "basic auth missing username",
			cfg: auth.AuthConfig{BasicAuth: &auth.BasicAuthConfig{
				Enabled: true, Password: "secret",
			}},
			wantErr: true,
			errMsg:  "username is required",
		},
		{
			name: "basic auth missing password",
			cfg: auth.AuthConfig{BasicAuth: &auth.BasicAuthConfig{
				Enabled: true, Username: "admin",
			}},
			wantErr: true,
			errMsg:  "password is required",
		},
		{
			name: "basic auth disabled skips validation",
			cfg: auth.AuthConfig{BasicAuth: &auth.BasicAuthConfig{
				Enabled: false,
			}},
		},
		{
			name: "valid apikey",
			cfg: auth.AuthConfig{APIKey: &auth.APIKeyConfig{
				Enabled: true, Keys: []auth.APIKeyEntry{{Name: "ci", Key: "abc123"}},
			}},
		},
		{
			name: "apikey no keys",
			cfg: auth.AuthConfig{APIKey: &auth.APIKeyConfig{
				Enabled: true,
			}},
			wantErr: true,
			errMsg:  "at least one key is required",
		},
		{
			name: "apikey empty key value",
			cfg: auth.AuthConfig{APIKey: &auth.APIKeyConfig{
				Enabled: true, Keys: []auth.APIKeyEntry{{Name: "ci", Key: ""}},
			}},
			wantErr: true,
			errMsg:  "key value is required",
		},
		{
			name: "valid jwt",
			cfg: auth.AuthConfig{JWT: &auth.JWTConfig{
				Enabled: true,
				Issuers: []auth.JWTIssuerConfig{{
					Name: "test", IssuerURL: "https://issuer.example.com/", JWKSURL: "https://issuer.example.com/.well-known/jwks.json",
				}},
			}},
		},
		{
			name: "jwt no issuers",
			cfg: auth.AuthConfig{JWT: &auth.JWTConfig{
				Enabled: true,
			}},
			wantErr: true,
			errMsg:  "at least one issuer is required",
		},
		{
			name: "jwt missing issuer URL",
			cfg: auth.AuthConfig{JWT: &auth.JWTConfig{
				Enabled: true,
				Issuers: []auth.JWTIssuerConfig{{Name: "test", JWKSURL: "https://example.com/jwks"}},
			}},
			wantErr: true,
			errMsg:  "issuerURL is required",
		},
		{
			name: "jwt missing JWKS URL",
			cfg: auth.AuthConfig{JWT: &auth.JWTConfig{
				Enabled: true,
				Issuers: []auth.JWTIssuerConfig{{Name: "test", IssuerURL: "https://example.com/"}},
			}},
			wantErr: true,
			errMsg:  "jwksURL is required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadFromFile_Valid(t *testing.T) {
	content := `
basicAuth:
  enabled: true
  username: admin
  password: secret
apiKey:
  enabled: true
  keys:
    - name: ci
      key: abc123
jwt:
  enabled: true
  issuers:
    - name: auth0
      issuerURL: https://example.auth0.com/
      audience: sreportal-api
      jwksURL: https://example.auth0.com/.well-known/jwks.json
      requiredClaims:
        scope: "release:write"
`
	path := writeTemp(t, content)

	cfg, err := auth.LoadFromFile(path)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled())
	assert.Equal(t, "admin", cfg.BasicAuth.Username)
	assert.Len(t, cfg.APIKey.Keys, 1)
	assert.Len(t, cfg.JWT.Issuers, 1)
	assert.Equal(t, "release:write", cfg.JWT.Issuers[0].RequiredClaims["scope"])
}

func TestLoadFromFile_FileNotFound(t *testing.T) {
	_, err := auth.LoadFromFile("/nonexistent/path.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read auth config")
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	path := writeTemp(t, "{{invalid yaml")

	_, err := auth.LoadFromFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse auth config")
}

func TestLoadFromFile_ValidationFails(t *testing.T) {
	content := `
basicAuth:
  enabled: true
  username: ""
  password: secret
`
	path := writeTemp(t, content)

	_, err := auth.LoadFromFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid auth config")
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
