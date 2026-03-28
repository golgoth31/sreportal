// Package maintenance provides the in-memory MaintenanceStore implementation
// backed by the generic readstore.Store.
package maintenance

import (
	"context"

	domainmaint "github.com/golgoth31/sreportal/internal/domain/maintenance"
	"github.com/golgoth31/sreportal/internal/readstore"
)

// MaintenanceStore is the in-memory implementation of MaintenanceReader and MaintenanceWriter.
// Keys are resource keys (e.g., "namespace/maintenance-name").
type MaintenanceStore struct {
	store *readstore.Store[domainmaint.MaintenanceView]
}

// NewMaintenanceStore creates a new empty MaintenanceStore.
func NewMaintenanceStore() *MaintenanceStore {
	return &MaintenanceStore{store: readstore.New[domainmaint.MaintenanceView]()}
}

// compile-time interface checks
var (
	_ domainmaint.MaintenanceReader = (*MaintenanceStore)(nil)
	_ domainmaint.MaintenanceWriter = (*MaintenanceStore)(nil)
)

// Replace stores maintenance views for the given key.
func (s *MaintenanceStore) Replace(_ context.Context, key string, views []domainmaint.MaintenanceView) error {
	s.store.Replace(key, views)
	return nil
}

// Delete removes the maintenance views for the given key.
func (s *MaintenanceStore) Delete(_ context.Context, key string) error {
	s.store.Delete(key)
	return nil
}

// List returns maintenance views matching the given options.
func (s *MaintenanceStore) List(_ context.Context, opts domainmaint.ListOptions) ([]domainmaint.MaintenanceView, error) {
	all := s.store.All()

	if opts.PortalRef == "" && opts.Phase == "" {
		return all, nil
	}

	filtered := make([]domainmaint.MaintenanceView, 0, len(all))
	for _, v := range all {
		if opts.PortalRef != "" && v.PortalRef != opts.PortalRef {
			continue
		}
		if opts.Phase != "" && v.Phase != opts.Phase {
			continue
		}
		filtered = append(filtered, v)
	}

	return filtered, nil
}

// Subscribe returns a channel that is closed on the next mutation.
func (s *MaintenanceStore) Subscribe() <-chan struct{} {
	return s.store.Subscribe()
}
