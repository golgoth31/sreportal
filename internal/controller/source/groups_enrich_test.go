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
	rsource "github.com/golgoth31/sreportal/internal/readstore/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
	svcsrc "github.com/golgoth31/sreportal/internal/source/service"
)

// TestCycle_FoldsGroupsAnnotationOntoEndpoint verifies the source cycle copies
// the sreportal.io/groups annotation from the resource onto the endpoint labels
// (via adapter.EnrichEndpointLabels), so downstream grouping can see it.
func TestCycle_FoldsGroupsAnnotationOntoEndpoint(t *testing.T) {
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
			Name: "grpsvc", Namespace: tTeamA,
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": "grpsvc.example.com",
				"sreportal.io/groups":                       "Team A, Shared",
			},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: tLBIP}},
		}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dns, svc).Build()
	reg := registry.NewRegistry(svcsrc.NewResolver())
	store := rsource.NewStore()

	_ = srccontrol.Cycle(context.Background(), c, reg, store, nil)

	got, err := store.Lookup(svcsrc.SourceTypeService, tTeamA, "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Team A, Shared", got[0].Endpoint.Labels["sreportal.io/groups"],
		"sreportal.io/groups annotation must be folded onto the endpoint labels")
}
