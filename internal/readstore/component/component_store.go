// Package component provides the in-memory ComponentStore implementation
// backed by the generic readstore.Store.
package component

import (
	"context"

	domaincomponent "github.com/golgoth31/sreportal/internal/domain/component"
	"github.com/golgoth31/sreportal/internal/readstore"
)

// ComponentStore is the in-memory implementation of ComponentReader and ComponentWriter.
// Keys are resource keys (e.g., "namespace/component-name").
type ComponentStore struct {
	store *readstore.Store[domaincomponent.ComponentView]
}

// NewComponentStore creates a new empty ComponentStore.
func NewComponentStore() *ComponentStore {
	return &ComponentStore{store: readstore.New[domaincomponent.ComponentView]()}
}

// compile-time interface checks
var (
	_ domaincomponent.ComponentReader = (*ComponentStore)(nil)
	_ domaincomponent.ComponentWriter = (*ComponentStore)(nil)
)

// Replace stores component views for the given key.
func (s *ComponentStore) Replace(_ context.Context, key string, views []domaincomponent.ComponentView) error {
	s.store.Replace(key, views)
	return nil
}

// Delete removes the component views for the given key.
func (s *ComponentStore) Delete(_ context.Context, key string) error {
	s.store.Delete(key)
	return nil
}

// List returns component views matching the given options.
func (s *ComponentStore) List(_ context.Context, opts domaincomponent.ListOptions) ([]domaincomponent.ComponentView, error) {
	all := s.store.All()

	if opts.PortalRef == "" && opts.Group == "" {
		return all, nil
	}

	filtered := make([]domaincomponent.ComponentView, 0, len(all))
	for _, v := range all {
		if opts.PortalRef != "" && v.PortalRef != opts.PortalRef {
			continue
		}
		if opts.Group != "" && v.Group != opts.Group {
			continue
		}
		filtered = append(filtered, v)
	}

	return filtered, nil
}

// Subscribe returns a channel that is closed on the next mutation.
func (s *ComponentStore) Subscribe() <-chan struct{} {
	return s.store.Subscribe()
}
