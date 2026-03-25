// Package dns provides the in-memory FQDNStore implementation backed by the
// generic readstore.Store. It implements both dns.FQDNReader and dns.FQDNWriter.
package dns

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/readstore"
)

// FQDNStore is the in-memory implementation of dns.FQDNReader and dns.FQDNWriter.
type FQDNStore struct {
	store *readstore.Store[domaindns.FQDNView]
}

// NewFQDNStore creates a new empty FQDNStore.
func NewFQDNStore() *FQDNStore {
	return &FQDNStore{store: readstore.New[domaindns.FQDNView]()}
}

// compile-time interface checks
var (
	_ domaindns.FQDNReader = (*FQDNStore)(nil)
	_ domaindns.FQDNWriter = (*FQDNStore)(nil)
)

// Replace atomically replaces all FQDNs for a given DNS resource key.
func (s *FQDNStore) Replace(_ context.Context, resourceKey string, fqdns []domaindns.FQDNView) error {
	s.store.Replace(resourceKey, fqdns)
	return nil
}

// Delete removes all FQDNs for a given DNS resource key.
func (s *FQDNStore) Delete(_ context.Context, resourceKey string) error {
	s.store.Delete(resourceKey)
	return nil
}

// List returns FQDNs matching the given filters, sorted by (Name, RecordType).
func (s *FQDNStore) List(_ context.Context, filters domaindns.FQDNFilters) ([]domaindns.FQDNView, error) {
	all := s.store.All()
	filtered := applyFilters(all, filters)
	sortFQDNViews(filtered)
	return filtered, nil
}

// Get returns a single FQDN by exact name and record type.
// Name matching is case-insensitive. If recordType is empty, the first match by name is returned.
func (s *FQDNStore) Get(_ context.Context, name, recordType string) (domaindns.FQDNView, error) {
	nameLower := strings.ToLower(name)
	all := s.store.All()

	for _, f := range all {
		if strings.ToLower(f.Name) != nameLower {
			continue
		}
		if recordType == "" || f.RecordType == recordType {
			return f, nil
		}
	}

	return domaindns.FQDNView{}, fmt.Errorf("%w: %s/%s", domaindns.ErrFQDNNotFound, name, recordType)
}

// Count returns the number of FQDNs matching the given filters.
func (s *FQDNStore) Count(_ context.Context, filters domaindns.FQDNFilters) (int, error) {
	all := s.store.All()
	return len(applyFilters(all, filters)), nil
}

// Subscribe returns a channel closed on the next store mutation.
func (s *FQDNStore) Subscribe() <-chan struct{} {
	return s.store.Subscribe()
}

func applyFilters(fqdns []domaindns.FQDNView, f domaindns.FQDNFilters) []domaindns.FQDNView {
	if f.Portal == "" && f.Namespace == "" && f.Source == "" && f.Search == "" {
		return fqdns
	}

	searchLower := strings.ToLower(f.Search)
	out := make([]domaindns.FQDNView, 0, len(fqdns))

	for _, fqdn := range fqdns {
		if f.Portal != "" && fqdn.PortalName != f.Portal {
			continue
		}
		if f.Namespace != "" && fqdn.Namespace != f.Namespace {
			continue
		}
		if f.Source != "" && string(fqdn.Source) != f.Source {
			continue
		}
		if f.Search != "" && !strings.Contains(strings.ToLower(fqdn.Name), searchLower) {
			continue
		}
		out = append(out, fqdn)
	}

	return out
}

func sortFQDNViews(fqdns []domaindns.FQDNView) {
	slices.SortFunc(fqdns, func(a, b domaindns.FQDNView) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return cmp.Compare(a.RecordType, b.RecordType)
	})
}
