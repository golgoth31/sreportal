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

package dns

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

func makeEvent(n int) domaindns.ConflictEvent {
	return domaindns.ConflictEvent{
		FQDNKey:      domaindns.ConflictFQDNKey{Name: fmt.Sprintf("fqdn-%d.example.com", n), RecordType: "A"},
		WinnerRecord: fmt.Sprintf("ns/winner-%d", n),
		LoserRecord:  fmt.Sprintf("ns/loser-%d", n),
		PortalRef:    "main",
		At:           time.Unix(int64(n), 0),
	}
}

// TestConflictRing_PushUnderCapacity verifies that N pushes (N < size) return
// all events in insertion order.
func TestConflictRing_PushUnderCapacity(t *testing.T) {
	r := newConflictRing(10)

	for i := 0; i < 5; i++ {
		r.Push(makeEvent(i))
	}

	snap := r.Snapshot()
	require.Len(t, snap, 5)
	for i, e := range snap {
		assert.Equal(t, makeEvent(i), e, "event at index %d mismatch", i)
	}
}

// TestConflictRing_PushExactlyToCapacity fills the ring without triggering
// wrap-around; all events must be returned in insertion order.
func TestConflictRing_PushExactlyToCapacity(t *testing.T) {
	const size = 4
	r := newConflictRing(size)

	for i := 0; i < size; i++ {
		r.Push(makeEvent(i))
	}

	snap := r.Snapshot()
	require.Len(t, snap, size)
	for i, e := range snap {
		assert.Equal(t, makeEvent(i), e, "event at index %d mismatch", i)
	}
}

// TestConflictRing_PushPastCapacity pushes size+k events and verifies that
// only the most-recent size events are returned in insertion order (the
// earliest k are dropped).
func TestConflictRing_PushPastCapacity(t *testing.T) {
	const size = 4
	const k = 3
	r := newConflictRing(size)

	total := size + k
	for i := 0; i < total; i++ {
		r.Push(makeEvent(i))
	}

	snap := r.Snapshot()
	require.Len(t, snap, size, "snapshot must hold exactly size events after wrap")

	// Oldest k events (0..k-1) must be gone; newest size events (k..total-1)
	// must appear in insertion order.
	for idx, e := range snap {
		want := makeEvent(k + idx)
		assert.Equal(t, want, e, "event at snapshot index %d should be event %d", idx, k+idx)
	}
}

// TestConflictRing_ZeroCapacity verifies that a ring of capacity 0 silently
// discards Push calls and Snapshot returns an empty slice.
func TestConflictRing_ZeroCapacity(t *testing.T) {
	r := newConflictRing(0)

	// Must not panic
	r.Push(makeEvent(0))
	r.Push(makeEvent(1))

	snap := r.Snapshot()
	assert.Empty(t, snap, "zero-capacity ring must always return empty snapshot")
}

// TestConflictRing_SnapshotIsCopy verifies that mutating the returned slice
// does not corrupt subsequent Snapshot calls.
func TestConflictRing_SnapshotIsCopy(t *testing.T) {
	r := newConflictRing(3)
	r.Push(makeEvent(0))
	r.Push(makeEvent(1))

	snap1 := r.Snapshot()
	require.Len(t, snap1, 2)

	// Corrupt the snapshot in-place
	snap1[0] = domaindns.ConflictEvent{}
	snap1[1] = domaindns.ConflictEvent{}

	snap2 := r.Snapshot()
	require.Len(t, snap2, 2, "subsequent snapshot must have same length")
	assert.Equal(t, makeEvent(0), snap2[0], "first event must not be corrupted by prior mutation")
	assert.Equal(t, makeEvent(1), snap2[1], "second event must not be corrupted by prior mutation")
}
