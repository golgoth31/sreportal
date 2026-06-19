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
	"context"

	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// IntraDNSDedupHandler enforces source priority at the FQDN level: the first
// (highest-priority) kind to produce a given FQDN owns it, and lower-priority
// kinds contribute nothing for that name.
type IntraDNSDedupHandler struct{}

// Handle implements reconciler.Handler.
//
// Ownership is keyed on the FQDN (DNSName) alone, not (name, recordType): once
// a higher-priority kind claims a name, every endpoint for that name from a
// lower-priority kind is dropped — even a different record type. Two sources
// disagreeing on the same FQDN (e.g. ingress→A vs an ExternalName service→CNAME)
// is exactly the conflict priority exists to resolve; keeping both would surface
// the same FQDN twice in the UI and emit conflicting records.
//
// Multiple record types from the SAME (winning) kind are preserved — a kind
// that publishes A and AAAA for one FQDN keeps both, because ownership is
// compared against the claiming kind, not re-checked per record type.
func (*IntraDNSDedupHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha2.DNS, ChainData]) error {
	ownerByName := map[string]registry.SourceType{}
	kept := make(map[registry.SourceType][]*endpoint.Endpoint, len(rc.Data.EndpointsByKind))
	for _, kind := range rc.Data.PriorityOrder {
		eps := rc.Data.EndpointsByKind[kind]
		out := make([]*endpoint.Endpoint, 0, len(eps))
		for _, e := range eps {
			owner, claimed := ownerByName[e.DNSName]
			if claimed && owner != kind {
				// Owned by a higher-priority kind — drop regardless of record type.
				continue
			}
			if !claimed {
				ownerByName[e.DNSName] = kind
			}
			out = append(out, e)
		}
		kept[kind] = out
	}
	rc.Data.KeptEndpointsByKind = kept
	return nil
}
