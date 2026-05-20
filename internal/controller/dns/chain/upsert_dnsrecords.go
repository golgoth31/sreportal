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
	"fmt"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// UpsertDNSRecordsHandler creates or updates one auto-origin DNSRecord per kind
// that produced at least one endpoint, owned by the DNS CR. It also deletes any
// previously-created auto DNSRecord whose kind is no longer producing.
type UpsertDNSRecordsHandler struct {
	Client client.Client
}

// Handle implements reconciler.Handler.
func (h *UpsertDNSRecordsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha2.DNS, ChainData]) error {
	dns := rc.Resource
	desiredKinds := map[registry.SourceType]bool{}

	for kind, eps := range rc.Data.KeptEndpointsByKind {
		if len(eps) == 0 {
			continue
		}
		desiredKinds[kind] = true
		if err := h.upsertOne(ctx, dns, kind, eps); err != nil {
			return err
		}
	}

	var existing sreportalv1alpha2.DNSRecordList
	if err := h.Client.List(ctx, &existing, client.InNamespace(dns.Namespace)); err != nil {
		return err
	}
	for i := range existing.Items {
		dr := &existing.Items[i]
		if !ownedBy(dr, dns) || dr.Spec.Origin != sreportalv1alpha2.DNSRecordOriginAuto {
			continue
		}
		if desiredKinds[registry.SourceType(dr.Spec.SourceType)] {
			continue
		}
		if err := h.Client.Delete(ctx, dr); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (h *UpsertDNSRecordsHandler) upsertOne(ctx context.Context, dns *sreportalv1alpha2.DNS, kind registry.SourceType, eps []*endpoint.Endpoint) error {
	name := fmt.Sprintf("%s-%s", dns.Name, string(kind))
	dr := &sreportalv1alpha2.DNSRecord{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: dns.Namespace}}
	desiredEntries := endpointsToEntries(eps)

	if _, err := controllerutil.CreateOrUpdate(ctx, h.Client, dr, func() error {
		if dr.Spec.Origin == "" {
			dr.Spec.Origin = sreportalv1alpha2.DNSRecordOriginAuto
		}
		dr.Spec.PortalRef = dns.Spec.PortalRef
		dr.Spec.SourceType = sreportalv1alpha2.SourceType(kind)
		dr.Spec.Entries = desiredEntries
		return controllerutil.SetControllerReference(dns, dr, h.Client.Scheme())
	}); err != nil {
		return err
	}
	return nil
}

// endpointsToEntries converts external-dns endpoints into the manifest-shape
// DNSRecordEntry used in spec.entries. The group label is propagated via
// DNSRecordEntry.Group; other labels are dropped here and will be re-derived
// downstream from the DNS CR config.
//
// Output is deterministic: entries are sorted by (FQDN, RecordType) and each
// entry's Targets is sorted. Duplicate (FQDN, RecordType) inputs are merged
// (targets unioned). Determinism + dedup keep CreateOrUpdate idempotent —
// the spec is written only when content actually changes, and the
// GenerationChangedPredicate on the DNSRecord controller filters out
// no-op spec updates.
func endpointsToEntries(eps []*endpoint.Endpoint) []sreportalv1alpha2.DNSRecordEntry {
	type key struct {
		fqdn, recordType string
	}
	byKey := make(map[key]*sreportalv1alpha2.DNSRecordEntry, len(eps))
	for _, e := range eps {
		k := key{fqdn: e.DNSName, recordType: e.RecordType}
		entry, ok := byKey[k]
		if !ok {
			entry = &sreportalv1alpha2.DNSRecordEntry{
				FQDN:       e.DNSName,
				RecordType: e.RecordType,
			}
			if g, gok := e.Labels["sreportal.io/group"]; gok {
				entry.Group = g
			}
			byKey[k] = entry
		}
		entry.Targets = append(entry.Targets, e.Targets...)
	}

	out := make([]sreportalv1alpha2.DNSRecordEntry, 0, len(byKey))
	for _, entry := range byKey {
		seen := make(map[string]struct{}, len(entry.Targets))
		dedup := entry.Targets[:0]
		for _, t := range entry.Targets {
			if _, exists := seen[t]; exists {
				continue
			}
			seen[t] = struct{}{}
			dedup = append(dedup, t)
		}
		sort.Strings(dedup)
		entry.Targets = dedup
		out = append(out, *entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FQDN != out[j].FQDN {
			return out[i].FQDN < out[j].FQDN
		}
		return out[i].RecordType < out[j].RecordType
	})
	return out
}

func ownedBy(obj client.Object, owner *sreportalv1alpha2.DNS) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == owner.UID && ref.Kind == "DNS" {
			return true
		}
	}
	return false
}
