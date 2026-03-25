// Package alertmanager provides the in-memory AlertmanagerStore implementation
// backed by the generic readstore.Store.
package alertmanager

import (
	"cmp"
	"context"
	"slices"

	domainam "github.com/golgoth31/sreportal/internal/domain/alertmanagerreadmodel"
	"github.com/golgoth31/sreportal/internal/readstore"
)

// AlertmanagerStore is the in-memory implementation of AlertmanagerReader and AlertmanagerWriter.
type AlertmanagerStore struct {
	store *readstore.Store[domainam.AlertmanagerView]
}

// NewAlertmanagerStore creates a new empty AlertmanagerStore.
func NewAlertmanagerStore() *AlertmanagerStore {
	return &AlertmanagerStore{store: readstore.New[domainam.AlertmanagerView]()}
}

// compile-time interface checks
var (
	_ domainam.AlertmanagerReader = (*AlertmanagerStore)(nil)
	_ domainam.AlertmanagerWriter = (*AlertmanagerStore)(nil)
)

// Replace stores an AlertmanagerView for the given key.
func (s *AlertmanagerStore) Replace(_ context.Context, key string, view domainam.AlertmanagerView) error {
	s.store.Replace(key, []domainam.AlertmanagerView{view})
	return nil
}

// Delete removes the entry for the given key.
func (s *AlertmanagerStore) Delete(_ context.Context, key string) error {
	s.store.Delete(key)
	return nil
}

// List returns all AlertmanagerViews matching the given filters, sorted by (namespace, name).
func (s *AlertmanagerStore) List(_ context.Context, filters domainam.AlertmanagerFilters) ([]domainam.AlertmanagerView, error) {
	all := s.store.All()

	var results []domainam.AlertmanagerView
	for _, v := range all {
		if filters.Portal != "" && v.PortalRef != filters.Portal {
			continue
		}
		if filters.Namespace != "" && v.Namespace != filters.Namespace {
			continue
		}
		results = append(results, v)
	}

	slices.SortFunc(results, func(a, b domainam.AlertmanagerView) int {
		if c := cmp.Compare(a.Namespace, b.Namespace); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})

	return results, nil
}

// Subscribe returns a channel that is closed on the next mutation.
func (s *AlertmanagerStore) Subscribe() <-chan struct{} {
	return s.store.Subscribe()
}
