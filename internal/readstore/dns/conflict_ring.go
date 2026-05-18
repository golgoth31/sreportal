package dns

import (
	"sync"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

type conflictRing struct {
	mu   sync.Mutex
	buf  []domaindns.ConflictEvent
	idx  int
	full bool
	size int
}

func newConflictRing(capacity int) *conflictRing {
	return &conflictRing{buf: make([]domaindns.ConflictEvent, capacity), size: capacity}
}

// Push records a new conflict, overwriting the oldest entry when full.
// A zero-capacity ring silently discards.
func (r *conflictRing) Push(e domaindns.ConflictEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size == 0 {
		return
	}
	r.buf[r.idx] = e
	r.idx = (r.idx + 1) % r.size
	if r.idx == 0 {
		r.full = true
	}
}

// Snapshot returns a copy of all events currently held.
func (r *conflictRing) Snapshot() []domaindns.ConflictEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		out := make([]domaindns.ConflictEvent, r.idx)
		copy(out, r.buf[:r.idx])
		return out
	}
	out := make([]domaindns.ConflictEvent, r.size)
	copy(out, r.buf[r.idx:])
	copy(out[r.size-r.idx:], r.buf[:r.idx])
	return out
}
