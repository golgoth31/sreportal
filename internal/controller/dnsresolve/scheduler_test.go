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
	"testing"
	"time"
)

func tk(record, fqdn, rt string) FQDNKey {
	return FQDNKey{RecordKey: record, DNSName: fqdn, RecordType: rt}
}

func TestScheduler_SyncSpreadsAcrossInterval(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	s := newScheduler(24*time.Hour, func() time.Time { return base }, 1)
	keys := make([]FQDNKey, 200)
	for i := range keys {
		keys[i] = tk("ns/r", "h-"+time.Duration(i).String(), "A")
	}
	s.Sync(keys)
	if due := s.Due(base); len(due) != 0 {
		t.Fatalf("expected 0 due at base, got %d", len(due))
	}
	if due := s.Due(base.Add(24 * time.Hour)); len(due) != 200 {
		t.Fatalf("expected 200 due by +24h, got %d", len(due))
	}
}

func TestScheduler_RescheduleMovesNextOut(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	s := newScheduler(24*time.Hour, func() time.Time { return base }, 1)
	k := tk("ns/r", "a.example.com", "A")
	s.Sync([]FQDNKey{k})
	s.Reschedule(k)
	if due := s.Due(base.Add(24 * time.Hour)); len(due) != 0 {
		t.Fatalf("rescheduled key must not be due before base+interval, got %d", len(due))
	}
	if due := s.Due(base.Add(48 * time.Hour)); len(due) != 1 {
		t.Fatalf("rescheduled key must be due by base+2*interval, got %d", len(due))
	}
}

func TestScheduler_ForceRecordMakesAllRecordKeysDue(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	s := newScheduler(24*time.Hour, func() time.Time { return base }, 1)
	a := tk("ns/r", "a", "A")
	b := tk("ns/r", "b", "A")
	other := tk("ns/other", "c", "A")
	s.Sync([]FQDNKey{a, b, other})
	s.ForceRecord("ns/r")
	if due := s.Due(base); len(due) != 2 {
		t.Fatalf("force must make the record's 2 keys due, got %d", len(due))
	}
}

func TestScheduler_SyncDropsRemovedKeys(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	s := newScheduler(24*time.Hour, func() time.Time { return base }, 1)
	a := tk("ns/r", "a", "A")
	b := tk("ns/r", "b", "A")
	s.Sync([]FQDNKey{a, b})
	s.Sync([]FQDNKey{a})
	s.ForceRecord("ns/r")
	if due := s.Due(base); len(due) != 1 {
		t.Fatalf("expected only surviving key due, got %d", len(due))
	}
}
