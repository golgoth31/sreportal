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
	store          *readstore.Store[domaindns.FQDNView]
	sourcePriority []string
}

// NewFQDNStore creates a new empty FQDNStore. sourcePriority defines the order
// used for cross-source deduplication at query time (e.g. ["service", "ingress"]).
// When nil or empty, no source-priority deduplication is performed.
func NewFQDNStore(sourcePriority []string) *FQDNStore {
	return &FQDNStore{
		store:          readstore.New[domaindns.FQDNView](),
		sourcePriority: sourcePriority,
	}
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
// When sourcePriority is configured, FQDNs with the same (Name, RecordType) from
// different sources are deduplicated, keeping only the highest-priority source.
func (s *FQDNStore) List(_ context.Context, filters domaindns.FQDNFilters) ([]domaindns.FQDNView, error) {
	all := s.store.All()
	filtered := applyFilters(all, filters)
	filtered = deduplicateBySourcePriority(filtered, s.sourcePriority)
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
	filtered := applyFilters(all, filters)
	filtered = deduplicateBySourcePriority(filtered, s.sourcePriority)
	return len(filtered), nil
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

// deduplicateBySourcePriority removes duplicate FQDNs (same Name+RecordType)
// that come from different sources, keeping only the one from the highest-priority
// source. Groups are merged across duplicates. Manual and remote sources are never
// deduplicated against external-dns sources (they use a different Source value).
// When priority is nil or empty, no deduplication is performed.
func deduplicateBySourcePriority(views []domaindns.FQDNView, priority []string) []domaindns.FQDNView {
	if len(priority) == 0 || len(views) == 0 {
		return views
	}

	// Build rank map: lower index = higher priority.
	rank := make(map[string]int, len(priority))
	for i, src := range priority {
		rank[src] = i
	}

	// sourceRank returns the priority rank based on SourceType.
	// Non-external-dns sources (manual, remote) are excluded from dedup by
	// assigning them a rank above any possible external-dns rank, so two entries
	// with that sentinel rank are never considered duplicates of each other.
	unlistedRank := len(priority)     // external-dns sources not in the priority list
	excludedRank := len(priority) + 1 // manual/remote — never deduplicated

	sourceRank := func(v domaindns.FQDNView) int {
		if v.Source != domaindns.SourceExternalDNS {
			return excludedRank
		}
		if r, ok := rank[v.SourceType]; ok {
			return r
		}
		return unlistedRank
	}

	type dedupKey = string
	seen := make(map[dedupKey]int) // key → index in result
	result := make([]domaindns.FQDNView, 0, len(views))

	for _, v := range views {
		key := v.Name + "/" + v.RecordType
		newRank := sourceRank(v)

		if idx, exists := seen[key]; exists {
			existing := &result[idx]
			existingRank := sourceRank(*existing)

			// Never deduplicate when either side is a non-external-dns source.
			if existingRank == excludedRank || newRank == excludedRank {
				uniqueKey := key + "/" + string(v.Source) + "/" + v.SourceType
				if _, alreadySeen := seen[uniqueKey]; !alreadySeen {
					seen[uniqueKey] = len(result)
					result = append(result, v)
				}
				continue
			}

			if newRank < existingRank {
				// New view has higher priority — replace but merge groups.
				mergedGroups := mergeGroups(existing.Groups, v.Groups)
				result[idx] = v
				result[idx].Groups = mergedGroups
			} else {
				// Existing wins — just merge groups from the new one.
				existing.Groups = mergeGroups(existing.Groups, v.Groups)
			}
		} else {
			seen[key] = len(result)
			result = append(result, v)
		}
	}

	return result
}

// mergeGroups merges two group slices, removing duplicates.
func mergeGroups(a, b []string) []string {
	merged := make([]string, len(a))
	copy(merged, a)
	for _, g := range b {
		if !slices.Contains(merged, g) {
			merged = append(merged, g)
		}
	}
	return merged
}
