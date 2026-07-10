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

package dns

import (
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

const (
	// SourceManual indicates a manually configured FQDN.
	SourceManual = "manual"
	// SourceExternalDNS indicates an FQDN discovered from external-dns.
	SourceExternalDNS = "external-dns"
	// SourceRemote indicates an FQDN fetched from a remote portal.
	SourceRemote = "remote"
)

// ChainData carries per-reconcile state through the DNS chain handlers.
type ChainData struct {
	// EndpointsByKind is populated by LookupSourcesHandler. Each entry is the
	// post-filter (namespace, labelFilter) slice of enriched endpoints for
	// that kind. Iteration order follows spec.sources.priority.
	EndpointsByKind map[registry.SourceType][]*endpoint.Endpoint

	// KeptEndpointsByKind is populated by IntraDNSDedupHandler — the
	// priority-deduped subset that UpsertDNSRecordsHandler will project.
	KeptEndpointsByKind map[registry.SourceType][]*endpoint.Endpoint

	// PriorityOrder is the iteration order across kinds (from
	// spec.sources.priority + spec.sources.* enabled fallback). Provided to
	// downstream handlers so they don't recompute it.
	PriorityOrder []registry.SourceType

	// PortalDisabled is set to true when the Portal exists but has DNS
	// feature disabled — controllers use this to choose between cleanup and
	// production paths.
	PortalDisabled bool

	// PreserveKinds holds the enabled kinds whose source has not produced a
	// successful collection yet (store not ready — e.g. just after a controller
	// restart, before external-dns informers sync). UpsertDNSRecordsHandler must
	// NOT delete the existing auto DNSRecord of such a kind: its empty lookup is
	// "not synced yet", not "authoritatively empty", and deleting would purge
	// good persisted records until the sources catch up.
	PreserveKinds map[registry.SourceType]bool

	// SkippedEntries is populated by ValidateEntriesHandler with the endpoints
	// dropped before projection because their FQDN failed the DNSRecord
	// validation pattern. Surfaced on DNS status and in metrics so a single bad
	// FQDN no longer aborts the whole reconcile silently.
	SkippedEntries []SkippedEntry
}

// SkippedEntry records a single endpoint dropped during validation.
type SkippedEntry struct {
	// FQDN is the offending fully qualified domain name.
	FQDN string
	// RecordType is the DNS record type of the dropped endpoint.
	RecordType string
	// Reason is a short machine-friendly cause (e.g. "invalid_fqdn").
	Reason string
	// Kind is the source kind that produced the entry.
	Kind registry.SourceType
}
