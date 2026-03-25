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
// Keys are day strings (e.g. "2026-03-25").
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

// Replace stores entries for the given day.
func (s *ReleaseStore) Replace(_ context.Context, day string, entries []domainrelease.EntryView) error {
	s.store.Replace(day, entries)
	return nil
}

// Delete removes the entries for the given day.
func (s *ReleaseStore) Delete(_ context.Context, day string) error {
	s.store.Delete(day)
	return nil
}

// ListEntries returns entries for a specific day. Returns empty slice if day not found.
func (s *ReleaseStore) ListEntries(_ context.Context, day string) ([]domainrelease.EntryView, error) {
	items := s.store.Get(day)
	if items == nil {
		return []domainrelease.EntryView{}, nil
	}
	return items, nil
}

// ListDays returns all days that have entries, sorted ascending.
func (s *ReleaseStore) ListDays(_ context.Context) ([]string, error) {
	keys := s.store.Keys()
	sort.Strings(keys)
	return keys, nil
}

// Subscribe returns a channel that is closed on the next mutation.
func (s *ReleaseStore) Subscribe() <-chan struct{} {
	return s.store.Subscribe()
}
