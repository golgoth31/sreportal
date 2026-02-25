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

package grpc_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	svcgrpc "github.com/golgoth31/sreportal/internal/grpc"
	dnsv1 "github.com/golgoth31/sreportal/internal/grpc/gen/sreportal/v1"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(s))
	return s
}

func TestListFQDNs_NoDuplicateGroups_WhenDNSAndDNSRecordHaveSameData(t *testing.T) {
	// Arrange: DNS with aggregated status AND DNSRecord with same endpoints.
	// This is the normal steady-state: the DNS controller has already aggregated
	// DNSRecord endpoints into DNS.Status.Groups.
	scheme := newScheme(t)
	now := metav1.NewTime(time.Now())

	dns := &sreportalv1alpha1.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dns",
			Namespace: "default",
		},
		Spec: sreportalv1alpha1.DNSSpec{
			PortalRef: "main",
		},
		Status: sreportalv1alpha1.DNSStatus{
			Groups: []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Services",
					Source: "external-dns",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{
							FQDN:       "api.example.com",
							RecordType: "A",
							Targets:    []string{"10.0.0.1"},
							LastSeen:   now,
						},
						{
							FQDN:       "web.example.com",
							RecordType: "A",
							Targets:    []string{"10.0.0.2"},
							LastSeen:   now,
						},
					},
				},
			},
		},
	}

	dnsRecord := &sreportalv1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-service",
			Namespace: "default",
		},
		Spec: sreportalv1alpha1.DNSRecordSpec{
			SourceType: "service",
			PortalRef:  "main",
		},
		Status: sreportalv1alpha1.DNSRecordStatus{
			Endpoints: []sreportalv1alpha1.EndpointStatus{
				{
					DNSName:    "api.example.com",
					RecordType: "A",
					Targets:    []string{"10.0.0.1"},
					LastSeen:   now,
				},
				{
					DNSName:    "web.example.com",
					RecordType: "A",
					Targets:    []string{"10.0.0.2"},
					LastSeen:   now,
				},
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dns, dnsRecord).
		WithStatusSubresource(dns, dnsRecord).
		Build()

	svc := svcgrpc.NewDNSService(k8sClient)

	// Act
	resp, err := svc.ListFQDNs(
		context.Background(),
		connect.NewRequest(&dnsv1.ListFQDNsRequest{}),
	)

	// Assert: each FQDN should appear exactly once with no duplicate groups
	require.NoError(t, err)
	require.Len(t, resp.Msg.Fqdns, 2, "expected exactly 2 FQDNs, got duplicates")

	for _, fqdn := range resp.Msg.Fqdns {
		assert.Len(t, fqdn.Groups, 1,
			"FQDN %q should have exactly 1 group, got %v", fqdn.Name, fqdn.Groups)
		assert.Equal(t, "Services", fqdn.Groups[0],
			"FQDN %q should be in group 'Services'", fqdn.Name)
	}
}

func TestListFQDNs_NoDuplicateGroupNames_WhenFQDNAppearsMultipleTimesInSameGroup(t *testing.T) {
	// Arrange: DNS with the same FQDN appearing twice in the same group
	// (this can happen if EndpointStatusToGroups hasn't deduplicated yet)
	scheme := newScheme(t)
	now := metav1.NewTime(time.Now())

	dns := &sreportalv1alpha1.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dns",
			Namespace: "default",
		},
		Spec: sreportalv1alpha1.DNSSpec{
			PortalRef: "main",
		},
		Status: sreportalv1alpha1.DNSStatus{
			Groups: []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Services",
					Source: "external-dns",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, LastSeen: now},
						{FQDN: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.2"}, LastSeen: now},
						{FQDN: "web.example.com", RecordType: "A", Targets: []string{"10.0.0.3"}, LastSeen: now},
					},
				},
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dns).
		WithStatusSubresource(dns).
		Build()

	svc := svcgrpc.NewDNSService(k8sClient)

	// Act
	resp, err := svc.ListFQDNs(
		context.Background(),
		connect.NewRequest(&dnsv1.ListFQDNsRequest{}),
	)

	// Assert: api.example.com should appear once, and its Groups should not have duplicates
	require.NoError(t, err)
	require.Len(t, resp.Msg.Fqdns, 2, "expected 2 unique FQDNs")

	for _, fqdn := range resp.Msg.Fqdns {
		if fqdn.Name == "api.example.com" {
			assert.Len(t, fqdn.Groups, 1,
				"api.example.com should have exactly 1 group entry, got %v", fqdn.Groups)
			assert.Equal(t, "Services", fqdn.Groups[0])
		}
	}
}

