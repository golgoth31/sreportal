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
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/golgoth31/sreportal/internal/domain/dns"
)

func TestParseResourceRef_ValidInput_ParsesKindNamespaceAndName(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantKind string
		wantNS   string
		wantName string
	}{
		{"service", "service/default/my-svc", "service", "default", "my-svc"},
		{"ingress", "ingress/production/my-ingress", "ingress", "production", "my-ingress"},
		{"dnsendpoint", "dnsendpoint/kube-system/my-ep", "dnsendpoint", "kube-system", "my-ep"},
		{"name with hyphens", "service/my-ns/long-svc-name", "service", "my-ns", "long-svc-name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := dns.ParseResourceRef(tc.input)

			require.NoError(t, err)
			require.Equal(t, tc.wantKind, ref.Kind())
			require.Equal(t, tc.wantNS, ref.Namespace())
			require.Equal(t, tc.wantName, ref.Name())
			require.False(t, ref.IsZero())
		})
	}
}

func TestParseResourceRef_InvalidFormat_ReturnsErrInvalidResourceRef(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"kind only", "service"},
		{"kind/name only (missing namespace)", "service/default"},
		{"empty kind", "/default/my-svc"},
		{"empty namespace", "service//my-svc"},
		{"empty name", "service/default/"},
		{"whitespace only kind", "  /default/my-svc"},
		{"whitespace only namespace", "service/  /my-svc"},
		{"whitespace only name", "service/default/  "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dns.ParseResourceRef(tc.input)

			require.Error(t, err)
			require.True(t, errors.Is(err, dns.ErrInvalidResourceRef),
				"expected ErrInvalidResourceRef, got: %v", err)
		})
	}
}

func TestResourceRef_IsZero_OnZeroValue(t *testing.T) {
	var ref dns.ResourceRef
	require.True(t, ref.IsZero())
}

func TestResourceRef_IsZero_OnParsedValue(t *testing.T) {
	ref, err := dns.ParseResourceRef("service/default/my-svc")
	require.NoError(t, err)
	require.False(t, ref.IsZero())
}

func TestResourceRef_Equality_SameValues(t *testing.T) {
	// Value objects: two instances with same content must be equal.
	a, err := dns.ParseResourceRef("service/default/my-svc")
	require.NoError(t, err)

	b, err := dns.ParseResourceRef("service/default/my-svc")
	require.NoError(t, err)

	require.Equal(t, a, b)
}
