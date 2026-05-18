package registry

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"
)

// SourceType identifies a kind of source object handled by a Resolver
// (e.g. "Service", "Ingress", "DNSEndpoint").
type SourceType string

// Resolver converts a single source object pulled from the controller-runtime
// cache into zero or more external-dns Endpoints. Filtering (namespace, labels)
// is the responsibility of the read-side (DNSReconciler).
type Resolver interface {
	Type() SourceType
	// ObjectList returns a fresh empty typed list suitable for cache.List.
	ObjectList() client.ObjectList
	// ResolveObject converts a single source object into zero or more Endpoints.
	ResolveObject(ctx context.Context, obj client.Object) ([]*endpoint.Endpoint, error)
}
