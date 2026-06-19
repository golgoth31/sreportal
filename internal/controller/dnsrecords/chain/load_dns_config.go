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
	"context"
	"fmt"
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// LoadDNSConfigHandler loads the DNS CR referenced by the DNSRecord and
// populates the shared ChainData with its group mapping and reconciliation
// configuration.
type LoadDNSConfigHandler struct {
	client client.Client
}

// NewLoadDNSConfigHandler constructs a LoadDNSConfigHandler.
func NewLoadDNSConfigHandler(c client.Client) *LoadDNSConfigHandler {
	return &LoadDNSConfigHandler{client: c}
}

// Handle finds the DNS CR whose spec.portalRef matches the record's portalRef
// (within the same namespace) and copies its config into rc.Data. Uses the
// spec.portalRef field indexer rather than Get-by-name so DNS.Name need not
// equal PortalRef.
func (h *LoadDNSConfigHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*v1alpha2.DNSRecord, ChainData]) error {
	record := rc.Resource
	var list v1alpha2.DNSList
	if err := h.client.List(ctx, &list,
		client.InNamespace(record.Namespace),
		client.MatchingFields{portalfeatures.FieldIndexPortalRef: record.Spec.PortalRef},
	); err != nil {
		return fmt.Errorf("list DNS for portal %q: %w", record.Spec.PortalRef, err)
	}
	if len(list.Items) == 0 {
		// No DNS CR for this portal — short-circuit so downstream handlers
		// don't run with default config. The DNS watch in SetupWithManager
		// will re-enqueue this DNSRecord when a matching DNS appears.
		return reconciler.ErrShortCircuit
	}
	// A Portal may be referenced by several DNS CRs (N:1 is allowed by the DNS
	// webhook), and the field-index List returns them in no guaranteed order.
	// Pick deterministically so an unchanged record always resolves the same
	// config and its projected group doesn't flap between reconciles:
	//   1. for auto records, prefer the owning DNS (controller ownerRef);
	//   2. otherwise fall back to the lowest name.
	dns := selectDNS(list.Items, rc.Data.OwnerDNSName)
	rc.Data.GroupMapping = &dns.Spec.GroupMapping
	rc.Data.DisableDNSCheck = dns.Spec.Reconciliation.DisableDNSCheck
	return nil
}

// DNSCheckDisabled reports whether the DNS CR governing record has DNS
// resolution disabled (spec.reconciliation.disableDNSCheck), mirroring the DNS
// selection used by LoadDNSConfigHandler. Returns false when no DNS matches or
// on a list error (fail open to resolution). Used by the async dnsresolve
// Runnable, which doesn't run the chain.
func DNSCheckDisabled(ctx context.Context, c client.Client, record *v1alpha2.DNSRecord) bool {
	if record.Spec.PortalRef == "" {
		return false
	}
	var list v1alpha2.DNSList
	if err := c.List(ctx, &list,
		client.InNamespace(record.Namespace),
		client.MatchingFields{portalfeatures.FieldIndexPortalRef: record.Spec.PortalRef},
	); err != nil || len(list.Items) == 0 {
		return false
	}
	owner := ""
	for _, or := range record.OwnerReferences {
		if or.Controller != nil && *or.Controller && or.Kind == "DNS" {
			owner = or.Name
			break
		}
	}
	return selectDNS(list.Items, owner).Spec.Reconciliation.DisableDNSCheck
}

// selectDNS deterministically picks one DNS from a non-empty list. If ownerName
// matches one of the items it wins; otherwise the item with the lowest name.
func selectDNS(items []v1alpha2.DNS, ownerName string) *v1alpha2.DNS {
	if ownerName != "" {
		for i := range items {
			if items[i].Name == ownerName {
				return &items[i]
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return &items[0]
}
