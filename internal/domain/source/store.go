// Package source defines the domain interfaces and DTOs for the
// in-memory store of cluster-wide source endpoints, consumed by the
// DNSReconciler.
package source

import (
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

// EnrichedEndpoint is an external-dns Endpoint annotated with the provenance
// of the source object that produced it. SourceLabels and SourceAnnotations
// carry the K8s metadata of the source object so consumers (DNS labelFilter,
// Components reconciler, etc.) can route or filter without touching the
// apiserver.
type EnrichedEndpoint struct {
	Endpoint          *endpoint.Endpoint
	Kind              registry.SourceType
	Namespace         string // "" for cluster-scoped sources
	Name              string
	SourceLabels      map[string]string
	SourceAnnotations map[string]string
}

// SourceEndpointReader is the read-side contract for the in-memory store.
// Returned slices are snapshot copies; callers may mutate freely.
type SourceEndpointReader interface {
	// Lookup returns enriched endpoints for the given kind, filtered by
	// namespace ("" = all namespaces) and labelFilter ("" = match-all,
	// labels.Selector syntax). An invalid labelFilter returns an error.
	Lookup(kind registry.SourceType, namespace, labelFilter string) ([]EnrichedEndpoint, error)
	// Ready reports whether the producer has applied at least one successful
	// collection for kind (i.e. ReplaceKind has been called). The read side uses
	// it to avoid purging persisted DNSRecords for a kind whose source has not
	// synced yet — e.g. right after a controller restart, when the in-memory
	// store is empty but the DNSRecord CRs still exist.
	Ready(kind registry.SourceType) bool
	// Kinds returns the source kinds currently present in the store (those a
	// producer has collected at least once), in deterministic order. Consumers
	// enumerate kinds from here rather than from a resolver registry, so the
	// read side is agnostic to how a kind is discovered (native external-dns or
	// hand-rolled resolver).
	Kinds() []registry.SourceType
}

// SourceEndpointWriter is the write-side contract, used by the
// SourceReconciler each polling cycle.
type SourceEndpointWriter interface {
	// ReplaceKind atomically swaps all entries for a kind.
	ReplaceKind(kind registry.SourceType, entries []EnrichedEndpoint)
	// DeleteKind removes all entries for a kind (used when the kind becomes
	// unused cluster-wide).
	DeleteKind(kind registry.SourceType)
	// CountKind returns the number of entries currently stored for a kind.
	// Used by the producer's anti-collapse guard to compare a freshly collected
	// count against the last good state before overwriting it.
	CountKind(kind registry.SourceType) int
}
