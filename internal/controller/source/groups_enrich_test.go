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
)

// TestCycle_FoldsGroupsAnnotationOntoEndpoint verifies the source cycle copies
// the sreportal.io/groups annotation from the resource onto the endpoint labels
// (via adapter.EnrichEndpointLabels) on the resolver path, so downstream
// grouping can see it.
func TestCycle_FoldsGroupsAnnotationOntoEndpoint(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "grpsvc", Namespace: tTeamA,
			Annotations: map[string]string{"sreportal.io/groups": "Team A, Shared"},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(crossDNS("d", tTeamA), svc).Build()
	reg := registry.NewRegistry(&fakeResolver{})
	store := rsource.NewStore()

	_ = srccontrol.Cycle(context.Background(), c, reg, nil, store, nil)

	got, err := store.Lookup(crossKind, tTeamA, "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Team A, Shared", got[0].Endpoint.Labels["sreportal.io/groups"],
		"sreportal.io/groups annotation must be folded onto the endpoint labels")
}
