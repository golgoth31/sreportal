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
	"maps"
	"sync"
)

// Cache caches remote clients keyed by an opaque string (typically "namespace/portalName").
// Entries are invalidated when the provided secret versions no longer match.
// The fallback client is shared across all non-TLS portals, avoiding per-call allocations.
type Cache struct {
	mu       sync.Mutex
	entries  map[string]cacheEntry
	fallback *Client
}

type cacheEntry struct {
	client         *Client
	secretVersions map[string]string
}

// NewCache creates a new cache with a shared fallback client for non-TLS portals.
func NewCache(opts ...Option) *Cache {
	return &Cache{
		entries:  make(map[string]cacheEntry),
		fallback: NewClient(opts...),
	}
}

// Fallback returns the shared client for portals without TLS configuration.
func (c *Cache) Fallback() *Client {
	return c.fallback
}

// Get returns a cached client if the secret versions match, or nil on cache miss.
func (c *Cache) Get(key string, secretVersions map[string]string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil
	}

	if !maps.Equal(entry.secretVersions, secretVersions) {
		return nil
	}

	return entry.client
}

// Put stores a client in the cache with the associated secret versions.
// The versions map is cloned to prevent caller mutations from affecting the cache.
func (c *Cache) Put(key string, secretVersions map[string]string, client *Client) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cacheEntry{
		client:         client,
		secretVersions: maps.Clone(secretVersions),
	}
}
