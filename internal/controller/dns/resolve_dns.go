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
	"sync"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// maxConcurrentLookups limits the number of parallel DNS resolutions.
	maxConcurrentLookups = 10
	// lookupTimeout is the per-FQDN DNS resolution timeout.
	lookupTimeout = 5 * time.Second
)

// ResolveDNSHandler resolves each FQDN against DNS to determine its SyncStatus.
type ResolveDNSHandler struct {
	resolver domaindns.Resolver
}

// NewResolveDNSHandler creates a new ResolveDNSHandler with the given resolver.
func NewResolveDNSHandler(r domaindns.Resolver) *ResolveDNSHandler {
	return &ResolveDNSHandler{resolver: r}
}

// Handle implements reconciler.Handler.
func (h *ResolveDNSHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DNS]) error {
	log := logf.FromContext(ctx).WithName("resolve-dns")

	groups, ok := rc.Data[DataKeyAggregatedGroups].([]sreportalv1alpha1.FQDNGroupStatus)
	if !ok || len(groups) == 0 {
		log.V(1).Info("no aggregated groups to resolve")
		return nil
	}

	// Collect all FQDN pointers for parallel resolution.
	type fqdnRef struct {
		groupIdx int
		fqdnIdx  int
	}

	var refs []fqdnRef
	for gi := range groups {
		// Skip remote groups — their SyncStatus comes from the remote portal.
		if groups[gi].Source == SourceRemote {
			continue
		}
		for fi := range groups[gi].FQDNs {
			refs = append(refs, fqdnRef{groupIdx: gi, fqdnIdx: fi})
		}
	}

	if len(refs) == 0 {
		return nil
	}

	log.V(1).Info("resolving FQDNs", "count", len(refs))

	// Parallel resolution with semaphore.
	sem := make(chan struct{}, maxConcurrentLookups)
	var wg sync.WaitGroup

	for _, ref := range refs {
		wg.Add(1)
		go func(r fqdnRef) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			fqdn := &groups[r.groupIdx].FQDNs[r.fqdnIdx]
			lookupCtx, cancel := context.WithTimeout(ctx, lookupTimeout)
			defer cancel()

			result := domaindns.CheckFQDN(lookupCtx, h.resolver, fqdn.FQDN, fqdn.RecordType, fqdn.Targets)
			fqdn.SyncStatus = string(result.Status)
		}(ref)
	}

	wg.Wait()

	rc.Data[DataKeyAggregatedGroups] = groups
	log.V(1).Info("DNS resolution completed", "count", len(refs))
	return nil
}
