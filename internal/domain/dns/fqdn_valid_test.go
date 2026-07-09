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

package dns_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

func TestValidFQDN(t *testing.T) {
	cases := []struct {
		name  string
		fqdn  string
		valid bool
	}{
		{"simple", "a.example.com", true},
		{"multi label", "foo.bar.example.co", true},
		{"hyphenated", "my-svc.example.com", true},
		{"regression override sentinel", "override-on-app-set", false},
		{"single label no dot", "localhost", false},
		{"trailing dot", "example.com.", false},
		{"empty", "", false},
		{"leading hyphen label", "-bad.example.com", false},
		{"tld too short", "a.b.c", false},
		{"underscore", "a_b.example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.valid, domaindns.ValidFQDN(tc.fqdn))
		})
	}
}
