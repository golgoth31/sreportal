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

package adapter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
)

func TestEndpointsHash_Deterministic(t *testing.T) {
	eps := []*endpoint.Endpoint{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, Labels: map[string]string{"k": "v"}},
		{DNSName: "web.example.com", RecordType: "CNAME", Targets: []string{"lb.example.com"}},
	}

	h1 := adapter.EndpointsHash(eps)
	h2 := adapter.EndpointsHash(eps)

	require.NotEmpty(t, h1)
	assert.Equal(t, h1, h2, "same input should produce same hash")
}

func TestEndpointsHash_DifferentOrder_SameHash(t *testing.T) {
	ep1 := &endpoint.Endpoint{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}}
	ep2 := &endpoint.Endpoint{DNSName: "web.example.com", RecordType: "A", Targets: []string{"10.0.0.2"}}

	h1 := adapter.EndpointsHash([]*endpoint.Endpoint{ep1, ep2})
	h2 := adapter.EndpointsHash([]*endpoint.Endpoint{ep2, ep1})

	assert.Equal(t, h1, h2, "order of endpoints should not affect hash")
}

func TestEndpointsHash_DifferentTargetOrder_SameHash(t *testing.T) {
	eps1 := []*endpoint.Endpoint{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1", "10.0.0.2"}},
	}
	eps2 := []*endpoint.Endpoint{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.2", "10.0.0.1"}},
	}

	assert.Equal(t, adapter.EndpointsHash(eps1), adapter.EndpointsHash(eps2),
		"order of targets should not affect hash")
}

func TestEndpointsHash_DifferentTargets_DifferentHash(t *testing.T) {
	eps1 := []*endpoint.Endpoint{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}},
	}
	eps2 := []*endpoint.Endpoint{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.2"}},
	}

	assert.NotEqual(t, adapter.EndpointsHash(eps1), adapter.EndpointsHash(eps2),
		"different targets should produce different hash")
}

func TestEndpointsHash_IgnoresLastSeenAndTTL(t *testing.T) {
	eps1 := []*endpoint.Endpoint{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, RecordTTL: 300},
	}
	eps2 := []*endpoint.Endpoint{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, RecordTTL: 600},
	}

	assert.Equal(t, adapter.EndpointsHash(eps1), adapter.EndpointsHash(eps2),
		"TTL should not affect hash")
}

func TestEndpointsHash_LabelsAffectHash(t *testing.T) {
	eps1 := []*endpoint.Endpoint{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"},
			Labels: map[string]string{"sreportal.io/portal": "main"}},
	}
	eps2 := []*endpoint.Endpoint{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"},
			Labels: map[string]string{"sreportal.io/portal": "other"}},
	}

	assert.NotEqual(t, adapter.EndpointsHash(eps1), adapter.EndpointsHash(eps2),
		"different labels should produce different hash")
}

func TestEndpointsHash_EmptyEndpoints(t *testing.T) {
	h1 := adapter.EndpointsHash(nil)
	h2 := adapter.EndpointsHash([]*endpoint.Endpoint{})

	assert.Equal(t, h1, h2, "nil and empty should produce same hash")
	require.NotEmpty(t, h1)
}

func TestEndpointsHash_IgnoresResourceLabel(t *testing.T) {
	// The resource label changes between controller restarts (external-dns
	// may re-discover with different resource refs). We must exclude it so
	// that the hash remains stable.
	eps1 := []*endpoint.Endpoint{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"},
			Labels: map[string]string{"sreportal.io/portal": "main"}},
	}
	eps2 := []*endpoint.Endpoint{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"},
			Labels: map[string]string{
				"sreportal.io/portal":           "main",
				endpoint.ResourceLabelKey:       "Service/default/api-svc",
				"sreportal.io/origin-kind":      "Service",
				"sreportal.io/origin-namespace": "default",
				"sreportal.io/origin-name":      "api-svc",
			}},
	}

	assert.Equal(t, adapter.EndpointsHash(eps1), adapter.EndpointsHash(eps2),
		"resource/origin labels should be excluded from hash")
}

func TestEndpointStatusHash_MatchesEndpointsHash(t *testing.T) {
	// The hash from ToEndpointStatus output should match the hash from
	// the original endpoints (for the fields that matter).
	eps := []*endpoint.Endpoint{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"},
			Labels: map[string]string{"sreportal.io/portal": "main"}},
	}

	fromEndpoints := adapter.EndpointsHash(eps)

	statuses := adapter.ToEndpointStatus(eps)
	fromStatuses := adapter.EndpointStatusHash(statuses)

	assert.Equal(t, fromEndpoints, fromStatuses,
		"hash from endpoints and from endpoint statuses should match")
}

func TestEndpointStatusHash_IgnoresSyncStatusAndLastSeen(t *testing.T) {
	s1 := []sreportalv1alpha1.EndpointStatus{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"},
			SyncStatus: "sync", LastSeen: metav1.Now()},
	}
	s2 := []sreportalv1alpha1.EndpointStatus{
		{DNSName: "api.example.com", RecordType: "A", Targets: []string{"10.0.0.1"},
			SyncStatus: "notsync", LastSeen: metav1.Now()},
	}

	assert.Equal(t, adapter.EndpointStatusHash(s1), adapter.EndpointStatusHash(s2),
		"SyncStatus and LastSeen should not affect hash")
}
