// Package release provides the in-memory ReleaseStore implementation
// backed by the generic readstore.Store.
package release

import (
	"context"
	"sort"

	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	"github.com/golgoth31/sreportal/internal/readstore"
)

// ReleaseStore is the in-memory implementation of ReleaseReader and ReleaseWriter.
// Keys are namespace/name of the Release CR; each EntryView carries PortalRef and Day for queries.
type ReleaseStore struct {
	store *readstore.Store[domainrelease.EntryView]
}

// NewReleaseStore creates a new empty ReleaseStore.
func NewReleaseStore() *ReleaseStore {
	return &ReleaseStore{store: readstore.New[domainrelease.EntryView]()}
}

// compile-time interface checks
var (
	_ domainrelease.ReleaseReader = (*ReleaseStore)(nil)
	_ domainrelease.ReleaseWriter = (*ReleaseStore)(nil)
)

// Replace stores entries under the given resource key (namespace/name).
func (s *ReleaseStore) Replace(_ context.Context, resourceKey string, entries []domainrelease.EntryView) error {
	s.store.Replace(resourceKey, entries)
	return nil
}

// Delete removes the entries for the given resource key.
func (s *ReleaseStore) Delete(_ context.Context, resourceKey string) error {
	s.store.Delete(resourceKey)
	return nil
}

// ListEntries returns entries for a specific day, optionally scoped to a portal name.
// When portal is empty, entries from all portals for that day are merged and sorted by time.
func (s *ReleaseStore) ListEntries(_ context.Context, day, portal string) ([]domainrelease.EntryView, error) {
	all := s.store.All()
	var out []domainrelease.EntryView
	for i := range all {
		e := all[i]
		if e.Day != day {
			continue
		}
		if portal != "" && e.PortalRef != portal {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		di := out[i].Date
		dj := out[j].Date
		if !di.Equal(dj) {
			return di.Before(dj)
		}
		return out[i].Origin < out[j].Origin
	})
	return out, nil
}

// ListDays returns all days that have entries for the given portal, sorted ascending.
// When portal is empty, returns unique days across all portals.
func (s *ReleaseStore) ListDays(_ context.Context, portal string) ([]string, error) {
	all := s.store.All()
	seen := make(map[string]struct{})
	for i := range all {
		e := all[i]
		if portal != "" && e.PortalRef != portal {
			continue
		}
		if e.Day != "" {
			seen[e.Day] = struct{}{}
		}
	}
	days := make([]string, 0, len(seen))
	for d := range seen {
		days = append(days, d)
	}
	sort.Strings(days)
	return days, nil
}

// Subscribe returns a channel that is closed on the next mutation.
func (s *ReleaseStore) Subscribe() <-chan struct{} {
	return s.store.Subscribe()
}
