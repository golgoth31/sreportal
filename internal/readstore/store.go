// Package readstore provides a generic in-memory read store with broadcast
// notification for change subscribers. It is the foundation for CQRS read-side
// projections used by gRPC and MCP services.
package readstore

import (
	"sync"
)

// Store is a generic in-memory read store. Data is partitioned by resource key
// (e.g., "namespace/dns-cr-name"). Mutations broadcast to subscribers via a
// channel-close pattern.
type Store[T any] struct {
	mu   sync.RWMutex
	data map[string][]T

	notifyMu sync.Mutex
	notifyCh chan struct{}
}

// New creates a new empty Store.
func New[T any]() *Store[T] {
	return &Store[T]{
		data:     make(map[string][]T),
		notifyCh: make(chan struct{}),
	}
}

// Replace atomically swaps all entries for a key and broadcasts to subscribers.
func (s *Store[T]) Replace(key string, items []T) {
	s.mu.Lock()
	s.data[key] = items
	s.mu.Unlock()

	s.broadcast()
}

// Delete removes a key and broadcasts to subscribers.
func (s *Store[T]) Delete(key string) {
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()

	s.broadcast()
}

// All returns a flat snapshot of all values across all keys. The returned slice
// is a copy; callers may mutate it freely without affecting the store.
func (s *Store[T]) All() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for _, items := range s.data {
		total += len(items)
	}

	out := make([]T, 0, total)
	for _, items := range s.data {
		out = append(out, items...)
	}

	return out
}

// Subscribe returns a channel that will be closed on the next mutation
// (Replace or Delete). After receiving the notification, callers must call
// Subscribe() again to wait for subsequent changes.
func (s *Store[T]) Subscribe() <-chan struct{} {
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()

	return s.notifyCh
}

// broadcast closes the current notification channel and replaces it with a
// fresh one, waking all goroutines waiting on Subscribe().
func (s *Store[T]) broadcast() {
	s.notifyMu.Lock()
	old := s.notifyCh
	s.notifyCh = make(chan struct{})
	s.notifyMu.Unlock()

	close(old)
}
