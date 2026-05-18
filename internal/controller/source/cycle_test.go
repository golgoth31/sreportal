package source_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	srccontrol "github.com/golgoth31/sreportal/internal/controller/source"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	rsource "github.com/golgoth31/sreportal/internal/readstore/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
	svcsrc "github.com/golgoth31/sreportal/internal/source/service"
)

func TestCycle_ProducesServiceEndpoints(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "team-a"},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef: "team-a",
			Sources: sreportalv1alpha2.SourcesSpec{
				Service: &sreportalv1alpha2.ServiceSourceSpec{
					CommonSourceSpec: sreportalv1alpha2.CommonSourceSpec{Enabled: true},
				},
			},
		},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "echo", Namespace: "team-a",
			Annotations: map[string]string{"external-dns.alpha.kubernetes.io/hostname": "echo.example.com"},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: "1.2.3.4"}},
		}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dns, svc).Build()
	reg := registry.NewRegistry(svcsrc.NewResolver())
	store := rsource.NewStore()

	prev := srccontrol.Cycle(context.Background(), c, reg, store, nil)
	require.NotEmpty(t, prev)
	got, err := store.Lookup(svcsrc.SourceTypeService, "team-a", "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "echo.example.com", got[0].Endpoint.DNSName)
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
	_ = srccontrol.Cycle(context.Background(), c, reg, store, prev)
	got, _ := store.Lookup(svcsrc.SourceTypeService, "", "")
	require.Empty(t, got)
}
