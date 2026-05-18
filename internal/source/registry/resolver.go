package registry

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"
)

// UnexpectedObjectTypeError is returned by a Resolver when ResolveObject is
// called with an object whose dynamic type does not match the Resolver's
// declared kind. This indicates a wiring bug (the registry handed the wrong
// type to the wrong resolver) and must surface — not be silenced as an empty
// result.
type UnexpectedObjectTypeError struct {
	Resolver SourceType
	Got      client.Object
}

func (e *UnexpectedObjectTypeError) Error() string {
	return fmt.Sprintf("resolver %s: unexpected object type %T", e.Resolver, e.Got)
}

// UnexpectedObjectType is the canonical constructor used by every Resolver
// implementation in this package. Keeps call sites short and avoids forcing
// each resolver to import "fmt".
func UnexpectedObjectType(resolver SourceType, got client.Object) error {
	return &UnexpectedObjectTypeError{Resolver: resolver, Got: got}
}

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