func TestListFQDNs_ReturnsAllFQDNs_FromDNSStatus(t *testing.T) {
	// Arrange: DNS with groups from multiple sources, no DNSRecords
	scheme := newScheme(t)
	now := metav1.NewTime(time.Now())

	dns := &sreportalv1alpha1.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dns",
			Namespace: "default",
		},
		Spec: sreportalv1alpha1.DNSSpec{
			PortalRef: "main",
		},
		Status: sreportalv1alpha1.DNSStatus{
			Groups: []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Services",
					Source: "external-dns",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, LastSeen: now},
					},
				},
				{
					Name:   "Internal",
					Source: "manual",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "internal.example.com", RecordType: "A", Targets: []string{"10.0.0.2"}, LastSeen: now},
					},
				},
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dns).
		WithStatusSubresource(dns).
		Build()

	svc := svcgrpc.NewDNSService(k8sClient)

	// Act
	resp, err := svc.ListFQDNs(
		context.Background(),
		connect.NewRequest(&dnsv1.ListFQDNsRequest{}),
	)

	// Assert
	require.NoError(t, err)
	require.Len(t, resp.Msg.Fqdns, 2)

	fqdnsByName := make(map[string]*dnsv1.FQDN, len(resp.Msg.Fqdns))
	for _, f := range resp.Msg.Fqdns {
		fqdnsByName[f.Name] = f
	}

	assert.Contains(t, fqdnsByName, "api.example.com")
	assert.Equal(t, "external-dns", fqdnsByName["api.example.com"].Source)
	assert.Contains(t, fqdnsByName, "internal.example.com")
	assert.Equal(t, "manual", fqdnsByName["internal.example.com"].Source)
}

func TestListFQDNs_FiltersWork(t *testing.T) {
	scheme := newScheme(t)
	now := metav1.NewTime(time.Now())

	dns := &sreportalv1alpha1.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dns",
			Namespace: "default",
		},
		Spec: sreportalv1alpha1.DNSSpec{
			PortalRef: "main",
		},
		Status: sreportalv1alpha1.DNSStatus{
			Groups: []sreportalv1alpha1.FQDNGroupStatus{
				{
					Name:   "Services",
					Source: "external-dns",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, LastSeen: now},
						{FQDN: "web.example.com", RecordType: "A", Targets: []string{"10.0.0.2"}, LastSeen: now},
					},
				},
				{
					Name:   "Internal",
					Source: "manual",
					FQDNs: []sreportalv1alpha1.FQDNStatus{
						{FQDN: "internal.example.com", RecordType: "A", Targets: []string{"10.0.0.3"}, LastSeen: now},
					},
				},
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dns).
		WithStatusSubresource(dns).
		Build()

	svc := svcgrpc.NewDNSService(k8sClient)

	cases := []struct {
		name     string
		request  *dnsv1.ListFQDNsRequest
		wantLen  int
		wantFQDN string
	}{
		{
			name:     "search filter",
			request:  &dnsv1.ListFQDNsRequest{Search: "api"},
			wantLen:  1,
			wantFQDN: "api.example.com",
		},
		{
			name:     "source filter external-dns",
			request:  &dnsv1.ListFQDNsRequest{Source: "external-dns"},
			wantLen:  2,
			wantFQDN: "api.example.com",
		},
		{
			name:     "source filter manual",
			request:  &dnsv1.ListFQDNsRequest{Source: "manual"},
			wantLen:  1,
			wantFQDN: "internal.example.com",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := svc.ListFQDNs(
				context.Background(),
				connect.NewRequest(tc.request),
			)
			require.NoError(t, err)
			assert.Len(t, resp.Msg.Fqdns, tc.wantLen)

			var found bool
			for _, f := range resp.Msg.Fqdns {
				if f.Name == tc.wantFQDN {
					found = true
					break
				}
			}
			assert.True(t, found, "expected to find FQDN %q", tc.wantFQDN)
		})
	}
}
