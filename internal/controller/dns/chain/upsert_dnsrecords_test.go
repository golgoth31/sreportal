/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dns_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnschain "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/ingress"
	"github.com/golgoth31/sreportal/internal/source/registry"
	"github.com/golgoth31/sreportal/internal/source/service"
)

func TestUpsertDNSRecords_CreatesAndDeletes(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))

	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "ns1", UID: "u1"},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: "p"},
	}
	existing := &sreportalv1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name: "d-ingress", Namespace: "ns1",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: sreportalv1alpha2.GroupVersion.String(),
				Kind:       "DNS",
				Name:       dns.Name,
				UID:        dns.UID,
				Controller: ptr.To(true),
			}},
		},
		Spec: sreportalv1alpha2.DNSRecordSpec{
			Origin:     sreportalv1alpha2.DNSRecordOriginAuto,
			SourceType: sreportalv1alpha2.SourceType(ingress.SourceTypeIngress),
			PortalRef:  "p",
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&sreportalv1alpha2.DNSRecord{}).
		WithObjects(dns, existing).
		Build()

	h := &dnschain.UpsertDNSRecordsHandler{Client: c}
	rc := &reconciler.ReconcileContext[*sreportalv1alpha2.DNS, dnschain.ChainData]{
		Resource: dns,
		Data: dnschain.ChainData{
			KeptEndpointsByKind: map[registry.SourceType][]*endpoint.Endpoint{
				service.SourceTypeService: {endpoint.NewEndpoint("a.example.com", "A", "1.1.1.1")},
			},
		},
	}
	require.NoError(t, h.Handle(context.Background(), rc))

	var created sreportalv1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: "ns1", Name: "d-service"}, &created))
	require.Equal(t, sreportalv1alpha2.DNSRecordOriginAuto, created.Spec.Origin)
	require.Equal(t, string(service.SourceTypeService), string(created.Spec.SourceType))
	require.Equal(t, "p", created.Spec.PortalRef)
	require.Empty(t, created.Spec.Entries)
	require.Len(t, created.Status.Endpoints, 1)
	require.Equal(t, "a.example.com", created.Status.Endpoints[0].DNSName)
	require.Equal(t, []string{"1.1.1.1"}, created.Status.Endpoints[0].Targets)

	var gone sreportalv1alpha2.DNSRecord
	err := c.Get(context.Background(), types.NamespacedName{Namespace: "ns1", Name: "d-ingress"}, &gone)
	require.True(t, apierrors.IsNotFound(err), "expected d-ingress to be deleted, got err=%v", err)
}
