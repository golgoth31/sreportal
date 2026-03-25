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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// SourceManual indicates a manually configured FQDN
	SourceManual = "manual"
	// SourceExternalDNS indicates an FQDN discovered from external-dns
	SourceExternalDNS = "external-dns"
	// SourceRemote indicates an FQDN fetched from a remote portal
	SourceRemote = "remote"
)

// AggregateFQDNsHandler converts manual DNS groups into FQDNGroupStatus entries.
type AggregateFQDNsHandler struct{}

// NewAggregateFQDNsHandler creates a new AggregateFQDNsHandler
func NewAggregateFQDNsHandler() *AggregateFQDNsHandler {
	return &AggregateFQDNsHandler{}
}

// Handle implements reconciler.Handler
func (h *AggregateFQDNsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DNS, ChainData]) error {
	logger := log.FromContext(ctx).WithName("aggregate-fqdns")
	now := metav1.Now()

	groups := make([]sreportalv1alpha1.FQDNGroupStatus, 0, len(rc.Data.ManualGroups))

	for _, group := range rc.Data.ManualGroups {
		fqdns := make([]sreportalv1alpha1.FQDNStatus, 0, len(group.Entries))
		for _, entry := range group.Entries {
			fqdns = append(fqdns, sreportalv1alpha1.FQDNStatus{
				FQDN:        entry.FQDN,
				Description: entry.Description,
				LastSeen:    now,
			})
		}
		sort.Slice(fqdns, func(i, j int) bool {
			return fqdns[i].FQDN < fqdns[j].FQDN
		})

		groups = append(groups, sreportalv1alpha1.FQDNGroupStatus{
			Name:        group.Name,
			Description: group.Description,
			Source:      SourceManual,
			FQDNs:       fqdns,
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	logger.V(1).Info("aggregated manual groups", "count", len(groups))
	rc.Data.AggregatedGroups = groups
	return nil
}
