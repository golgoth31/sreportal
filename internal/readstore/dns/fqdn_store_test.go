package dns_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	dnsstore "github.com/golgoth31/sreportal/internal/readstore/dns"
)

func TestFQDNStore_ListReturnsIsolatedSlices(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	require.NoError(t, s.Replace(ctx, "ns/a", "p1", []domaindns.FQDNView{
		{Name: "iso.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}, Groups: []string{"g1"}},
	}))

	out, err := s.List(ctx, domaindns.FQDNFilters{})
	require.NoError(t, err)
	require.Len(t, out, 1)
	snapshotPortals := append([]string(nil), out[0].Portals...)

	require.NoError(t, s.Replace(ctx, "ns/b", "p2", []domaindns.FQDNView{
		{Name: "iso.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}, Groups: []string{"g2"}},
	}))

	assert.Equal(t, snapshotPortals, out[0].Portals, "caller's slice must not observe in-place mutation by a subsequent Replace")
	assert.Equal(t, []string{"g1"}, out[0].Groups, "Groups slice must be isolated from the store")
}

func TestFQDNStore_Constructs(t *testing.T) {
	s := dnsstore.NewFQDNStore()
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestFQDNStore_InsertsSingleFQDN(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	fqdns := []domaindns.FQDNView{
		{Name: "foo.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}, Portals: []string{"portal-x"}},
	}
	err := s.Replace(ctx, "ns/rec-a", "portal-x", fqdns)
	require.NoError(t, err)

	out, err := s.List(ctx, domaindns.FQDNFilters{Portal: "portal-x"})
	require.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "foo.example.com", out[0].Name)
	assert.Contains(t, out[0].Portals, "portal-x")
}

func TestFQDNStore_DedupSamePortal(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	view := domaindns.FQDNView{Name: "shared.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}, Portals: []string{"portal-x"}}

	err := s.Replace(ctx, "ns/rec-a", "portal-x", []domaindns.FQDNView{view})
	require.NoError(t, err)
	err = s.Replace(ctx, "ns/rec-b", "portal-x", []domaindns.FQDNView{view})
	require.NoError(t, err)

	out, err := s.List(ctx, domaindns.FQDNFilters{Portal: "portal-x"})
	require.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, []string{"portal-x"}, out[0].Portals)
}

func TestFQDNStore_DedupAcrossPortals(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	view := domaindns.FQDNView{Name: "multi.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}}

	err := s.Replace(ctx, "ns/rec-a", "portal-x", []domaindns.FQDNView{view})
	require.NoError(t, err)
	err = s.Replace(ctx, "ns/rec-b", "portal-y", []domaindns.FQDNView{view})
	require.NoError(t, err)

	out, err := s.List(ctx, domaindns.FQDNFilters{})
	require.NoError(t, err)
	assert.Len(t, out, 1)
	assert.ElementsMatch(t, []string{"portal-x", "portal-y"}, out[0].Portals)
}

func TestFQDNStore_MergesGroups(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	err := s.Replace(ctx, "ns/rec-a", "portal-x", []domaindns.FQDNView{
		{Name: "g.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}, Groups: []string{"team-a"}},
	})
	require.NoError(t, err)
	err = s.Replace(ctx, "ns/rec-b", "portal-y", []domaindns.FQDNView{
		{Name: "g.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}, Groups: []string{"team-b"}},
	})
	require.NoError(t, err)

	got, err := s.Get(ctx, "g.example.com", "A")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"team-a", "team-b"}, got.Groups)
}

func TestFQDNStore_PortalChangeRemovesOldIndex(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	view := domaindns.FQDNView{Name: "moves.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}}

	require.NoError(t, s.Replace(ctx, "ns/rec-a", "portal-x", []domaindns.FQDNView{view}))
	require.NoError(t, s.Replace(ctx, "ns/rec-a", "portal-y", []domaindns.FQDNView{view}))

	oldPortal, err := s.List(ctx, domaindns.FQDNFilters{Portal: "portal-x"})
	require.NoError(t, err)
	assert.Empty(t, oldPortal, "old portal index should be cleared after portalRef change")

	newPortal, err := s.List(ctx, domaindns.FQDNFilters{Portal: "portal-y"})
	require.NoError(t, err)
	assert.Len(t, newPortal, 1)
	assert.Equal(t, []string{"portal-y"}, newPortal[0].Portals)
}

func TestFQDNStore_ConflictKeepsFirstWriter(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	err := s.Replace(ctx, "ns/rec-a", "portal-x", []domaindns.FQDNView{
		{Name: "c.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}},
	})
	require.NoError(t, err)
	err = s.Replace(ctx, "ns/rec-b", "portal-y", []domaindns.FQDNView{
		{Name: "c.example.com", RecordType: "A", Targets: []string{"2.2.2.2"}},
	})
	require.NoError(t, err)

	got, err := s.Get(ctx, "c.example.com", "A")
	require.NoError(t, err)
	assert.Equal(t, []string{"1.1.1.1"}, got.Targets)

	s.AnnotateOwner("ns/rec-b", "ns", "dns-b")
	conflicts := s.Conflicts("ns", "dns-b")
	assert.Len(t, conflicts, 1)
	assert.Equal(t, "ns/rec-b", conflicts[0].LoserRecord)
}

func TestFQDNStore_ReplacePreservesOwnerAnnotation(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	// 1. rec-b writes first (becomes winner).
	require.NoError(t, s.Replace(ctx, "ns/rec-b", "p", []domaindns.FQDNView{
		{Name: "x.example.com", RecordType: "A", Targets: []string{"2.2.2.2"}},
	}))

	// 2. rec-a writes mismatching targets — generates the only conflict event,
	//    with rec-a as loser.
	require.NoError(t, s.Replace(ctx, "ns/rec-a", "p", []domaindns.FQDNView{
		{Name: "x.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}},
	}))

	// 3. Annotate rec-a's DNS owner.
	s.AnnotateOwner("ns/rec-a", "ns", "dns-a")

	// 4. Benign replay of rec-a with identical targets. No new conflict is
	//    emitted (targets match the winner is false — first writer wins, so
	//    rec-a's targets still mismatch). We expect the second conflict event
	//    here too, but the key invariant is that the owner annotation survives.
	require.NoError(t, s.Replace(ctx, "ns/rec-a", "p", []domaindns.FQDNView{
		{Name: "x.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}},
	}))

	// 5. Conflicts scoped to dns-a (the loser owner) must surface events.
	//    If Replace had dropped the owner annotation, this would be empty.
	conflicts := s.Conflicts("ns", "dns-a")
	assert.NotEmpty(t, conflicts, "owner annotation must be preserved across Replace replays")
	for _, c := range conflicts {
		assert.Equal(t, "ns/rec-a", c.LoserRecord)
	}
}

func TestFQDNStore_ConflictsScopedToDNSOwner(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	require.NoError(t, s.Replace(ctx, "ns/a", "p", []domaindns.FQDNView{
		{Name: "x.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}},
	}))
	s.AnnotateOwner("ns/a", "ns", "dns-a")

	require.NoError(t, s.Replace(ctx, "ns/b", "p", []domaindns.FQDNView{
		{Name: "x.example.com", RecordType: "A", Targets: []string{"2.2.2.2"}},
	}))
	s.AnnotateOwner("ns/b", "ns", "dns-b")

	assert.Len(t, s.Conflicts("ns", "dns-b"), 1, "loser dns-b should see the conflict")
	assert.Empty(t, s.Conflicts("ns", "dns-a"), "winner dns-a should not see itself in conflicts")
}

func TestFQDNStore_DeleteRemovesLastContributor(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	require.NoError(t, s.Replace(ctx, "ns/a", "p", []domaindns.FQDNView{
		{Name: "x.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}},
	}))

	require.NoError(t, s.Delete(ctx, "ns/a"))

	out, err := s.List(ctx, domaindns.FQDNFilters{})
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestFQDNStore_DeleteKeepsFQDNIfAnotherContributor(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	view := domaindns.FQDNView{Name: "x.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}}

	require.NoError(t, s.Replace(ctx, "ns/a", "p", []domaindns.FQDNView{view}))
	require.NoError(t, s.Replace(ctx, "ns/b", "p", []domaindns.FQDNView{view}))

	require.NoError(t, s.Delete(ctx, "ns/a"))

	out, err := s.List(ctx, domaindns.FQDNFilters{Portal: "p"})
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestFQDNStore_DeleteRemovesPortalWhenLastContributorDrops(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	view := domaindns.FQDNView{Name: "x.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}}

	require.NoError(t, s.Replace(ctx, "ns/a", "p1", []domaindns.FQDNView{view}))
	require.NoError(t, s.Replace(ctx, "ns/b", "p2", []domaindns.FQDNView{view}))

	require.NoError(t, s.Delete(ctx, "ns/a"))

	got, err := s.Get(ctx, "x.example.com", "A")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"p2"}, got.Portals)
}

func TestFQDNStore_ShrinkingReplaceRemovesOrphanedKeys(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	require.NoError(t, s.Replace(ctx, "ns/a", "p", []domaindns.FQDNView{
		{Name: "a.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}},
		{Name: "b.example.com", RecordType: "A", Targets: []string{"2.2.2.2"}},
	}))

	require.NoError(t, s.Replace(ctx, "ns/a", "p", []domaindns.FQDNView{
		{Name: "a.example.com", RecordType: "A", Targets: []string{"1.1.1.1"}},
	}))

	out, err := s.List(ctx, domaindns.FQDNFilters{Portal: "p"})
	require.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "a.example.com", out[0].Name)
}

// newPopulatedStore creates a store with two portal entries for Task 4.6 tests.
func newPopulatedStore(t *testing.T) (*dnsstore.FQDNStore, context.Context) {
	t.Helper()
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()
	require.NoError(t, s.Replace(ctx, "ns/a", "p1", []domaindns.FQDNView{
		{Name: "alpha.example.com", RecordType: "A", Namespace: "ns", Source: domaindns.SourceExternalDNS},
		{Name: "beta.example.com", RecordType: "A", Namespace: "ns", Source: domaindns.SourceExternalDNS},
	}))
	require.NoError(t, s.Replace(ctx, "ns/b", "p2", []domaindns.FQDNView{
		{Name: "gamma.example.com", RecordType: "A", Namespace: "ns", Source: domaindns.SourceManual},
	}))
	return s, ctx
}

func TestFQDNStore_ListFiltersByPortal(t *testing.T) {
	s, ctx := newPopulatedStore(t)

	out, err := s.List(ctx, domaindns.FQDNFilters{Portal: "p1"})
	require.NoError(t, err)
	assert.Len(t, out, 2)
	names := make([]string, len(out))
	for i, v := range out {
		names[i] = v.Name
	}
	assert.ElementsMatch(t, []string{"alpha.example.com", "beta.example.com"}, names)
}

func TestFQDNStore_ListSortedByNameThenRecordType(t *testing.T) {
	s, ctx := newPopulatedStore(t)

	out, err := s.List(ctx, domaindns.FQDNFilters{})
	require.NoError(t, err)
	assert.Len(t, out, 3)
	assert.Equal(t, "alpha.example.com", out[0].Name)
	assert.Equal(t, "beta.example.com", out[1].Name)
	assert.Equal(t, "gamma.example.com", out[2].Name)
}

func TestFQDNStore_CountMatchesListLength(t *testing.T) {
	s, ctx := newPopulatedStore(t)

	listed, err := s.List(ctx, domaindns.FQDNFilters{Portal: "p1"})
	require.NoError(t, err)

	count, err := s.Count(ctx, domaindns.FQDNFilters{Portal: "p1"})
	require.NoError(t, err)
	assert.Equal(t, len(listed), count)
}

func TestFQDNStore_GetReturnsErrFQDNNotFound(t *testing.T) {
	ctx := context.Background()
	s := dnsstore.NewFQDNStore()

	_, err := s.Get(ctx, "nope.example.com", "A")
	require.Error(t, err)
	assert.ErrorIs(t, err, domaindns.ErrFQDNNotFound)
}
