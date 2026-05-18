package registry

// Registry aggregates all Resolvers known to the operator.
type Registry struct {
	byKind map[SourceType]Resolver
}

// NewRegistry constructs a Registry from the given Resolvers. A duplicate
// Type() panics — registration is a static, startup-only operation.
func NewRegistry(resolvers ...Resolver) *Registry {
	r := &Registry{byKind: make(map[SourceType]Resolver, len(resolvers))}
	for _, res := range resolvers {
		if _, dup := r.byKind[res.Type()]; dup {
			panic("duplicate Resolver registered for kind " + string(res.Type()))
		}
		r.byKind[res.Type()] = res
	}
	return r
}

// Get returns the Resolver for kind, or false if none is registered.
func (r *Registry) Get(kind SourceType) (Resolver, bool) {
	res, ok := r.byKind[kind]
	return res, ok
}

// Resolvers returns all registered Resolvers in deterministic order by
// SourceType string.
func (r *Registry) Resolvers() []Resolver {
	keys := make([]SourceType, 0, len(r.byKind))
	for k := range r.byKind {
		keys = append(keys, k)
	}
	// stable order
	sortSourceTypes(keys)
	out := make([]Resolver, 0, len(keys))
	for _, k := range keys {
		out = append(out, r.byKind[k])
	}
	return out
}

func sortSourceTypes(s []SourceType) {
	// simple insertion sort (N≤11)
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
