// Package source provides the in-memory SourceEndpointStore.
// It implements both domain.source.SourceEndpointReader and SourceEndpointWriter.
package source

import (
	"maps"
	"sync"

	"k8s.io/apimachinery/pkg/labels"

	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// Store indexes EnrichedEndpoints by (SourceType, Namespace).
type Store struct {
	mu     sync.RWMutex
	byKind map[registry.SourceType]map[string][]domainsource.EnrichedEndpoint
	// ready records kinds for which ReplaceKind has succeeded at least once, so
	// the read side can tell "authoritatively empty" from "not synced yet".
	ready map[registry.SourceType]bool
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{
		byKind: map[registry.SourceType]map[string][]domainsource.EnrichedEndpoint{},
		ready:  map[registry.SourceType]bool{},
	}
}

// compile-time interface checks
var (
	_ domainsource.SourceEndpointReader = (*Store)(nil)
	_ domainsource.SourceEndpointWriter = (*Store)(nil)
)

// ReplaceKind atomically swaps all entries for a kind.
func (s *Store) ReplaceKind(kind registry.SourceType, entries []domainsource.EnrichedEndpoint) {
	byNs := map[string][]domainsource.EnrichedEndpoint{}
	for _, e := range entries {
		byNs[e.Namespace] = append(byNs[e.Namespace], e)
	}
	s.mu.Lock()
	s.byKind[kind] = byNs
	s.ready[kind] = true
	s.mu.Unlock()
}

// DeleteKind removes all entries for a kind.
func (s *Store) DeleteKind(kind registry.SourceType) {
	s.mu.Lock()
	delete(s.byKind, kind)
	delete(s.ready, kind)
	s.mu.Unlock()
}

// Ready reports whether ReplaceKind has succeeded at least once for kind.
func (s *Store) Ready(kind registry.SourceType) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready[kind]
}

// CountKind returns the total number of entries stored for a kind across all
// namespaces (0 if the kind is absent).
func (s *Store) CountKind(kind registry.SourceType) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, bucket := range s.byKind[kind] {
		n += len(bucket)
	}
	return n
}

// Lookup returns enriched endpoints for kind/namespace/labelFilter.
// namespace "" matches all namespaces; labelFilter "" matches all labels.
func (s *Store) Lookup(kind registry.SourceType, namespace, labelFilter string) ([]domainsource.EnrichedEndpoint, error) {
	sel := labels.Everything()
	if labelFilter != "" {
		parsed, err := labels.Parse(labelFilter)
		if err != nil {
			return nil, err
		}
		sel = parsed
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	byNs, ok := s.byKind[kind]
	if !ok {
		return nil, nil
	}

	pickBucket := func(ns string) []domainsource.EnrichedEndpoint { return byNs[ns] }
	var pool []domainsource.EnrichedEndpoint
	if namespace != "" {
		pool = pickBucket(namespace)
	} else {
		for _, bucket := range byNs {
			pool = append(pool, bucket...)
		}
	}

	out := make([]domainsource.EnrichedEndpoint, 0, len(pool))
	for _, e := range pool {
		if !sel.Matches(labels.Set(e.SourceLabels)) {
			continue
		}
		out = append(out, cloneEnrichedEndpoint(e))
	}
	return out, nil
}

// cloneEnrichedEndpoint returns a copy of e with SourceLabels and
// SourceAnnotations deep-cloned so callers may mutate them without affecting
// store state. The *endpoint.Endpoint pointer is shared — Endpoint is an
// external-dns DTO treated as read-only.
func cloneEnrichedEndpoint(e domainsource.EnrichedEndpoint) domainsource.EnrichedEndpoint {
	if e.SourceLabels != nil {
		labelsCopy := make(map[string]string, len(e.SourceLabels))
		maps.Copy(labelsCopy, e.SourceLabels)
		e.SourceLabels = labelsCopy
	}
	if e.SourceAnnotations != nil {
		anns := make(map[string]string, len(e.SourceAnnotations))
		maps.Copy(anns, e.SourceAnnotations)
		e.SourceAnnotations = anns
	}
	return e
}
