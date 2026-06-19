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
	"github.com/golgoth31/sreportal/internal/source/registry"
	svcsrc "github.com/golgoth31/sreportal/internal/source/service"
)

const (
	tTeamA       = "team-a"
	tLBIP        = "1.2.3.4"
	tHostnameAnn = "external-dns.alpha.kubernetes.io/hostname"
)

func TestCycle_ProducesServiceEndpoints(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: tTeamA},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: tTeamA,
			Sources: sreportalv1alpha2.SourcesSpec{
				Service: &sreportalv1alpha2.ServiceSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
				},
			},
		},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "echo", Namespace: tTeamA,
			Annotations: map[string]string{tHostnameAnn: "echo.example.com"},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: tLBIP}},
		}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dns, svc).Build()
	reg := registry.NewRegistry(svcsrc.NewResolver())
	store := rsource.NewStore()

	prev := srccontrol.Cycle(context.Background(), c, reg, nil, store, nil)
	require.NotEmpty(t, prev)
	got, err := store.Lookup(svcsrc.SourceTypeService, tTeamA, "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "echo.example.com", got[0].Endpoint.DNSName)
}

// TestCycle_SetsResourceLabelWhenResolverOmitsIt verifies that Cycle fills in
// the external-dns "resource" label (kind/namespace/name) for resolvers that
// don't set it themselves (e.g. service, ingress) — required for
// DNSRecordEntry.OriginRef to be carried downstream. See PR #291.
func TestCycle_SetsResourceLabelWhenResolverOmitsIt(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: tTeamA},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: tTeamA,
			Sources: sreportalv1alpha2.SourcesSpec{
				Service: &sreportalv1alpha2.ServiceSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
				},
			},
		},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "echo", Namespace: tTeamA,
			Annotations: map[string]string{tHostnameAnn: "echo.example.com"},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: tLBIP}},
		}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dns, svc).Build()
	reg := registry.NewRegistry(svcsrc.NewResolver())
	store := rsource.NewStore()

	_ = srccontrol.Cycle(context.Background(), c, reg, nil, store, nil)
	got, err := store.Lookup(svcsrc.SourceTypeService, tTeamA, "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "service/team-a/echo", got[0].Endpoint.Labels[endpoint.ResourceLabelKey],
		"resource label must be set so DNSRecordEntry.OriginRef is populated")
}

func TestCycle_DeletesKindsNoLongerEnabled(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := registry.NewRegistry(svcsrc.NewResolver())
	store := rsource.NewStore()
	store.ReplaceKind(svcsrc.SourceTypeService, []domainsource.EnrichedEndpoint{
		{Kind: svcsrc.SourceTypeService, Namespace: "x"},
	})
	prev := map[registry.SourceType]bool{svcsrc.SourceTypeService: true}
	_ = srccontrol.Cycle(context.Background(), c, reg, nil, store, prev)
	got, _ := store.Lookup(svcsrc.SourceTypeService, "", "")
	require.Empty(t, got)
}

// TestCycle_RemoteDNSSkipped verifies that a DNS CR with IsRemote=true is
// excluded from computeEnabledKinds, so its source kinds do NOT drive the
// cycle even when enabled.
func TestCycle_RemoteDNSSkipped(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	// local DNS: service enabled
	localDNS := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "local", Namespace: tTeamA},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: tTeamA,
			Sources: sreportalv1alpha2.SourcesSpec{
				Service: &sreportalv1alpha2.ServiceSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
				},
			},
		},
	}
	// remote DNS: ingress enabled — must be ignored entirely by computeEnabledKinds
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
	reg := registry.NewRegistry(svcsrc.NewResolver())
	store := rsource.NewStore()

	next := srccontrol.Cycle(context.Background(), c, reg, nil, store, nil)

	// Only the local DNS's service kind should be enabled.
	require.True(t, next[svcsrc.SourceTypeService], "service kind must be enabled from local DNS")

	// Ingress (remote-only) must NOT appear in the returned map.
	const ingressKind registry.SourceType = "ingress"
	require.False(t, next[ingressKind], "ingress kind from remote DNS must NOT be enabled")
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

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "ns"},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: "ns",
			Sources: sreportalv1alpha2.SourcesSpec{
				Service: &sreportalv1alpha2.ServiceSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
				},
			},
		},
	}
	inner := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dns).Build()
	c := &noMatchErrClient{Client: inner}

	reg := registry.NewRegistry(svcsrc.NewResolver())
	store := rsource.NewStore()

	// Pre-populate cache with a known entry so we can verify it is NOT wiped.
	cachedEntry := domainsource.EnrichedEndpoint{
		Kind:      svcsrc.SourceTypeService,
		Namespace: "ns",
		Name:      "cached-svc",
	}
	store.ReplaceKind(svcsrc.SourceTypeService, []domainsource.EnrichedEndpoint{cachedEntry})

	// Set metric to a sentinel value so the Set(0) call is observable.
	metrics.SourceKindActive.WithLabelValues(string(svcsrc.SourceTypeService)).Set(99)

	prev := map[registry.SourceType]bool{svcsrc.SourceTypeService: true}
	_ = srccontrol.Cycle(context.Background(), c, reg, nil, store, prev)

	// Previously cached state must be preserved — not wiped.
	got, err := store.Lookup(svcsrc.SourceTypeService, "ns", "")
	require.NoError(t, err)
	require.Len(t, got, 1, "cached entry must be preserved when CRD is not installed")
	require.Equal(t, "cached-svc", got[0].Name)

	// SourceKindActive must be set to 0.
	active := testutil.ToFloat64(metrics.SourceKindActive.WithLabelValues(string(svcsrc.SourceTypeService)))
	require.Equal(t, float64(0), active, "SourceKindActive must be 0 when CRD is not installed")
}

