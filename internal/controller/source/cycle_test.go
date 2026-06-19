package source_test

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	srccontrol "github.com/golgoth31/sreportal/internal/controller/source"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/metrics"
	rsource "github.com/golgoth31/sreportal/internal/readstore/source"
	cprec "github.com/golgoth31/sreportal/internal/source/crossplanescalewayrecord"
	"github.com/golgoth31/sreportal/internal/source/externaldns"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

const (
	tTeamA = "team-a"
	tLBIP  = "1.2.3.4"
)

// crossKind is the only kind still served by the hand-rolled resolver path
// (everything external-dns handles natively goes through the Provider). These
// tests exercise the generic Cycle resolver mechanics via this kind.
var crossKind = cprec.SourceTypeCrossplaneScalewayRecord

// fakeResolver is a registry.Resolver for the crossplane kind: it lists
// Services (so the CRD-absent path can be simulated via a ServiceList) and
// resolves each object to "<name>.example.com", failing for names in failNames.
type fakeResolver struct {
	resolveErr error
	failNames  map[string]bool
}

func (r *fakeResolver) Type() registry.SourceType     { return crossKind }
func (r *fakeResolver) ObjectList() client.ObjectList { return &corev1.ServiceList{} }
func (r *fakeResolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	if r.failNames[obj.GetName()] {
		return nil, r.resolveErr
	}
	return []*endpoint.Endpoint{
		endpoint.NewEndpoint(obj.GetName()+".example.com", endpoint.RecordTypeA, tLBIP),
	}, nil
}

// crossDNS builds a DNS CR (namespace ns) that enables only the crossplane kind.
func crossDNS(name, ns string) *sreportalv1alpha2.DNS {
	return &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: ns,
			Sources: sreportalv1alpha2.SourcesSpec{
				CrossplaneScalewayRecord: &sreportalv1alpha2.CrossplaneScalewayRecordSourceSpec{Enabled: true},
			},
		},
	}
}

func TestCycle_ProducesEndpoints(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo", Namespace: tTeamA}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(crossDNS("d", tTeamA), svc).Build()
	reg := registry.NewRegistry(&fakeResolver{})
	store := rsource.NewStore()

	prev := srccontrol.Cycle(context.Background(), c, reg, nil, store, nil)
	require.NotEmpty(t, prev)
	got, err := store.Lookup(crossKind, tTeamA, "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "echo.example.com", got[0].Endpoint.DNSName)
}

// TestCycle_SetsResourceLabelWhenResolverOmitsIt verifies that Cycle fills in
// the external-dns "resource" label (kind/namespace/name) for resolvers that
// don't set it themselves — required for DNSRecordEntry.OriginRef. See PR #291.
func TestCycle_SetsResourceLabelWhenResolverOmitsIt(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo", Namespace: tTeamA}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(crossDNS("d", tTeamA), svc).Build()
	reg := registry.NewRegistry(&fakeResolver{})
	store := rsource.NewStore()

	_ = srccontrol.Cycle(context.Background(), c, reg, nil, store, nil)
	got, err := store.Lookup(crossKind, tTeamA, "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, string(crossKind)+"/team-a/echo", got[0].Endpoint.Labels[endpoint.ResourceLabelKey],
		"resource label must be set so DNSRecordEntry.OriginRef is populated")
}

func TestCycle_DeletesKindsNoLongerEnabled(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := registry.NewRegistry(&fakeResolver{})
	store := rsource.NewStore()
	store.ReplaceKind(crossKind, []domainsource.EnrichedEndpoint{
		{Kind: crossKind, Namespace: "x"},
	})
	prev := map[registry.SourceType]bool{crossKind: true}
	_ = srccontrol.Cycle(context.Background(), c, reg, nil, store, prev)
	got, _ := store.Lookup(crossKind, "", "")
	require.Empty(t, got)
}

// TestCycle_RemoteDNSSkipped verifies that a DNS CR with IsRemote=true is
// excluded, so its source kinds do NOT drive the cycle even when enabled.
func TestCycle_RemoteDNSSkipped(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	localDNS := crossDNS("local", tTeamA)
	// remote DNS: ingress enabled — must be ignored entirely.
	remoteDNS := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "remote", Namespace: "team-b"},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: "team-b",
			IsRemote:  true,
			Sources: sreportalv1alpha2.SourcesSpec{
				Ingress: &sreportalv1alpha2.IngressSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
				},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(localDNS, remoteDNS).Build()
	reg := registry.NewRegistry(&fakeResolver{})
	store := rsource.NewStore()

	next := srccontrol.Cycle(context.Background(), c, reg, nil, store, nil)

	require.True(t, next[crossKind], "crossplane kind must be enabled from local DNS")
	require.False(t, next[externaldns.KindIngress], "ingress kind from remote DNS must NOT be enabled")
}

