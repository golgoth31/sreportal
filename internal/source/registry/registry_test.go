package registry_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

type fakeResolver struct{ kind registry.SourceType }

func (f fakeResolver) Type() registry.SourceType   { return f.kind }
func (fakeResolver) ObjectList() client.ObjectList { return nil }
func (fakeResolver) ResolveObject(context.Context, client.Object) ([]*endpoint.Endpoint, error) {
	return nil, nil
}

func TestRegistry_GetAndResolvers(t *testing.T) {
	a := fakeResolver{kind: "A"}
	b := fakeResolver{kind: "B"}
	r := registry.NewRegistry(b, a) // intentionally out of order
	got, ok := r.Get("A")
	require.True(t, ok)
	assert.Equal(t, registry.SourceType("A"), got.Type())
	_, ok = r.Get("missing")
	assert.False(t, ok)
	all := r.Resolvers()
	require.Len(t, all, 2)
	assert.Equal(t, registry.SourceType("A"), all[0].Type())
	assert.Equal(t, registry.SourceType("B"), all[1].Type())
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	_ = registry.NewRegistry(fakeResolver{kind: "X"}, fakeResolver{kind: "X"})
}
