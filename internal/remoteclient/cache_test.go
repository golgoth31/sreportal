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

package remoteclient

import (
	"testing"
)

func TestCache_Get_ReturnsCachedClientWhenVersionsMatch(t *testing.T) {
	cache := NewCache()
	client := NewClient()
	versions := map[string]string{"secret-a": "v1"}

	cache.Put("ns/portal", versions, client)

	got := cache.Get("ns/portal", map[string]string{"secret-a": "v1"})
	if got != client {
		t.Fatal("expected cached client, got nil")
	}
}

func TestCache_Get_ReturnsNilOnVersionMismatch(t *testing.T) {
	cache := NewCache()
	client := NewClient()

	cache.Put("ns/portal", map[string]string{"secret-a": "v1"}, client)

	got := cache.Get("ns/portal", map[string]string{"secret-a": "v2"})
	if got != nil {
		t.Fatal("expected nil on version mismatch")
	}
}

func TestCache_Get_ReturnsNilOnCacheMiss(t *testing.T) {
	cache := NewCache()

	got := cache.Get("ns/unknown", nil)
	if got != nil {
		t.Fatal("expected nil on cache miss")
	}
}

func TestCache_Get_InvalidatesWhenNewSecretAdded(t *testing.T) {
	cache := NewCache()
	client := NewClient()

	cache.Put("ns/portal", map[string]string{"ca": "v1"}, client)

	got := cache.Get("ns/portal", map[string]string{"ca": "v1", "cert": "v1"})
	if got != nil {
		t.Fatal("expected nil when secret set changed")
	}
}

func TestCache_Fallback_ReturnsSameInstance(t *testing.T) {
	cache := NewCache()

	if cache.Fallback() == nil {
		t.Fatal("expected non-nil fallback client")
	}
	if cache.Fallback() != cache.Fallback() {
		t.Fatal("expected same fallback client instance")
	}
}

func TestCache_Put_DoesNotAliasVersionsMap(t *testing.T) {
	cache := NewCache()
	client := NewClient()
	versions := map[string]string{"secret-a": "v1"}

	cache.Put("ns/portal", versions, client)

	// Mutate caller's map — should not affect cache
	versions["secret-a"] = "v2"

	got := cache.Get("ns/portal", map[string]string{"secret-a": "v1"})
	if got != client {
		t.Fatal("cache should not be affected by caller mutating the versions map")
	}
}

func TestCache_Put_OverwritesExistingEntry(t *testing.T) {
	cache := NewCache()
	old := NewClient()
	new := NewClient()

	cache.Put("ns/portal", map[string]string{"s": "v1"}, old)
	cache.Put("ns/portal", map[string]string{"s": "v2"}, new)

	if got := cache.Get("ns/portal", map[string]string{"s": "v1"}); got != nil {
		t.Fatal("old versions should no longer match")
	}
	if got := cache.Get("ns/portal", map[string]string{"s": "v2"}); got != new {
		t.Fatal("expected new client for updated versions")
	}
}
