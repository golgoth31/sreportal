// Package deploystatus implements the in-memory read store for deploy-status entries.
package deploystatus

import (
	"strings"
	"sync"

	dom "github.com/golgoth31/sreportal/internal/domain/deploystatus"
)

// scopeKey builds a stable map key for a (portalRef, namespace) pair.
// The separator "|" is chosen because it cannot appear in K8s namespace names.
func scopeKey(portalRef, namespace string) string { return portalRef + "|" + namespace }

// Store is a concurrency-safe in-memory deploy-status read store.
//
// It maintains per-scope contributions indexed by (portalRef, namespace) and
// exposes a flat view per portalRef via List/Count. Multiple subscribers can
// wait for mutations via Subscribe.
type Store struct {
	mu      sync.RWMutex
	byScope map[string][]dom.Entry // key: portalRef|namespace

	subsMu sync.Mutex
	subs   map[chan struct{}]struct{}
}

var (
	_ dom.Reader = (*Store)(nil)
	_ dom.Writer = (*Store)(nil)
)

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{
		byScope: make(map[string][]dom.Entry),
		subs:    make(map[chan struct{}]struct{}),
	}
}

// ReplaceForNamespace implements Writer.
// It replaces all entries contributed by (portalRef, namespace) with entries.
// Contributions from other namespaces under the same portalRef are preserved.
func (s *Store) ReplaceForNamespace(portalRef, namespace string, entries []dom.Entry) {
	k := scopeKey(portalRef, namespace)

	s.mu.Lock()
	if len(entries) == 0 {
		delete(s.byScope, k)
	} else {
		cp := make([]dom.Entry, len(entries))
		copy(cp, entries)
		s.byScope[k] = cp
	}
	s.mu.Unlock()

	s.broadcast()
}

// RemoveForNamespace implements Writer.
// It is a convenience wrapper for ReplaceForNamespace(..., nil).
func (s *Store) RemoveForNamespace(portalRef, namespace string) {
	s.mu.Lock()
	delete(s.byScope, scopeKey(portalRef, namespace))
	s.mu.Unlock()

	s.broadcast()
}

// List implements Reader. It returns a flat snapshot of all entries across all
// namespaces for the given portalRef. The returned slice is a copy.
func (s *Store) List(portalRef string) []dom.Entry {
	prefix := portalRef + "|"

	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []dom.Entry
	for k, entries := range s.byScope {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		out = append(out, entries...)
	}
	return out
}

// Count implements Reader.
func (s *Store) Count(portalRef string) int {
	return len(s.List(portalRef))
}

// Subscribe implements Reader. It returns a channel that is closed on the next
// mutation and an unsubscribe function the caller must invoke when done.
// After the channel is closed, call Subscribe again to wait for further changes.
func (s *Store) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{})

	s.subsMu.Lock()
	s.subs[ch] = struct{}{}
	s.subsMu.Unlock()

	unsub := func() {
		s.subsMu.Lock()
		_, ok := s.subs[ch]
		if ok {
			delete(s.subs, ch)
			close(ch)
		}
		s.subsMu.Unlock()
	}
	return ch, unsub
}

// broadcast closes every subscriber channel (waking all waiters) and removes
// them from the set. Callers that want further notifications must re-subscribe.
func (s *Store) broadcast() {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()

	for ch := range s.subs {
		close(ch)
		delete(s.subs, ch)
	}
}
