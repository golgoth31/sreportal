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

// IntraDNSDedupHandler drops endpoints whose FQDN was already produced by a
// higher-priority kind earlier in the iteration order.
type IntraDNSDedupHandler struct{}

// dedupKey matches the FQDNStore identity ({Name, RecordType}) so endpoints
// for the same FQDN but different record types (e.g. A vs AAAA) don't shadow
// each other at this stage.
type dedupKey struct {
	name       string
	recordType string
}

// Handle implements reconciler.Handler.
func (*IntraDNSDedupHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha2.DNS, ChainData]) error {
	seen := map[dedupKey]struct{}{}
	kept := make(map[registry.SourceType][]*endpoint.Endpoint, len(rc.Data.EndpointsByKind))
	for _, kind := range rc.Data.PriorityOrder {
		eps := rc.Data.EndpointsByKind[kind]
		out := make([]*endpoint.Endpoint, 0, len(eps))
		for _, e := range eps {
			k := dedupKey{name: e.DNSName, recordType: e.RecordType}
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, e)
		}
		kept[kind] = out
	}
	rc.Data.KeptEndpointsByKind = kept
	return nil
}
