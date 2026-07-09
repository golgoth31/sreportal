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
	"sort"

	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

const (
	// reasonInvalidFQDN is the skip reason for endpoints whose DNSName fails the
	// DNSRecord FQDN validation pattern.
	reasonInvalidFQDN = "invalid_fqdn"
	// reasonInvalidRecordType is the skip reason for endpoints whose record type
	// is not in the DNSRecord CRD enum.
	reasonInvalidRecordType = "invalid_record_type"
)

// ValidateEntriesHandler drops the endpoints that would be rejected by the
// DNSRecord CRD validation (FQDN pattern or record-type enum) before they reach
// UpsertDNSRecordsHandler.
//
// Rationale: CreateOrUpdate writes every endpoint into a single
// DNSRecord.spec.entries. A single entry that violates the FQDN Pattern or the
// recordType Enum makes the API server reject the whole object, which used to
// abort the entire DNS reconcile and abandon all the (valid) records for that
// source. Pre-filtering with the exact same constraints (domaindns.FQDNPattern
// and domaindns.ValidRecordTypes) keeps the valid entries, records the dropped
// ones on ChainData for status projection, and emits per-kind metrics.
//
// A kind that has any dropped entry this cycle is added to PreserveKinds so
// UpsertDNSRecordsHandler does not delete its existing (last-good) DNSRecord
// when filtering leaves the kind with zero valid entries. This keeps the
// last-good record for a kind that produces only invalid entries — whether the
// all-invalid state is a transient glitch or persistent (in the persistent
// case the stale record is retained by design and the EntriesValid=False
// condition surfaces the problem).
type ValidateEntriesHandler struct{}

// Handle implements reconciler.Handler.
func (*ValidateEntriesHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha2.DNS, ChainData]) error {
	ns, name := rc.Resource.Namespace, rc.Resource.Name
	filtered := make(map[registry.SourceType][]*endpoint.Endpoint, len(rc.Data.KeptEndpointsByKind))

	// Drop this DNS resource's stale valid-entry gauges before re-setting the
	// kinds produced this reconcile, so a kind that stopped producing does not
	// leave a frozen non-zero series. The gauge is current-value, so wipe+re-set
	// is safe; the invalid counter is cumulative and must NOT be reset here.
	metrics.DeleteDNSEntriesValidSeries(ns, name)

	for kind, eps := range rc.Data.KeptEndpointsByKind {
		kept := make([]*endpoint.Endpoint, 0, len(eps))
		invalidByReason := map[string]int{}
		for _, e := range eps {
			reason := skipReason(e)
			if reason == "" {
				kept = append(kept, e)
				continue
			}
			invalidByReason[reason]++
			rc.Data.SkippedEntries = append(rc.Data.SkippedEntries, SkippedEntry{
				FQDN:       e.DNSName,
				RecordType: e.RecordType,
				Reason:     reason,
				Kind:       kind,
			})
		}
		filtered[kind] = kept

		metrics.DNSEntriesValid.WithLabelValues(ns, name, string(kind)).Set(float64(len(kept)))
		for reason, n := range invalidByReason {
			metrics.DNSEntriesInvalid.WithLabelValues(ns, name, string(kind), reason).Add(float64(n))
		}
		// Guard the kind's last-good record against a transient all-invalid glitch.
		if len(invalidByReason) > 0 {
			if rc.Data.PreserveKinds == nil {
				rc.Data.PreserveKinds = map[registry.SourceType]bool{}
			}
			rc.Data.PreserveKinds[kind] = true
		}
	}

	rc.Data.KeptEndpointsByKind = filtered

	// Deterministic order so the projected DNS status is stable across reconciles.
	sort.SliceStable(rc.Data.SkippedEntries, func(i, j int) bool {
		a, b := rc.Data.SkippedEntries[i], rc.Data.SkippedEntries[j]
		if a.FQDN != b.FQDN {
			return a.FQDN < b.FQDN
		}
		if a.RecordType != b.RecordType {
			return a.RecordType < b.RecordType
		}
		return a.Kind < b.Kind
	})

	if n := len(rc.Data.SkippedEntries); n > 0 {
		log.FromContext(ctx).Info("skipped invalid DNS entries before projection",
			"dns", rc.Resource.Namespace+"/"+rc.Resource.Name,
			"count", n)
	}
	return nil
}

// skipReason returns the skip reason for an endpoint, or "" if it is valid.
func skipReason(e *endpoint.Endpoint) string {
	if !domaindns.ValidFQDN(e.DNSName) {
		return reasonInvalidFQDN
	}
	if !domaindns.ValidRecordType(e.RecordType) {
		return reasonInvalidRecordType
	}
	return ""
}
