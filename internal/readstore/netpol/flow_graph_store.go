// Package netpol provides the in-memory FlowGraphStore implementation
// backed by the generic readstore.Store.
package netpol

import (
	"context"
	"strings"
	"sync"

	domainnetpol "github.com/golgoth31/sreportal/internal/domain/netpol"
	"github.com/golgoth31/sreportal/internal/readstore"
)

// FlowGraphStore is the in-memory implementation of FlowGraphReader and FlowGraphWriter.
// Keys are NFD names (e.g. "nfd-main"). Each key tracks its portalRef for portal filtering.
type FlowGraphStore struct {
	nodes *readstore.Store[domainnetpol.FlowNode]
	edges *readstore.Store[domainnetpol.FlowEdge]

	portalMu   sync.RWMutex
	portalRefs map[string]string // key → portalRef
}

// NewFlowGraphStore creates a new empty FlowGraphStore.
func NewFlowGraphStore() *FlowGraphStore {
	return &FlowGraphStore{
		nodes:      readstore.New[domainnetpol.FlowNode](),
		edges:      readstore.New[domainnetpol.FlowEdge](),
		portalRefs: make(map[string]string),
	}
}

// compile-time interface checks
var (
	_ domainnetpol.FlowGraphReader = (*FlowGraphStore)(nil)
	_ domainnetpol.FlowGraphWriter = (*FlowGraphStore)(nil)
)

// ReplaceNodes stores nodes for the given key and updates its portalRef.
func (s *FlowGraphStore) ReplaceNodes(_ context.Context, key, portalRef string, nodes []domainnetpol.FlowNode) error {
	s.portalMu.Lock()
	s.portalRefs[key] = portalRef
	s.portalMu.Unlock()

	s.nodes.Replace(key, nodes)

	return nil
}

// ReplaceEdges stores edges for the given key and updates its portalRef.
func (s *FlowGraphStore) ReplaceEdges(_ context.Context, key, portalRef string, edges []domainnetpol.FlowEdge) error {
	s.portalMu.Lock()
	s.portalRefs[key] = portalRef
	s.portalMu.Unlock()

	s.edges.Replace(key, edges)

	return nil
}

// Delete removes all nodes and edges for the given key.
func (s *FlowGraphStore) Delete(_ context.Context, key string) error {
	s.portalMu.Lock()
	delete(s.portalRefs, key)
	s.portalMu.Unlock()

	s.nodes.Delete(key)
	s.edges.Delete(key)

	return nil
}

// ListNodes returns a deduplicated, filtered list of nodes across all keys.
func (s *FlowGraphStore) ListNodes(_ context.Context, filters domainnetpol.FlowGraphFilters) ([]domainnetpol.FlowNode, error) {
	allowedKeys := s.allowedKeys(filters.Portal)
	nodeMap := s.mergedNodes(allowedKeys)

	if filters.Namespace != "" {
		for id, n := range nodeMap {
			if n.Namespace != filters.Namespace {
				delete(nodeMap, id)
			}
		}
	}

	if filters.Search != "" {
		edgeMap := s.mergedEdges(allowedKeys)
		filterBySearch(nodeMap, edgeMap, filters.Search)
	}

	nodes := make([]domainnetpol.FlowNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}

	return nodes, nil
}

// ListEdges returns a deduplicated, filtered list of edges across all keys.
// Edges referencing nodes not in the filtered node set are pruned.
func (s *FlowGraphStore) ListEdges(_ context.Context, filters domainnetpol.FlowGraphFilters) ([]domainnetpol.FlowEdge, error) {
	allowedKeys := s.allowedKeys(filters.Portal)
	nodeMap := s.mergedNodes(allowedKeys)
	edgeMap := s.mergedEdges(allowedKeys)

	if filters.Namespace != "" {
		for id, n := range nodeMap {
			if n.Namespace != filters.Namespace {
				delete(nodeMap, id)
			}
		}

		pruneOrphanEdges(nodeMap, edgeMap)
	}

	if filters.Search != "" {
		filterBySearch(nodeMap, edgeMap, filters.Search)
	}

	// Final prune to ensure edge consistency with the node set.
	pruneOrphanEdges(nodeMap, edgeMap)

	edges := make([]domainnetpol.FlowEdge, 0, len(edgeMap))
	for _, e := range edgeMap {
		edges = append(edges, e)
	}

	return edges, nil
}

