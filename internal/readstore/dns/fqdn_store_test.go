package dns_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	dnsstore "github.com/golgoth31/sreportal/internal/readstore/dns"
)

func seedStore(t *testing.T) *dnsstore.FQDNStore {
	t.Helper()
	s := dnsstore.NewFQDNStore(nil)
	ctx := context.Background()

	err := s.Replace(ctx, "default/dns-main", []domaindns.FQDNView{
		{
			Name: "api.example.com", Source: domaindns.SourceExternalDNS,
			Groups: []string{"Services"}, RecordType: "A",
			Targets: []string{"10.0.0.1"}, LastSeen: time.Now(),
			PortalName: "main", Namespace: "default", SyncStatus: "synced",
		},
		{
			Name: "web.example.com", Source: domaindns.SourceExternalDNS,
			Groups: []string{"Services"}, RecordType: "A",
			Targets: []string{"10.0.0.2"}, LastSeen: time.Now(),
			PortalName: "main", Namespace: "default",
		},
	})
	require.NoError(t, err)

	err = s.Replace(ctx, "staging/dns-staging", []domaindns.FQDNView{
		{
			Name: "api.staging.com", Source: domaindns.SourceManual,
			Groups: []string{"Internal"}, RecordType: "CNAME",
			Targets: []string{"lb.staging.com"}, LastSeen: time.Now(),
			PortalName: "staging", Namespace: "staging",
		},
	})
	require.NoError(t, err)

	return s
}

func TestFQDNStore_List_ReturnsAllSorted(t *testing.T) {
	s := seedStore(t)

	fqdns, err := s.List(context.Background(), domaindns.FQDNFilters{})
	require.NoError(t, err)
	require.Len(t, fqdns, 3)

	// Sorted by (Name, RecordType)
	assert.Equal(t, "api.example.com", fqdns[0].Name)
	assert.Equal(t, "api.staging.com", fqdns[1].Name)
	assert.Equal(t, "web.example.com", fqdns[2].Name)
}

func TestFQDNStore_List_FiltersByPortal(t *testing.T) {
	s := seedStore(t)

	fqdns, err := s.List(context.Background(), domaindns.FQDNFilters{Portal: "main"})
	require.NoError(t, err)
	require.Len(t, fqdns, 2)

	for _, f := range fqdns {
		assert.Equal(t, "main", f.PortalName)
	}
}

func TestFQDNStore_List_FiltersByNamespace(t *testing.T) {
	s := seedStore(t)

	fqdns, err := s.List(context.Background(), domaindns.FQDNFilters{Namespace: "staging"})
	require.NoError(t, err)
	require.Len(t, fqdns, 1)
	assert.Equal(t, "api.staging.com", fqdns[0].Name)
}

func TestFQDNStore_List_FiltersBySource(t *testing.T) {
	s := seedStore(t)

	fqdns, err := s.List(context.Background(), domaindns.FQDNFilters{Source: "manual"})
	require.NoError(t, err)
	require.Len(t, fqdns, 1)
	assert.Equal(t, "api.staging.com", fqdns[0].Name)
}

func TestFQDNStore_List_FiltersBySearch_CaseInsensitive(t *testing.T) {
	s := seedStore(t)

	fqdns, err := s.List(context.Background(), domaindns.FQDNFilters{Search: "API"})
	require.NoError(t, err)
	require.Len(t, fqdns, 2)
	assert.Equal(t, "api.example.com", fqdns[0].Name)
	assert.Equal(t, "api.staging.com", fqdns[1].Name)
}

func TestFQDNStore_List_CombinesFilters(t *testing.T) {
	s := seedStore(t)

	fqdns, err := s.List(context.Background(), domaindns.FQDNFilters{
		Portal: "main",
		Search: "api",
	})
	require.NoError(t, err)
	require.Len(t, fqdns, 1)
	assert.Equal(t, "api.example.com", fqdns[0].Name)
}

func TestFQDNStore_Get_ReturnsExact(t *testing.T) {
	s := seedStore(t)

	fqdn, err := s.Get(context.Background(), "api.example.com", "A")
	require.NoError(t, err)
	assert.Equal(t, "api.example.com", fqdn.Name)
	assert.Equal(t, "A", fqdn.RecordType)
}

func TestFQDNStore_Get_ReturnsError_WhenNotFound(t *testing.T) {
	s := seedStore(t)

	_, err := s.Get(context.Background(), "nonexistent.com", "A")
	require.ErrorIs(t, err, domaindns.ErrFQDNNotFound)
}

func TestFQDNStore_Get_MatchesCaseInsensitive(t *testing.T) {
	s := seedStore(t)

	fqdn, err := s.Get(context.Background(), "API.EXAMPLE.COM", "A")
	require.NoError(t, err)
	assert.Equal(t, "api.example.com", fqdn.Name)
}

