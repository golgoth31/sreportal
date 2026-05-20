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
	return nil
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
