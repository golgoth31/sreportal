package source_test

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/adapter"
	srccontrol "github.com/golgoth31/sreportal/internal/controller/source"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/metrics"
	rsource "github.com/golgoth31/sreportal/internal/readstore/source"
	"github.com/golgoth31/sreportal/internal/source/externaldns"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

const tNsDefault = "default"

func ingressDNS() *sreportalv1alpha2.DNS {
	return &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: tNsDefault},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: tNsDefault,
			Sources: sreportalv1alpha2.SourcesSpec{
				Ingress: &sreportalv1alpha2.IngressSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
				},
			},
		},
	}
}

// TestCycle_NativeIngressFromRules is the regression guard for #274 (194→5): an
// Ingress carrying spec.rules[].host and NO external-dns hostname annotation
// must yield an FQDN through the native external-dns source path, and the
// sreportal.io/groups annotation must be folded onto the endpoint (enrichment
// via re-fetch from the controller-runtime cache).
func TestCycle_NativeIngressFromRules(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, networkingv1.AddToScheme(scheme))

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app", Namespace: tNsDefault,
			Annotations: map[string]string{adapter.GroupsAnnotationKey: "team-x"},
		},
		Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "app.example.com"}}},
		Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{
			Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: tLBIP}},
		}},
	}

	// controller-runtime client: drives DNS listing + enrichment re-fetch.
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ingressDNS(), ing).Build()
	// clientset feeding the external-dns ingress source.
	provider := externaldns.NewProvider(kubefake.NewSimpleClientset(ing), nil, nil)

	// ingress is discovered natively by the provider; the registry only holds
	// non-native resolvers (none needed here).
	reg := registry.NewRegistry()
	store := rsource.NewStore()

	prev := srccontrol.Cycle(context.Background(), c, reg, provider, store, nil)
	require.True(t, prev[externaldns.KindIngress])

	got, err := store.Lookup(externaldns.KindIngress, tNsDefault, "")
	require.NoError(t, err)
	require.Len(t, got, 1, "ingress spec.rules host must be discovered natively")
	require.Equal(t, "app.example.com", got[0].Endpoint.DNSName)
	require.Equal(t, tNsDefault, got[0].Namespace)
	require.Equal(t, "app", got[0].Name)
	require.Equal(t, "team-x", got[0].Endpoint.Labels[adapter.GroupsAnnotationKey],
		"sreportal.io/groups must be enriched onto the endpoint via re-fetch")
}

// TestCycle_NativeDropGuard verifies §3: a fresh empty collection must NOT
// overwrite a non-empty cache; the previous state is preserved and the drop
// guard counter increments.
func TestCycle_NativeDropGuard(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, networkingv1.AddToScheme(scheme))

	// No Ingress objects anywhere -> external-dns returns 0 endpoints.
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ingressDNS()).Build()
	provider := externaldns.NewProvider(kubefake.NewSimpleClientset(), nil, nil)
	reg := registry.NewRegistry()
	store := rsource.NewStore()

	// Pre-populate a known-good cache entry that must survive the empty cycle.
	store.ReplaceKind(externaldns.KindIngress, []domainsource.EnrichedEndpoint{
		{Kind: externaldns.KindIngress, Namespace: tNsDefault, Name: "previously-good"},
	})

	metrics.SourceDropGuardTriggered.Reset()

	_ = srccontrol.Cycle(context.Background(), c, reg, provider, store, nil)

	got, err := store.Lookup(externaldns.KindIngress, tNsDefault, "")
	require.NoError(t, err)
	require.Len(t, got, 1, "non-empty cache must be preserved when collection returns empty")
	require.Equal(t, "previously-good", got[0].Name)

	triggered := testutil.ToFloat64(metrics.SourceDropGuardTriggered.WithLabelValues(string(externaldns.KindIngress)))
	require.Equal(t, float64(1), triggered, "drop guard counter must increment")
}