// noMatchErrClient wraps a fake client and returns a meta.NoKindMatchError for
// List calls targeting *corev1.ServiceList, simulating a CRD not being installed.
type noMatchErrClient struct {
	client.Client
}

func (c *noMatchErrClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*corev1.ServiceList); ok {
		return &apimeta.NoKindMatchError{
			GroupKind:        schema.GroupKind{Group: "", Kind: "Service"},
			SearchedVersions: []string{"v1"},
		}
	}
	return c.Client.List(ctx, list, opts...)
}

// TestCycle_NoMatchError_CRDNotInstalled verifies that when List returns a
// meta.NoKindMatchError (CRD not installed), the cycle:
//   - does NOT call store.ReplaceKind or store.DeleteKind for that kind,
//   - preserves previously cached state,
//   - sets SourceKindActive=0 for the affected kind.
func TestCycle_NoMatchError_CRDNotInstalled(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	inner := fake.NewClientBuilder().WithScheme(scheme).WithObjects(crossDNS("d", "ns")).Build()
	c := &noMatchErrClient{Client: inner}

	reg := registry.NewRegistry(&fakeResolver{})
	store := rsource.NewStore()

	// Pre-populate cache with a known entry so we can verify it is NOT wiped.
	store.ReplaceKind(crossKind, []domainsource.EnrichedEndpoint{
		{Kind: crossKind, Namespace: "ns", Name: "cached"},
	})

	// Set metric to a sentinel value so the Set(0) call is observable.
	metrics.SourceKindActive.WithLabelValues(string(crossKind)).Set(99)

	prev := map[registry.SourceType]bool{crossKind: true}
	_ = srccontrol.Cycle(context.Background(), c, reg, nil, store, prev)

	got, err := store.Lookup(crossKind, "ns", "")
	require.NoError(t, err)
	require.Len(t, got, 1, "cached entry must be preserved when CRD is not installed")
	require.Equal(t, "cached", got[0].Name)

	active := testutil.ToFloat64(metrics.SourceKindActive.WithLabelValues(string(crossKind)))
	require.Equal(t, float64(0), active, "SourceKindActive must be 0 when CRD is not installed")
}

// TestCycle_PartialResolveError verifies that when some objects fail
// ResolveObject but others succeed: survivors are stored, SourceErrorsTotal
// increments per failure, and SourceKindActive stays 1.
func TestCycle_PartialResolveError(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	goodSvc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "good", Namespace: "ns"}}
	badSvc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "ns"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(crossDNS("d", "ns"), goodSvc, badSvc).Build()

	reg := registry.NewRegistry(&fakeResolver{
		resolveErr: errors.New("resolve failed"),
		failNames:  map[string]bool{"bad": true},
	})
	store := rsource.NewStore()

	metrics.SourceErrorsTotal.Reset()
	metrics.SourceKindActive.Reset()

	_ = srccontrol.Cycle(context.Background(), c, reg, nil, store, nil)

	got, err := store.Lookup(crossKind, "ns", "")
	require.NoError(t, err)
	require.Len(t, got, 1, "only the good object should be in the store")
	require.Equal(t, "good", got[0].Name)

	errCount := testutil.ToFloat64(metrics.SourceErrorsTotal.WithLabelValues(string(crossKind)))
	require.Equal(t, float64(1), errCount)

	active := testutil.ToFloat64(metrics.SourceKindActive.WithLabelValues(string(crossKind)))
	require.Equal(t, float64(1), active)
}

// TestCycle_AllObjectsFailResolve verifies the atomic-wipe guard: when every
// object fails ResolveObject, store.ReplaceKind is NOT called, the previously
// cached state is preserved, and the error counter increments per failure.
func TestCycle_AllObjectsFailResolve(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	svc1 := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc1", Namespace: "ns"}}
	svc2 := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc2", Namespace: "ns"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(crossDNS("d", "ns"), svc1, svc2).Build()

	reg := registry.NewRegistry(&fakeResolver{
		resolveErr: errors.New("always fail"),
		failNames:  map[string]bool{"svc1": true, "svc2": true},
	})
	store := rsource.NewStore()

	store.ReplaceKind(crossKind, []domainsource.EnrichedEndpoint{
		{Kind: crossKind, Namespace: "ns", Name: "previously-good"},
	})

	metrics.SourceErrorsTotal.Reset()
	metrics.SourceKindActive.Reset()

	_ = srccontrol.Cycle(context.Background(), c, reg, nil, store, nil)

	got, err := store.Lookup(crossKind, "ns", "")
	require.NoError(t, err)
	require.Len(t, got, 1, "previously cached state must be preserved when all objects fail")
	require.Equal(t, "previously-good", got[0].Name)

	errCount := testutil.ToFloat64(metrics.SourceErrorsTotal.WithLabelValues(string(crossKind)))
	require.Equal(t, float64(2), errCount)
}
