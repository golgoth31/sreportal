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

const (
	testTargetIP = "1.2.3.4"
	testFQDN     = "a.example.com"
)

type stubResolver struct{ addrs []string }

func (s stubResolver) LookupHost(context.Context, string) ([]string, error) { return s.addrs, nil }
func (s stubResolver) LookupCNAME(context.Context, string) (string, error)  { return "", nil }

func recordWithEndpoint() *v1alpha2.DNSRecord {
	return &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "ns"},
		Spec:       v1alpha2.DNSRecordSpec{PortalRef: "p", SourceType: "ingress"},
		Status: v1alpha2.DNSRecordStatus{Endpoints: []v1alpha2.EndpointStatus{
			{DNSName: testFQDN, RecordType: "A", Targets: []string{testTargetIP}, LastSeen: metav1.Now()},
		}},
	}
}

func newTestClient(t *testing.T, rec *v1alpha2.DNSRecord) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha2.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&v1alpha2.DNSRecord{}).WithObjects(rec).Build()
}

// TestResolveRecord_SetsSyncStatus verifies resolveRecord resolves the endpoint
// and patches the DNSRecord status (it does NOT touch the read store — that
// stays the reconcile's job).
func TestResolveRecord_SetsSyncStatus(t *testing.T) {
	rec := recordWithEndpoint()
	c := newTestClient(t, rec)

	r := &Runnable{Client: c, Resolver: stubResolver{addrs: []string{testTargetIP}}}
	require.NoError(t, r.resolveRecord(context.Background(), rec, []FQDNKey{
		{RecordKey: "ns/r", DNSName: testFQDN, RecordType: "A"},
	}))

	var got v1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(rec), &got))
	require.Equal(t, v1alpha2.SyncStatus(domaindns.SyncStatusSync), got.Status.Endpoints[0].SyncStatus)
}

// TestRunnable_ForceThenTickResolves verifies a forced record is resolved on the
// next tick and its status patched.
func TestRunnable_ForceThenTickResolves(t *testing.T) {
	rec := recordWithEndpoint()
	c := newTestClient(t, rec)
	r := New(c, stubResolver{addrs: []string{testTargetIP}})

	r.Force("ns/r")
	require.NoError(t, r.tick(context.Background()))

	var got v1alpha2.DNSRecord
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(rec), &got))
	require.Equal(t, v1alpha2.SyncStatus(domaindns.SyncStatusSync), got.Status.Endpoints[0].SyncStatus)
}

// TestRunnable_ForceRetainedUntilMaterialised verifies that forcing a record
// whose endpoints aren't materialised yet retains the request (so it isn't
// silently lost) until a later tick where they exist.
func TestRunnable_ForceRetainedUntilMaterialised(t *testing.T) {
	// Record exists but has NO status endpoints yet.
	rec := &v1alpha2.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "ns"},
		Spec:       v1alpha2.DNSRecordSpec{PortalRef: "p"},
	}
	c := newTestClient(t, rec)
	r := New(c, stubResolver{addrs: []string{testTargetIP}})

	r.Force("ns/r")
	require.NoError(t, r.tick(context.Background())) // no endpoints -> nothing resolved, force retained

	r.mu.Lock()
	_, retained := r.forced["ns/r"]
	r.mu.Unlock()
	require.True(t, retained, "force must be retained while endpoints are not materialised")
}