// fakeErrorResolver is a registry.Resolver that returns an error for objects
// whose name appears in the failNames set, and a synthetic endpoint otherwise.
type fakeErrorResolver struct {
	resolveErr error
	failNames  map[string]bool
}

func (r *fakeErrorResolver) Type() registry.SourceType { return svcsrc.SourceTypeService }
func (r *fakeErrorResolver) ObjectList() client.ObjectList {
	return svcsrc.NewResolver().ObjectList()
}
func (r *fakeErrorResolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	if r.failNames[obj.GetName()] {
		return nil, r.resolveErr
	}
	return []*endpoint.Endpoint{
		endpoint.NewEndpoint(obj.GetName()+".example.com", endpoint.RecordTypeA, tLBIP),
	}, nil
}

// TestCycle_PartialResolveError verifies that when some objects fail
// ResolveObject but others succeed:
//   - store.ReplaceKind is called with the survivors only,
//   - SourceErrorsTotal increments once per failing object,
//   - SourceKindActive stays 1.
func TestCycle_PartialResolveError(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "ns"},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: "ns",
			Sources: sreportalv1alpha2.SourcesSpec{
				Service: &sreportalv1alpha2.ServiceSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
				},
			},
		},
	}
	// good: resolves fine; bad: resolver returns error
	goodSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "good", Namespace: "ns"},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: tLBIP}},
		}},
	}
	badSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "ns"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dns, goodSvc, badSvc).Build()

	resolveErr := errors.New("resolve failed")
	resolver := &fakeErrorResolver{
		resolveErr: resolveErr,
		failNames:  map[string]bool{"bad": true},
	}
	reg := registry.NewRegistry(resolver)
	store := rsource.NewStore()

	// Reset metrics for a deterministic baseline.
	metrics.SourceErrorsTotal.Reset()
	metrics.SourceKindActive.Reset()

	_ = srccontrol.Cycle(context.Background(), c, reg, nil, store, nil)

	// Survivors must be in the store.
	got, err := store.Lookup(svcsrc.SourceTypeService, "ns", "")
	require.NoError(t, err)
	require.Len(t, got, 1, "only the good object should be in the store")
	require.Equal(t, "good", got[0].Name)

	// Error counter must have incremented once (for "bad").
	errCount := testutil.ToFloat64(metrics.SourceErrorsTotal.WithLabelValues(string(svcsrc.SourceTypeService)))
	require.Equal(t, float64(1), errCount)

	// Kind must still be active.
	active := testutil.ToFloat64(metrics.SourceKindActive.WithLabelValues(string(svcsrc.SourceTypeService)))
	require.Equal(t, float64(1), active)
}

// TestCycle_AllObjectsFailResolve verifies the atomic-wipe guard: when every
// object fails ResolveObject, store.ReplaceKind is NOT called, the previously
// cached state is preserved, and the error counter increments per failure.
func TestCycle_AllObjectsFailResolve(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "ns"},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: "ns",
			Sources: sreportalv1alpha2.SourcesSpec{
				Service: &sreportalv1alpha2.ServiceSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
				},
			},
		},
	}
	svc1 := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc1", Namespace: "ns"}}
	svc2 := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc2", Namespace: "ns"}}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dns, svc1, svc2).Build()

	resolveErr := errors.New("always fail")
	resolver := &fakeErrorResolver{
		resolveErr: resolveErr,
		failNames:  map[string]bool{"svc1": true, "svc2": true},
	}
	reg := registry.NewRegistry(resolver)
	store := rsource.NewStore()

	// Pre-populate with a cached entry to verify it is preserved.
	cachedEntry := domainsource.EnrichedEndpoint{
		Kind:      svcsrc.SourceTypeService,
		Namespace: "ns",
		Name:      "previously-good",
	}
	store.ReplaceKind(svcsrc.SourceTypeService, []domainsource.EnrichedEndpoint{cachedEntry})

	metrics.SourceErrorsTotal.Reset()
	metrics.SourceKindActive.Reset()

	_ = srccontrol.Cycle(context.Background(), c, reg, nil, store, nil)

	// Cache must NOT have been wiped — previous state preserved.
	got, err := store.Lookup(svcsrc.SourceTypeService, "ns", "")
	require.NoError(t, err)
	require.Len(t, got, 1, "previously cached state must be preserved when all objects fail")
	require.Equal(t, "previously-good", got[0].Name)

	// Error counter must have incremented twice (once per failing object).
	errCount := testutil.ToFloat64(metrics.SourceErrorsTotal.WithLabelValues(string(svcsrc.SourceTypeService)))
	require.Equal(t, float64(2), errCount)
}
