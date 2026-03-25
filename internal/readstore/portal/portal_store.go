// Package portal provides the in-memory PortalStore implementation backed by the
// generic readstore.Store. It implements both portal.PortalReader and portal.PortalWriter.
package portal

import (
	"cmp"
	"context"
	"slices"

	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
	"github.com/golgoth31/sreportal/internal/readstore"
)

// PortalStore is the in-memory implementation of portal.PortalReader and portal.PortalWriter.
// Each key maps to a single PortalView (wrapped in a slice for the generic store).
type PortalStore struct {
	store *readstore.Store[domainportal.PortalView]
}

// NewPortalStore creates a new empty PortalStore.
func NewPortalStore() *PortalStore {
	return &PortalStore{store: readstore.New[domainportal.PortalView]()}
}

// compile-time interface checks
var (
	_ domainportal.PortalReader = (*PortalStore)(nil)
	_ domainportal.PortalWriter = (*PortalStore)(nil)
)

// Replace stores a portal view for the given key.
func (s *PortalStore) Replace(_ context.Context, key string, portal domainportal.PortalView) error {
	s.store.Replace(key, []domainportal.PortalView{portal})
	return nil
}

// Delete removes the portal for the given key.
func (s *PortalStore) Delete(_ context.Context, key string) error {
	s.store.Delete(key)
	return nil
}

// List returns all portals matching the given filters, sorted by name.
func (s *PortalStore) List(_ context.Context, filters domainportal.PortalFilters) ([]domainportal.PortalView, error) {
	all := s.store.All()

	var results []domainportal.PortalView
	for _, v := range all {
		if filters.Namespace != "" && v.Namespace != filters.Namespace {
			continue
		}
		results = append(results, v)
	}

	slices.SortFunc(results, func(a, b domainportal.PortalView) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return results, nil
}

// Subscribe returns a channel that is closed on the next mutation.
func (s *PortalStore) Subscribe() <-chan struct{} {
	return s.store.Subscribe()
}