func TestFQDNStore_Count_WithFilters(t *testing.T) {
	s := seedStore(t)

	count, err := s.Count(context.Background(), domaindns.FQDNFilters{Portal: "main"})
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	count, err = s.Count(context.Background(), domaindns.FQDNFilters{})
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestFQDNStore_Replace_MergesMultipleResources(t *testing.T) {
	s := dnsstore.NewFQDNStore(nil)
	ctx := context.Background()

	_ = s.Replace(ctx, "ns1/dns1", []domaindns.FQDNView{{Name: "a.com", RecordType: "A"}})
	_ = s.Replace(ctx, "ns2/dns2", []domaindns.FQDNView{{Name: "b.com", RecordType: "A"}})

	fqdns, err := s.List(ctx, domaindns.FQDNFilters{})
	require.NoError(t, err)
	require.Len(t, fqdns, 2)
}

func TestFQDNStore_Delete_RemovesResource(t *testing.T) {
	s := seedStore(t)
	ctx := context.Background()

	err := s.Delete(ctx, "default/dns-main")
	require.NoError(t, err)

	fqdns, err := s.List(ctx, domaindns.FQDNFilters{})
	require.NoError(t, err)
	require.Len(t, fqdns, 1)
	assert.Equal(t, "api.staging.com", fqdns[0].Name)
}

func TestFQDNStore_Subscribe_NotifiesOnChange(t *testing.T) {
	s := dnsstore.NewFQDNStore(nil)

	ch := s.Subscribe()

	_ = s.Replace(context.Background(), "k", []domaindns.FQDNView{{Name: "x.com"}})

	select {
	case <-ch:
		// expected
	default:
		t.Fatal("expected notification after Replace")
	}
}

func TestFQDNStore_List_DeduplicatesBySourcePriority(t *testing.T) {
	// Priority: dnsendpoint > ingress > service
	s := dnsstore.NewFQDNStore([]string{"dnsendpoint", "ingress", "service"})
	ctx := context.Background()

	// Same FQDN from two different source types (service and ingress)
	_ = s.Replace(ctx, "ns/record-svc", []domaindns.FQDNView{
		{
			Name: "app.example.com", Source: domaindns.SourceExternalDNS,
			SourceType: "service", RecordType: "A",
			Targets: []string{"10.0.0.1"}, Groups: []string{"svc-group"},
			PortalName: "main", Namespace: "ns",
		},
	})
	_ = s.Replace(ctx, "ns/record-ing", []domaindns.FQDNView{
		{
			Name: "app.example.com", Source: domaindns.SourceExternalDNS,
			SourceType: "ingress", RecordType: "A",
			Targets: []string{"10.0.0.2"}, Groups: []string{"ing-group"},
			PortalName: "main", Namespace: "ns",
		},
	})

	fqdns, err := s.List(ctx, domaindns.FQDNFilters{})
	require.NoError(t, err)
	require.Len(t, fqdns, 1, "duplicate FQDN should be deduplicated")

	// ingress has higher priority than service → ingress targets win
	assert.Equal(t, []string{"10.0.0.2"}, fqdns[0].Targets)
	assert.Equal(t, "ingress", fqdns[0].SourceType)
	// Groups should be merged from both sources
	assert.ElementsMatch(t, []string{"ing-group", "svc-group"}, fqdns[0].Groups)
}

func TestFQDNStore_List_DeduplicatesBySourcePriority_ManualNeverDeduplicated(t *testing.T) {
	s := dnsstore.NewFQDNStore([]string{"dnsendpoint", "service"})
	ctx := context.Background()

	// Same FQDN from manual and external-dns
	_ = s.Replace(ctx, "ns/record-svc", []domaindns.FQDNView{
		{
			Name: "app.example.com", Source: domaindns.SourceExternalDNS,
			SourceType: "service", RecordType: "A",
			Targets: []string{"10.0.0.1"}, PortalName: "main", Namespace: "ns",
		},
	})
	_ = s.Replace(ctx, "ns/manual", []domaindns.FQDNView{
		{
			Name: "app.example.com", Source: domaindns.SourceManual,
			RecordType: "A",
			Targets:    []string{"10.0.0.99"}, PortalName: "main", Namespace: "ns",
		},
	})

	fqdns, err := s.List(ctx, domaindns.FQDNFilters{})
	require.NoError(t, err)
	// Manual sources should never be deduplicated against external-dns
	require.Len(t, fqdns, 2, "manual and external-dns should coexist")
}

func TestFQDNStore_List_DeduplicatesBySourcePriority_UnlistedSourceLowestPriority(t *testing.T) {
	// Only "ingress" is in priority list; "service" is not listed
	s := dnsstore.NewFQDNStore([]string{"ingress"})
	ctx := context.Background()

	_ = s.Replace(ctx, "ns/record-svc", []domaindns.FQDNView{
		{
			Name: "app.example.com", Source: domaindns.SourceExternalDNS,
			SourceType: "service", RecordType: "A",
			Targets: []string{"10.0.0.1"}, PortalName: "main", Namespace: "ns",
		},
	})
	_ = s.Replace(ctx, "ns/record-ing", []domaindns.FQDNView{
		{
			Name: "app.example.com", Source: domaindns.SourceExternalDNS,
			SourceType: "ingress", RecordType: "A",
			Targets: []string{"10.0.0.2"}, PortalName: "main", Namespace: "ns",
		},
	})

	fqdns, err := s.List(ctx, domaindns.FQDNFilters{})
	require.NoError(t, err)
	require.Len(t, fqdns, 1)
	// ingress is listed, service is not → ingress wins
	assert.Equal(t, []string{"10.0.0.2"}, fqdns[0].Targets)
	assert.Equal(t, "ingress", fqdns[0].SourceType)
}

func TestFQDNStore_Count_DeduplicatesBySourcePriority(t *testing.T) {
	s := dnsstore.NewFQDNStore([]string{"ingress", "service"})
	ctx := context.Background()

	_ = s.Replace(ctx, "ns/record-svc", []domaindns.FQDNView{
		{
			Name: "app.example.com", Source: domaindns.SourceExternalDNS,
			SourceType: "service", RecordType: "A",
			PortalName: "main", Namespace: "ns",
		},
	})
	_ = s.Replace(ctx, "ns/record-ing", []domaindns.FQDNView{
		{
			Name: "app.example.com", Source: domaindns.SourceExternalDNS,
			SourceType: "ingress", RecordType: "A",
			PortalName: "main", Namespace: "ns",
		},
	})

	count, err := s.Count(ctx, domaindns.FQDNFilters{})
	require.NoError(t, err)
	assert.Equal(t, 1, count, "duplicates should be deduplicated in count too")
}
