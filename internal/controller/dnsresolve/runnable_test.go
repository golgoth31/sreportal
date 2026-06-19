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

package dnsresolve

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
)

type stubResolver struct{ addrs []string }

func (s stubResolver) LookupHost(_ context.Context, _ string) ([]string, error) {
	return s.addrs, nil
}
func (s stubResolver) LookupCNAME(_ context.Context, _ string) (string, error) {
	return "", nil
}

type capWriter struct{ views []domaindns.FQDNView }

func (w *capWriter) Replace(_ context.Context, _, _ string, fqdns []domaindns.FQDNView) error {
	w.views = fqdns
	return nil
}
func (w *capWriter) Delete(_ context.Context, _ string) error { return nil }
func (w *capWriter) AnnotateOwner(_, _, _ string)             {}

func TestResolveRecord_SetsSyncStatusAndRefreshesStore(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha2.AddToScheme(scheme))

	rec := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "ns"},
		Spec:       v1alpha2.DNSRecordSpec{PortalRef: "p", SourceType: "ingress"},
		Status: v1alpha2.DNSRecordStatus{Endpoints: []v1alpha2.EndpointStatus{
			{DNSName: "a.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}, LastSeen: metav1.Now()},
		}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&v1alpha2.DNSRecord{}).WithObjects(rec).Build()
	w := &capWriter{}

	r := &Runnable{Client: c, Resolver: stubResolver{addrs: []string{"1.2.3.4"}}, FQDNWriter: w}
	require.NoError(t, r.resolveRecord(context.Background(), rec, nil, []FQDNKey{
		{RecordKey: "ns/r", DNSName: "a.example.com", RecordType: "A"},
	}))

	var got v1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(rec), &got))
	require.Equal(t, v1alpha2.SyncStatus(domaindns.SyncStatusSync), got.Status.Endpoints[0].SyncStatus)
	require.Len(t, w.views, 1)
	require.Equal(t, string(domaindns.SyncStatusSync), w.views[0].SyncStatus)
}

func TestRunnable_ForceThenTickResolves(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha2.AddToScheme(scheme))
	rec := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "ns"},
		Spec:       v1alpha2.DNSRecordSpec{PortalRef: "p"},
		Status: v1alpha2.DNSRecordStatus{Endpoints: []v1alpha2.EndpointStatus{
			{DNSName: "a.example.com", RecordType: "A", Targets: []string{"1.2.3.4"}, LastSeen: metav1.Now()},
		}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&v1alpha2.DNSRecord{}).WithObjects(rec).Build()
	w := &capWriter{}
	r := New(c, stubResolver{addrs: []string{"1.2.3.4"}}, w)

	r.Force("ns/r")
	require.NoError(t, r.tick(context.Background()))

	var got v1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(rec), &got))
	require.Equal(t, v1alpha2.SyncStatus(domaindns.SyncStatusSync), got.Status.Endpoints[0].SyncStatus)
}
