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

package chain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

func TestDeduplicateByPriority_NoPriority_NoChange(t *testing.T) {
	input := map[PortalSourceKey][]*endpoint.Endpoint{
		{PortalName: tPortalMain, SourceType: tSrcService}: {
			{DNSName: tFQDNAPI, RecordType: "A", Targets: endpoint.Targets{tIP10001}},
		},
		{PortalName: tPortalMain, SourceType: tSrcIngress}: {
			{DNSName: tFQDNAPI, RecordType: "A", Targets: endpoint.Targets{tIP10002}},
		},
	}

	result := DeduplicateByPriority(input, nil)
	require.Len(t, result, 2, "no priority → no dedup")
}

func TestDeduplicateByPriority_HigherPriorityWins(t *testing.T) {
	// Priority: ingress > service
	priority := []string{tSrcIngress, tSrcService}

	input := map[PortalSourceKey][]*endpoint.Endpoint{
		{PortalName: tPortalMain, SourceType: tSrcService}: {
			{DNSName: tFQDNAPI, RecordType: "A", Targets: endpoint.Targets{tIP10001}},
			{DNSName: "unique-svc.example.com", RecordType: "A", Targets: endpoint.Targets{"10.0.0.3"}},
		},
		{PortalName: tPortalMain, SourceType: tSrcIngress}: {
			{DNSName: tFQDNAPI, RecordType: "A", Targets: endpoint.Targets{tIP10002}},
		},
	}

	result := DeduplicateByPriority(input, priority)

	// ingress should keep api.example.com (higher priority)
	ingressKey := PortalSourceKey{PortalName: tPortalMain, SourceType: tSrcIngress}
	require.Contains(t, result, ingressKey)
	assert.Len(t, result[ingressKey], 1)
	assert.Equal(t, tFQDNAPI, result[ingressKey][0].DNSName)

	// service should lose api.example.com but keep unique-svc.example.com
	svcKey := PortalSourceKey{PortalName: tPortalMain, SourceType: tSrcService}
	require.Contains(t, result, svcKey)
	assert.Len(t, result[svcKey], 1)
	assert.Equal(t, "unique-svc.example.com", result[svcKey][0].DNSName)
}

func TestDeduplicateByPriority_UnlistedSourceLosesToListed(t *testing.T) {
	// Only tSrcIngress in priority; tSrcService is unlisted → lowest rank
	priority := []string{tSrcIngress}

	input := map[PortalSourceKey][]*endpoint.Endpoint{
		{PortalName: tPortalMain, SourceType: tSrcService}: {
			{DNSName: tFQDNAPI, RecordType: "A", Targets: endpoint.Targets{tIP10001}},
		},
		{PortalName: tPortalMain, SourceType: tSrcIngress}: {
			{DNSName: tFQDNAPI, RecordType: "A", Targets: endpoint.Targets{tIP10002}},
		},
	}

	result := DeduplicateByPriority(input, priority)

	ingressKey := PortalSourceKey{PortalName: tPortalMain, SourceType: tSrcIngress}
	assert.Len(t, result[ingressKey], 1)

	// service loses its only endpoint → key removed from map
	svcKey := PortalSourceKey{PortalName: tPortalMain, SourceType: tSrcService}
	assert.Empty(t, result[svcKey])
}

func TestDeduplicateByPriority_DifferentPortals_Independent(t *testing.T) {
	priority := []string{tSrcIngress, tSrcService}

	input := map[PortalSourceKey][]*endpoint.Endpoint{
		{PortalName: "portal-a", SourceType: tSrcService}: {
			{DNSName: tFQDNAPI, RecordType: "A", Targets: endpoint.Targets{tIP10001}},
		},
		{PortalName: "portal-b", SourceType: tSrcService}: {
			{DNSName: tFQDNAPI, RecordType: "A", Targets: endpoint.Targets{tIP10002}},
		},
	}

	result := DeduplicateByPriority(input, priority)

	// Different portals → no cross-portal dedup, both kept
	assert.Len(t, result, 2)
	keyA := PortalSourceKey{PortalName: "portal-a", SourceType: tSrcService}
	keyB := PortalSourceKey{PortalName: "portal-b", SourceType: tSrcService}
	assert.Len(t, result[keyA], 1)
	assert.Len(t, result[keyB], 1)
}

func TestDeduplicateByPriority_EmptySourceAfterDedup_RemovedFromMap(t *testing.T) {
	priority := []string{"dnsendpoint", tSrcService}

	input := map[PortalSourceKey][]*endpoint.Endpoint{
		{PortalName: tPortalMain, SourceType: registry.SourceType(tSrcService)}: {
			{DNSName: tFQDNAPI, RecordType: "A", Targets: endpoint.Targets{tIP10001}},
		},
		{PortalName: tPortalMain, SourceType: registry.SourceType("dnsendpoint")}: {
			{DNSName: tFQDNAPI, RecordType: "A", Targets: endpoint.Targets{tIP10002}},
		},
	}

	result := DeduplicateByPriority(input, priority)

	// dnsendpoint wins, service loses its only endpoint
	dnsKey := PortalSourceKey{PortalName: tPortalMain, SourceType: "dnsendpoint"}
	assert.Len(t, result[dnsKey], 1)

	svcKey := PortalSourceKey{PortalName: tPortalMain, SourceType: tSrcService}
	_, exists := result[svcKey]
	assert.False(t, exists, "empty source should be removed from map")
}

func TestDeduplicateByPriority_MultipleRecordTypes_SameFQDN_AllKeptForWinner(t *testing.T) {
	priority := []string{tSrcIngress, tSrcService}

	input := map[PortalSourceKey][]*endpoint.Endpoint{
		{PortalName: tPortalMain, SourceType: tSrcService}: {
			{DNSName: tFQDNAPI, RecordType: "A", Targets: endpoint.Targets{tIP10001}},
		},
		{PortalName: tPortalMain, SourceType: tSrcIngress}: {
			{DNSName: tFQDNAPI, RecordType: "A", Targets: endpoint.Targets{tIP10002}},
			{DNSName: tFQDNAPI, RecordType: "AAAA", Targets: endpoint.Targets{"::1"}},
		},
	}

	result := DeduplicateByPriority(input, priority)

	ingressKey := PortalSourceKey{PortalName: tPortalMain, SourceType: tSrcIngress}
	assert.Len(t, result[ingressKey], 2, "ingress wins → all its record types kept")

	svcKey := PortalSourceKey{PortalName: tPortalMain, SourceType: tSrcService}
	_, exists := result[svcKey]
	assert.False(t, exists, "service loses api.example.com entirely")
}
