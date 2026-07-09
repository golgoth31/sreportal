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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// SourcesStatusHandler sets the SourcesReady and TargetsConflict conditions
// on the DNS CR based on the lookup result and the FQDNStore conflict ring.
type SourcesStatusHandler struct {
	Conflicts domaindns.FQDNConflictReader
}

// Handle implements reconciler.Handler.
func (h *SourcesStatusHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha2.DNS, ChainData]) error {
	dns := rc.Resource
	// SourcesReady reflects whether the lookup chain reached this step with at
	// least one enabled source. When PriorityOrder is empty the DNS CR has no
	// source enabled (spec.sources.* all off) — that's a configuration gap, not
	// a healthy "Producing" state, so emit Unknown/NoSourcesEnabled instead.
	// Chain failures upstream (LookupSourcesHandler error) short-circuit before
	// this handler runs; the DNS controller is responsible for flipping
	// SourcesReady=False in that path.
	if len(rc.Data.PriorityOrder) == 0 {
		SetCondition(dns, metav1.Condition{
			Type:    "SourcesReady",
			Status:  metav1.ConditionUnknown,
			Reason:  "NoSourcesEnabled",
			Message: "no source kinds are enabled in spec.sources",
		})
	} else {
		SetCondition(dns, metav1.Condition{
			Type:   "SourcesReady",
			Status: metav1.ConditionTrue,
			Reason: "Producing",
		})
	}

	var events []domaindns.ConflictEvent
	if h.Conflicts != nil {
		events = h.Conflicts.Conflicts(dns.Namespace, dns.Name)
	}
	if len(events) > 0 {
		SetCondition(dns, metav1.Condition{
			Type:    "TargetsConflict",
			Status:  metav1.ConditionTrue,
			Reason:  "FirstWriterWins",
			Message: "this DNS lost target conflicts on one or more FQDNs",
		})
	} else {
		SetCondition(dns, metav1.Condition{
			Type:   "TargetsConflict",
			Status: metav1.ConditionFalse,
			Reason: "NoConflicts",
		})
	}

	projectSkippedEntries(dns, rc.Data.SkippedEntries)
	return nil
}

const (
	// maxSkippedStatus bounds how many skipped entries are mirrored onto the DNS
	// status. It must stay <= the +kubebuilder:validation:MaxItems marker on
	// DNSStatus.SkippedEntries so a large batch of invalid entries can never
	// bloat the status object past the etcd size limit (which would fail the
	// whole status write). The full count stays in the condition message and the
	// dns_entries_invalid_total metric.
	maxSkippedStatus = 100
	// maxSkippedFQDNLen bounds the mirrored FQDN length to the DNS name limit,
	// matching the +kubebuilder:validation:MaxLength marker on
	// SkippedFQDNStatus.FQDN.
	maxSkippedFQDNLen = 253
	// maxSkippedRecordTypeLen bounds the mirrored record type, matching the
	// +kubebuilder:validation:MaxLength marker on SkippedFQDNStatus.RecordType.
	// A dropped entry's record type is source-controlled and unbounded.
	maxSkippedRecordTypeLen = 16
)

// truncateRunes returns s limited to max Unicode code points, on a rune
// boundary. CRD MaxLength counts code points, so a byte-slice could both split
// a multi-byte rune (corrupting the stored value) and over-truncate a valid
// multi-byte string; slicing on runes avoids both.
func truncateRunes(s string, limit int) string {
	if len(s) <= limit { // byte len <= limit => rune count <= limit
		return s
	}
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit])
}

// projectSkippedEntries mirrors a bounded sample of the validation-dropped
// entries onto the DNS status and flips the EntriesValid condition. skipped is
// already sorted deterministically by ValidateEntriesHandler.
func projectSkippedEntries(dns *sreportalv1alpha2.DNS, skipped []SkippedEntry) {
	if len(skipped) == 0 {
		dns.Status.SkippedEntries = nil
		SetCondition(dns, metav1.Condition{
			Type:   "EntriesValid",
			Status: metav1.ConditionTrue,
			Reason: "AllValid",
		})
		return
	}

	sample := skipped
	if len(sample) > maxSkippedStatus {
		sample = sample[:maxSkippedStatus]
	}
	out := make([]sreportalv1alpha2.SkippedFQDNStatus, 0, len(sample))
	for _, s := range sample {
		out = append(out, sreportalv1alpha2.SkippedFQDNStatus{
			FQDN:       truncateRunes(s.FQDN, maxSkippedFQDNLen),
			SourceType: string(s.Kind),
			RecordType: truncateRunes(s.RecordType, maxSkippedRecordTypeLen),
			Reason:     s.Reason,
		})
	}
	dns.Status.SkippedEntries = out

	SetCondition(dns, metav1.Condition{
		Type:    "EntriesValid",
		Status:  metav1.ConditionFalse,
		Reason:  "InvalidEntriesSkipped",
		Message: fmt.Sprintf("%d entr%s skipped due to validation failure", len(skipped), plural(len(skipped))),
	})
}

func plural(n int) string {
	if n == 1 {
		return "y was"
	}
	return "ies were"
}

// SetCondition upserts c into dns.Status.Conditions, preserving
// LastTransitionTime when status is unchanged.
func SetCondition(dns *sreportalv1alpha2.DNS, c metav1.Condition) {
	for i := range dns.Status.Conditions {
		if dns.Status.Conditions[i].Type == c.Type {
			if dns.Status.Conditions[i].Status != c.Status {
				c.LastTransitionTime = metav1.Now()
			} else {
				c.LastTransitionTime = dns.Status.Conditions[i].LastTransitionTime
			}
			dns.Status.Conditions[i] = c
			return
		}
	}
	c.LastTransitionTime = metav1.Now()
	dns.Status.Conditions = append(dns.Status.Conditions, c)
}
