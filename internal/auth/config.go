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
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

// AuthConfig holds the complete authentication configuration.
type AuthConfig struct {
	BasicAuth *BasicAuthConfig `json:"basicAuth,omitempty" yaml:"basicAuth,omitempty"`
	APIKey    *APIKeyConfig    `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	JWT       *JWTConfig       `json:"jwt,omitempty" yaml:"jwt,omitempty"`
}

// BasicAuthConfig configures HTTP Basic authentication.
type BasicAuthConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

// APIKeyConfig configures X-API-Key header authentication.
type APIKeyConfig struct {
	Enabled bool          `json:"enabled" yaml:"enabled"`
	Keys    []APIKeyEntry `json:"keys" yaml:"keys"`
}

// APIKeyEntry is a named API key.
type APIKeyEntry struct {
	Name string `json:"name" yaml:"name"`
	Key  string `json:"key" yaml:"key"`
}

// JWTConfig configures JWT Bearer token authentication.
type JWTConfig struct {
	Enabled bool              `json:"enabled" yaml:"enabled"`
	Issuers []JWTIssuerConfig `json:"issuers" yaml:"issuers"`
}

// JWTIssuerConfig configures a single JWT issuer.
type JWTIssuerConfig struct {
	Name           string            `json:"name" yaml:"name"`
	IssuerURL      string            `json:"issuerURL" yaml:"issuerURL"`
	Audience       string            `json:"audience" yaml:"audience"`
	JWKSURL        string            `json:"jwksURL" yaml:"jwksURL"`
	RequiredClaims map[string]string `json:"requiredClaims,omitempty" yaml:"requiredClaims,omitempty"`
}

// Enabled returns true if at least one authentication method is enabled.
func (c *AuthConfig) Enabled() bool {
	if c.BasicAuth != nil && c.BasicAuth.Enabled {
		return true
	}
	if c.APIKey != nil && c.APIKey.Enabled {
		return true
	}
	if c.JWT != nil && c.JWT.Enabled {
		return true
	}
	return false
}

// Validate checks that the configuration is internally consistent.
func (c *AuthConfig) Validate() error {
	if c.BasicAuth != nil && c.BasicAuth.Enabled {
		if c.BasicAuth.Username == "" {
			return fmt.Errorf("basicAuth: %w: username is required", ErrInvalidCredentials)
		}
		if c.BasicAuth.Password == "" {
			return fmt.Errorf("basicAuth: %w: password is required", ErrInvalidCredentials)
		}
	}
	if c.APIKey != nil && c.APIKey.Enabled {
		if len(c.APIKey.Keys) == 0 {
			return fmt.Errorf("apiKey: %w: at least one key is required", ErrInvalidCredentials)
		}
		for i, k := range c.APIKey.Keys {
			if k.Key == "" {
				return fmt.Errorf("apiKey.keys[%d]: %w: key value is required", i, ErrInvalidCredentials)
			}
		}
	}
	if c.JWT != nil && c.JWT.Enabled {
		if len(c.JWT.Issuers) == 0 {
			return fmt.Errorf("jwt: %w: at least one issuer is required", ErrInvalidCredentials)
		}
		for i, iss := range c.JWT.Issuers {
			if iss.IssuerURL == "" {
				return fmt.Errorf("jwt.issuers[%d]: issuerURL is required", i)
			}
			if iss.JWKSURL == "" {
				return fmt.Errorf("jwt.issuers[%d]: jwksURL is required", i)
			}
		}
	}
	return nil
}

// LoadFromFile reads the authentication configuration from a YAML file.
func LoadFromFile(path string) (*AuthConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read auth config: %w", err)
	}

	cfg := &AuthConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse auth config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid auth config: %w", err)
	}

	return cfg, nil
}
