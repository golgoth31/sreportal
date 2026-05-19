package source_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/external-dns/endpoint"

	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	rsource "github.com/golgoth31/sreportal/internal/readstore/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

const (
	kindSvc   registry.SourceType = "Service"
	kindIng   registry.SourceType = "Ingress"
	ns1                           = "ns1"
	ns2                           = "ns2"
	labelTeam                     = "team"
)

func ep(name string) *endpoint.Endpoint {
	return endpoint.NewEndpoint(name, endpoint.RecordTypeA, "1.2.3.4")
}

func TestStore_ReplaceAndLookupByNamespace(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{
		{Endpoint: ep("a.example.com"), Kind: kindSvc, Namespace: ns1, Name: "a", SourceLabels: map[string]string{labelTeam: "x"}},
		{Endpoint: ep("b.example.com"), Kind: kindSvc, Namespace: ns2, Name: "b", SourceLabels: map[string]string{labelTeam: "y"}},
	})
	got, err := s.Lookup(kindSvc, ns1, "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "a.example.com", got[0].Endpoint.DNSName)
}

func TestStore_LookupAllNamespaces(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{
		{Endpoint: ep("a.example.com"), Namespace: ns1},
		{Endpoint: ep("b.example.com"), Namespace: ns2},
	})
	got, err := s.Lookup(kindSvc, "", "")
	require.NoError(t, err)
	require.Len(t, got, 2)
}

func TestStore_LookupLabelFilter(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{
		{Endpoint: ep("a.example.com"), Namespace: ns1, SourceLabels: map[string]string{labelTeam: "x"}},
		{Endpoint: ep("b.example.com"), Namespace: ns1, SourceLabels: map[string]string{labelTeam: "y"}},
	})
	got, err := s.Lookup(kindSvc, ns1, "team=x")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "a.example.com", got[0].Endpoint.DNSName)
}

func TestStore_LookupInvalidLabelFilter(t *testing.T) {
	s := rsource.NewStore()
	_, err := s.Lookup(kindSvc, "", "==invalid")
	assert.Error(t, err)
}

func TestStore_LookupUnknownKind(t *testing.T) {
	s := rsource.NewStore()
	got, err := s.Lookup(kindIng, "", "")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestStore_ReplaceKind_Atomicity(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{{Endpoint: ep("old.example.com"), Namespace: ns1}})
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{{Endpoint: ep("new.example.com"), Namespace: ns1}})
	got, _ := s.Lookup(kindSvc, ns1, "")
	require.Len(t, got, 1)
	assert.Equal(t, "new.example.com", got[0].Endpoint.DNSName)
}

func TestStore_DeleteKind(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{{Endpoint: ep("a.example.com"), Namespace: ns1}})
	s.DeleteKind(kindSvc)
	got, _ := s.Lookup(kindSvc, ns1, "")
	assert.Empty(t, got)
}

func TestStore_ReplaceKind_ClearsStaleNamespaces(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{
		{Endpoint: ep("a.example.com"), Namespace: ns1},
		{Endpoint: ep("b.example.com"), Namespace: ns2},
	})
	// Replace with entries that no longer mention ns2.
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{
		{Endpoint: ep("c.example.com"), Namespace: ns1},
	})
	got, _ := s.Lookup(kindSvc, ns2, "")
	assert.Empty(t, got, "stale namespace ns2 should be cleared after replace")
	got, _ = s.Lookup(kindSvc, "", "")
	require.Len(t, got, 1)
}

func TestStore_KindIsolation(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{{Endpoint: ep("svc.example.com"), Namespace: ns1}})
	s.ReplaceKind(kindIng, []domainsource.EnrichedEndpoint{{Endpoint: ep("ing.example.com"), Namespace: ns1}})
	gotSvc, _ := s.Lookup(kindSvc, "", "")
	gotIng, _ := s.Lookup(kindIng, "", "")
	require.Len(t, gotSvc, 1)
	require.Len(t, gotIng, 1)
	assert.Equal(t, "svc.example.com", gotSvc[0].Endpoint.DNSName)
	assert.Equal(t, "ing.example.com", gotIng[0].Endpoint.DNSName)
	s.DeleteKind(kindSvc)
	gotIng, _ = s.Lookup(kindIng, "", "")
	require.Len(t, gotIng, 1, "deleting one kind must not affect another")
}

func TestStore_LookupReturnsSnapshotCopy_SourceLabels(t *testing.T) {
	s := rsource.NewStore()
	s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{
		{Endpoint: ep("a.example.com"), Namespace: ns1, SourceLabels: map[string]string{labelTeam: "x"}},
	})
	got, _ := s.Lookup(kindSvc, ns1, "")
	require.Len(t, got, 1)
	// Mutate returned SourceLabels; must not affect store state.
	got[0].SourceLabels[labelTeam] = "TAMPERED"
	got[0].SourceLabels["new"] = "extra"
	got2, _ := s.Lookup(kindSvc, ns1, "")
	require.Len(t, got2, 1)
	assert.Equal(t, "x", got2[0].SourceLabels[labelTeam])
	_, ok := got2[0].SourceLabels["new"]
	assert.False(t, ok, "store must not retain caller mutations")
}

func TestStore_Concurrent_ReplaceAndLookup(t *testing.T) {
	s := rsource.NewStore()
	const writers, readers, iter = 4, 8, 200
	var wg sync.WaitGroup
	for range writers {
		wg.Go(func() {
			for range iter {
				s.ReplaceKind(kindSvc, []domainsource.EnrichedEndpoint{{Endpoint: ep("x.example.com"), Namespace: ns1}})
			}
		})
	}
	for range readers {
		wg.Go(func() {
			for range iter {
				_, _ = s.Lookup(kindSvc, ns1, "")
			}
		})
	}
	wg.Wait()
}
