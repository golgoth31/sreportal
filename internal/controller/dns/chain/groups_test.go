package dns_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnschain "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/externaldns"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// TestUpsertDNSRecordsHandler_PropagatesGroups verifies the multi-group
// sreportal.io/groups label is parsed into DNSRecordEntry.Groups.
func TestUpsertDNSRecordsHandler_PropagatesGroups(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: upsertTestNS1, UID: "u1"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "p"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&sreportalv1alpha2.DNSRecord{}).
		WithObjects(dns).
		Build()

	ep := endpoint.NewEndpoint("a.example.com", "A", upsertTestTargetA).
		WithLabel("sreportal.io/groups", "Team A, Shared")

	h := &dnschain.UpsertDNSRecordsHandler{Client: c}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: dns,
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				externaldns.KindService: {ep},
			},
		},
	}
	require.NoError(t, h.Handle(context.Background(), rc))

	var created sreportalv1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: upsertTestNS1, Name: upsertTestRecord}, &created))
	require.Len(t, created.Spec.Entries, 1)
	require.Equal(t, []string{"Team A", "Shared"}, created.Spec.Entries[0].Groups)
}