// Subscribe returns a channel that is closed on the next mutation.
func (s *FlowGraphStore) Subscribe() <-chan struct{} {
	return s.nodes.Subscribe()
}

// allowedKeys returns the set of keys matching the portal filter.
// Returns nil when no portal filter is specified (all keys allowed).
func (s *FlowGraphStore) allowedKeys(portal string) map[string]struct{} {
	if portal == "" {
		return nil
	}

	s.portalMu.RLock()
	defer s.portalMu.RUnlock()

	allowed := make(map[string]struct{})
	for key, ref := range s.portalRefs {
		if ref == portal {
			allowed[key] = struct{}{}
		}
	}

	return allowed
}

// mergedNodes builds a deduplicated node map from allowed keys.
func (s *FlowGraphStore) mergedNodes(allowedKeys map[string]struct{}) map[string]domainnetpol.FlowNode {
	nodeMap := make(map[string]domainnetpol.FlowNode)

	for _, key := range s.nodes.Keys() {
		if allowedKeys != nil {
			if _, ok := allowedKeys[key]; !ok {
				continue
			}
		}

		for _, n := range s.nodes.Get(key) {
			if _, ok := nodeMap[n.ID]; !ok {
				nodeMap[n.ID] = n
			}
		}
	}

	return nodeMap
}

// mergedEdges builds a deduplicated edge map from allowed keys.
func (s *FlowGraphStore) mergedEdges(allowedKeys map[string]struct{}) map[string]domainnetpol.FlowEdge {
	edgeMap := make(map[string]domainnetpol.FlowEdge)

	for _, key := range s.edges.Keys() {
		if allowedKeys != nil {
			if _, ok := allowedKeys[key]; !ok {
				continue
			}
		}

		for _, e := range s.edges.Get(key) {
			edgeKey := e.From + "|" + e.To + "|" + e.EdgeType
			if existing, ok := edgeMap[edgeKey]; !ok {
				edgeMap[edgeKey] = e
			} else if e.LastSeen != nil && (existing.LastSeen == nil || e.LastSeen.After(*existing.LastSeen)) {
				edgeMap[edgeKey] = e
			}
		}
	}

	return edgeMap
}

// filterBySearch keeps only nodes matching the search term and their direct neighbors (1 hop).
func filterBySearch(nodeMap map[string]domainnetpol.FlowNode, edgeMap map[string]domainnetpol.FlowEdge, search string) {
	search = strings.ToLower(search)

	directMatch := make(map[string]bool)
	for id, n := range nodeMap {
		if strings.Contains(strings.ToLower(n.Label), search) ||
			strings.Contains(strings.ToLower(n.Group), search) ||
			strings.Contains(strings.ToLower(n.Namespace), search) {
			directMatch[id] = true
		}
	}

	matched := make(map[string]bool, len(directMatch))
	for id := range directMatch {
		matched[id] = true
	}

	for _, e := range edgeMap {
		if directMatch[e.From] {
			matched[e.To] = true
		}

		if directMatch[e.To] {
			matched[e.From] = true
		}
	}

	for id := range nodeMap {
		if !matched[id] {
			delete(nodeMap, id)
		}
	}

	pruneOrphanEdges(nodeMap, edgeMap)
}

// pruneOrphanEdges removes edges whose source or target node is not in the node map.
func pruneOrphanEdges(nodeMap map[string]domainnetpol.FlowNode, edgeMap map[string]domainnetpol.FlowEdge) {
	for key, e := range edgeMap {
		if _, ok := nodeMap[e.From]; !ok {
			delete(edgeMap, key)
			continue
		}

		if _, ok := nodeMap[e.To]; !ok {
			delete(edgeMap, key)
		}
	}
}
