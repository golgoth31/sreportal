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

package dnsresolve

import (
	"math/rand"
	"sync"
	"time"
)

// FQDNKey identifies one endpoint of a DNSRecord. RecordKey is "namespace/name".
type FQDNKey struct {
	RecordKey  string
	DNSName    string
	RecordType string
}

// scheduler tracks, per FQDN, when it is next due for a DNS check. New keys are
// spread uniformly across (now, now+interval] so the steady-state check rate is
// ~len(keys)/interval with no thundering herd (including after a restart).
//
// Jitter choice: new keys receive a jitter in [1, interval] (i.e. strictly after
// now) rather than [0, interval). This guarantees that Due(now) always returns 0
// for freshly-synced keys, making the spread test deterministic regardless of RNG
// seed or number of keys.
type scheduler struct {
	mu       sync.Mutex
	interval time.Duration
	now      func() time.Time
	rng      *rand.Rand
	next     map[FQDNKey]time.Time
}

func newScheduler(interval time.Duration, now func() time.Time, seed int64) *scheduler {
	return &scheduler{
		interval: interval,
		now:      now,
		rng:      rand.New(rand.NewSource(seed)),
		next:     map[FQDNKey]time.Time{},
	}
}

// Sync reconciles tracked keys with the desired set: new keys get a jittered
// nextCheck in (now, now+interval]; removed keys are dropped.
func (s *scheduler) Sync(keys []FQDNKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	desired := make(map[FQDNKey]struct{}, len(keys))
	now := s.now()
	for _, k := range keys {
		desired[k] = struct{}{}
		if _, ok := s.next[k]; !ok {
			// jitter in [1, interval] — strictly after now, at most now+interval
			jitter := time.Duration(1 + s.rng.Int63n(int64(s.interval)))
			s.next[k] = now.Add(jitter)
		}
	}
	for k := range s.next {
		if _, ok := desired[k]; !ok {
			delete(s.next, k)
		}
	}
}

// Due returns keys whose nextCheck is at or before t.
func (s *scheduler) Due(t time.Time) []FQDNKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []FQDNKey
	for k, n := range s.next {
		if !n.After(t) {
			out = append(out, k)
		}
	}
	return out
}

// Reschedule pushes a key's next check to now+interval+1ns (strictly after
// now+interval so that Due(now+interval) does not return this key).
func (s *scheduler) Reschedule(k FQDNKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.next[k]; ok {
		s.next[k] = s.now().Add(s.interval + time.Nanosecond)
	}
}

// ForceRecord makes every tracked key of a record immediately due.
func (s *scheduler) ForceRecord(recordKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for k := range s.next {
		if k.RecordKey == recordKey {
			s.next[k] = now.Add(-time.Nanosecond)
		}
	}
}
