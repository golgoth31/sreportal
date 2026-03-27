// Package incident provides the in-memory IncidentStore implementation
// backed by the generic readstore.Store.
package incident

import (
	"context"

	domainincident "github.com/golgoth31/sreportal/internal/domain/incident"
	"github.com/golgoth31/sreportal/internal/readstore"
)

// IncidentStore is the in-memory implementation of IncidentReader and IncidentWriter.
// Keys are resource keys (e.g., "namespace/incident-name").
type IncidentStore struct {
	store *readstore.Store[domainincident.IncidentView]
}

// NewIncidentStore creates a new empty IncidentStore.
func NewIncidentStore() *IncidentStore {
	return &IncidentStore{store: readstore.New[domainincident.IncidentView]()}
}

// compile-time interface checks
var (
	_ domainincident.IncidentReader = (*IncidentStore)(nil)
	_ domainincident.IncidentWriter = (*IncidentStore)(nil)
)

// Replace stores incident views for the given key.
func (s *IncidentStore) Replace(_ context.Context, key string, views []domainincident.IncidentView) error {
	s.store.Replace(key, views)
	return nil
}

// Delete removes the incident views for the given key.
func (s *IncidentStore) Delete(_ context.Context, key string) error {
	s.store.Delete(key)
	return nil
}

// List returns incident views matching the given options.
func (s *IncidentStore) List(_ context.Context, opts domainincident.ListOptions) ([]domainincident.IncidentView, error) {
	all := s.store.All()

	if opts.PortalRef == "" && opts.Phase == "" {
		return all, nil
	}

	filtered := make([]domainincident.IncidentView, 0, len(all))
	for _, v := range all {
		if opts.PortalRef != "" && v.PortalRef != opts.PortalRef {
			continue
		}
		if opts.Phase != "" && v.CurrentPhase != opts.Phase {
			continue
		}
		filtered = append(filtered, v)
	}

	return filtered, nil
}

// Subscribe returns a channel that is closed on the next mutation.
func (s *IncidentStore) Subscribe() <-chan struct{} {
	return s.store.Subscribe()
}
