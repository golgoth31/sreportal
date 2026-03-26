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

package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

func TestDeduplicateByPriority_NoPriority_NoChange(t *testing.T) {
	input := map[PortalSourceKey][]*endpoint.Endpoint{
		{PortalName: "main", SourceType: "service"}: {
			{DNSName: "api.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.1"}},
		},
		{PortalName: "main", SourceType: "ingress"}: {
			{DNSName: "api.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.2"}},
		},
	}

	result := DeduplicateByPriority(input, nil)
	require.Len(t, result, 2, "no priority → no dedup")
}

func TestDeduplicateByPriority_HigherPriorityWins(t *testing.T) {
	// Priority: ingress > service
	priority := []string{"ingress", "service"}

	input := map[PortalSourceKey][]*endpoint.Endpoint{
		{PortalName: "main", SourceType: "service"}: {
			{DNSName: "api.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.1"}},
			{DNSName: "unique-svc.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.3"}},
		},
		{PortalName: "main", SourceType: "ingress"}: {
			{DNSName: "api.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.2"}},
		},
	}

	result := DeduplicateByPriority(input, priority)

	// ingress should keep api.example.com (higher priority)
	ingressKey := PortalSourceKey{PortalName: "main", SourceType: "ingress"}
	require.Contains(t, result, ingressKey)
	assert.Len(t, result[ingressKey], 1)
	assert.Equal(t, "api.example.com", result[ingressKey][0].DNSName)

	// service should lose api.example.com but keep unique-svc.example.com
	svcKey := PortalSourceKey{PortalName: "main", SourceType: "service"}
	require.Contains(t, result, svcKey)
	assert.Len(t, result[svcKey], 1)
	assert.Equal(t, "unique-svc.example.com", result[svcKey][0].DNSName)
}

func TestDeduplicateByPriority_UnlistedSourceLosesToListed(t *testing.T) {
	// Only "ingress" in priority; "service" is unlisted → lowest rank
	priority := []string{"ingress"}

	input := map[PortalSourceKey][]*endpoint.Endpoint{
		{PortalName: "main", SourceType: "service"}: {
			{DNSName: "api.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.1"}},
		},
		{PortalName: "main", SourceType: "ingress"}: {
			{DNSName: "api.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.2"}},
		},
	}

	result := DeduplicateByPriority(input, priority)

	ingressKey := PortalSourceKey{PortalName: "main", SourceType: "ingress"}
	assert.Len(t, result[ingressKey], 1)

	// service loses its only endpoint → key removed from map
	svcKey := PortalSourceKey{PortalName: "main", SourceType: "service"}
	assert.Empty(t, result[svcKey])
}

func TestDeduplicateByPriority_DifferentPortals_Independent(t *testing.T) {
	priority := []string{"ingress", "service"}

	input := map[PortalSourceKey][]*endpoint.Endpoint{
		{PortalName: "portal-a", SourceType: "service"}: {
			{DNSName: "api.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.1"}},
		},
		{PortalName: "portal-b", SourceType: "service"}: {
			{DNSName: "api.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.2"}},
		},
	}

	result := DeduplicateByPriority(input, priority)

	// Different portals → no cross-portal dedup, both kept
	assert.Len(t, result, 2)
	keyA := PortalSourceKey{PortalName: "portal-a", SourceType: "service"}
	keyB := PortalSourceKey{PortalName: "portal-b", SourceType: "service"}
	assert.Len(t, result[keyA], 1)
	assert.Len(t, result[keyB], 1)
}

func TestDeduplicateByPriority_EmptySourceAfterDedup_RemovedFromMap(t *testing.T) {
	priority := []string{"dnsendpoint", "service"}

	input := map[PortalSourceKey][]*endpoint.Endpoint{
		{PortalName: "main", SourceType: registry.SourceType("service")}: {
			{DNSName: "api.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.1"}},
		},
		{PortalName: "main", SourceType: registry.SourceType("dnsendpoint")}: {
			{DNSName: "api.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.2"}},
		},
	}

	result := DeduplicateByPriority(input, priority)

	// dnsendpoint wins, service loses its only endpoint
	dnsKey := PortalSourceKey{PortalName: "main", SourceType: "dnsendpoint"}
	assert.Len(t, result[dnsKey], 1)

	svcKey := PortalSourceKey{PortalName: "main", SourceType: "service"}
	_, exists := result[svcKey]
	assert.False(t, exists, "empty source should be removed from map")
}

func TestDeduplicateByPriority_MultipleRecordTypes_SameFQDN_AllKeptForWinner(t *testing.T) {
	priority := []string{"ingress", "service"}

	input := map[PortalSourceKey][]*endpoint.Endpoint{
		{PortalName: "main", SourceType: "service"}: {
			{DNSName: "api.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.1"}},
		},
		{PortalName: "main", SourceType: "ingress"}: {
			{DNSName: "api.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.2"}},
			{DNSName: "api.example.com", RecordType: "AAAA", Targets: endpoint.Targets{"::1"}},
		},
	}

	result := DeduplicateByPriority(input, priority)

	ingressKey := PortalSourceKey{PortalName: "main", SourceType: "ingress"}
	assert.Len(t, result[ingressKey], 2, "ingress wins → all its record types kept")

	svcKey := PortalSourceKey{PortalName: "main", SourceType: "service"}
	_, exists := result[svcKey]
	assert.False(t, exists, "service loses api.example.com entirely")
}
