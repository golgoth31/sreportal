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

package statuspage

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCRName_SimpleCase(t *testing.T) {
	name := GenerateCRName("main", "API Down")
	assert.LessOrEqual(t, len(name), 63)
	assert.True(t, strings.HasPrefix(name, "main-api-down-"))
	// Hash suffix is 7 hex chars
	parts := strings.Split(name, "-")
	hash := parts[len(parts)-1]
	assert.Len(t, hash, 7)
}

func TestGenerateCRName_Deterministic(t *testing.T) {
	a := GenerateCRName("main", "API Down")
	b := GenerateCRName("main", "API Down")
	assert.Equal(t, a, b)
}

func TestGenerateCRName_DifferentInputsDifferentHash(t *testing.T) {
	a := GenerateCRName("main", "API Down")
	b := GenerateCRName("main", "DB Down")
	assert.NotEqual(t, a, b)
}

func TestGenerateCRName_DifferentPortalsDifferentHash(t *testing.T) {
	a := GenerateCRName("prod", "API Down")
	b := GenerateCRName("staging", "API Down")
	assert.NotEqual(t, a, b)
}

func TestGenerateCRName_MaxLength63(t *testing.T) {
	longPortal := strings.Repeat("a", 100)
	longTitle := strings.Repeat("b", 100)
	name := GenerateCRName(longPortal, longTitle)
	assert.LessOrEqual(t, len(name), 63)
}

func TestGenerateCRName_TruncatesTitleFirst(t *testing.T) {
	portal := "main"
	longTitle := strings.Repeat("x", 100)
	name := GenerateCRName(portal, longTitle)
	assert.LessOrEqual(t, len(name), 63)
	assert.True(t, strings.HasPrefix(name, "main-"))
}

func TestGenerateCRName_TruncatesPortalIfNeeded(t *testing.T) {
	longPortal := strings.Repeat("p", 60)
	title := "a"
	name := GenerateCRName(longPortal, title)
	assert.LessOrEqual(t, len(name), 63)
	// Portal must have been truncated
	parts := strings.SplitN(name, "-", 2)
	assert.Less(t, len(parts[0]), 60)
}

func TestGenerateCRName_SlugifiesSpecialChars(t *testing.T) {
	name := GenerateCRName("My Portal!", "API Gateway (v2)")
	assert.LessOrEqual(t, len(name), 63)
	// No uppercase, no special chars except hyphens
	for _, c := range name {
		assert.True(t, (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-',
			"unexpected char: %c", c)
	}
}

func TestGenerateCRName_NoLeadingTrailingHyphens(t *testing.T) {
	name := GenerateCRName("--portal--", "--title--")
	assert.False(t, strings.HasPrefix(name, "-"))
	assert.False(t, strings.HasSuffix(name, "-"))
}

func TestGenerateCRName_CollapsesConsecutiveHyphens(t *testing.T) {
	name := GenerateCRName("my   portal", "some---title")
	assert.NotContains(t, name, "--")
}

func TestGenerateCRName_HashComputedOnFullInput(t *testing.T) {
	// Two inputs that would slug to the same prefix but differ in the truncated part
	titleA := "api-gateway-" + strings.Repeat("a", 50)
	titleB := "api-gateway-" + strings.Repeat("b", 50)
	a := GenerateCRName("main", titleA)
	b := GenerateCRName("main", titleB)
	// Same portal, same slug prefix after truncation, but different hash
	assert.NotEqual(t, a, b)
}

func TestGenerateCRName_ValidDNS1123Label(t *testing.T) {
	cases := []struct {
		portal string
		title  string
	}{
		{"main", "API Down"},
		{"UPPER", "CASE Title"},
		{"123", "456"},
		{"a-b-c", "d-e-f"},
		{"special!@#$%", "chars&*()"},
	}
	for _, tc := range cases {
		name := GenerateCRName(tc.portal, tc.title)
		require.LessOrEqual(t, len(name), 63)
		require.Regexp(t, `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, name)
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"API Gateway (v2)", "api-gateway-v2"},
		{"--leading--trailing--", "leading-trailing"},
		{"UPPERCASE", "uppercase"},
		{"already-slug", "already-slug"},
		{"multiple   spaces", "multiple-spaces"},
		{"special!@#$chars", "special-chars"},
		{"123-numbers", "123-numbers"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, slugify(tc.input))
		})
	}
}
