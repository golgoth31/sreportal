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
)

// Authenticator validates a request and returns nil on success.
type Authenticator interface {
	Authenticate(ctx context.Context, headers http.Header) error
}

// Chain tries each authenticator in order. If any one succeeds, the request
// is considered authenticated. If all fail, the last error is returned.
type Chain struct {
	authenticators []Authenticator
}

// NewChain creates a new authentication chain from the given authenticators.
func NewChain(authenticators ...Authenticator) *Chain {
	return &Chain{authenticators: authenticators}
}

// Authenticate tries each authenticator. Returns nil on the first success.
func (c *Chain) Authenticate(ctx context.Context, headers http.Header) error {
	if len(c.authenticators) == 0 {
		return ErrNoAuthMethod
	}

	var errs []string
	for _, a := range c.authenticators {
		if err := a.Authenticate(ctx, headers); err == nil {
			return nil
		} else {
			errs = append(errs, err.Error())
		}
	}

	return fmt.Errorf("%w: %s", ErrUnauthenticated, strings.Join(errs, "; "))
}
