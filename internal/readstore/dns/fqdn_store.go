// Package dns provides the in-memory FQDNStore implementation.
// It implements both dns.FQDNReader and dns.FQDNWriter.
// Source-priority dedup is enforced upstream (per-DNS-CR at DNSRecord generation
// time); the store treats each Replace call as authoritative for its
// (recordKey, portalRef) tuple.
package dns

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/metrics"
)

// FQDNKey uniquely identifies an (fqdn, recordType) pair.
type FQDNKey struct {
	Name       string
	RecordType string
}

type recordContribution struct {
	keys         map[FQDNKey]struct{}
	portalRef    string
	dnsNamespace string
	dnsName      string
}

// FQDNStore is the in-memory implementation of dns.FQDNReader and dns.FQDNWriter.
type FQDNStore struct {
	mu             sync.RWMutex
	fqdns          map[FQDNKey]*domaindns.FQDNView
	byPortal       map[string]map[FQDNKey]struct{}
	byRecord       map[string]recordContribution
	perPortalCount map[FQDNKey]map[string]int
	conflicts      *conflictRing

	notifyMu sync.Mutex
	notifyCh chan struct{}
}

// NewFQDNStore returns an empty FQDNStore. Source priority is enforced
// upstream (per-DNS-CR at DNSRecord generation time); the store treats
// each Replace call as authoritative for its (recordKey, portalRef) tuple.
func NewFQDNStore() *FQDNStore {
	return &FQDNStore{
		fqdns:          map[FQDNKey]*domaindns.FQDNView{},
		byPortal:       map[string]map[FQDNKey]struct{}{},
		byRecord:       map[string]recordContribution{},
		perPortalCount: map[FQDNKey]map[string]int{},
		conflicts:      newConflictRing(256),
		notifyCh:       make(chan struct{}),
	}
}

// compile-time interface checks
var (
	_ domaindns.FQDNReader         = (*FQDNStore)(nil)
	_ domaindns.FQDNWriter         = (*FQDNStore)(nil)
	_ domaindns.FQDNConflictReader = (*FQDNStore)(nil)
)

// Replace atomically replaces all FQDNs contributed by a single DNSRecord.
func (s *FQDNStore) Replace(ctx context.Context, recordKey, portalRef string, fqdns []domaindns.FQDNView) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newKeys := make(map[FQDNKey]struct{}, len(fqdns))
	for _, v := range fqdns {
		newKeys[FQDNKey{Name: v.Name, RecordType: v.RecordType}] = struct{}{}
	}

	prev := s.byRecord[recordKey]
	portalChanged := prev.portalRef != "" && prev.portalRef != portalRef

	// Remove keys this record no longer contributes. When the record's portal
	// changed, also decrement kept keys under the previous portal so the old
	// portal index does not retain a ghost contribution.
	for k := range prev.keys {
		if _, kept := newKeys[k]; !kept || portalChanged {
			s.decrementContribution(k, prev.portalRef)
		}
	}

	for _, v := range fqdns {
		k := FQDNKey{Name: v.Name, RecordType: v.RecordType}
		existing, ok := s.fqdns[k]
		if !ok {
			cp := v
			cp.Portals = []string{portalRef}
			s.fqdns[k] = &cp
		} else {
			if !sameTargets(existing.Targets, v.Targets) {
				s.conflicts.Push(domaindns.ConflictEvent{
					FQDNKey:     domaindns.ConflictFQDNKey{Name: k.Name, RecordType: k.RecordType},
					LoserRecord: recordKey,
					PortalRef:   portalRef,
					At:          time.Now(),
				})
				metrics.DNSTargetsConflictTotal.WithLabelValues(portalRef).Inc()
				// first-writer wins for Targets/SyncStatus/OriginRef/Description
			}
			existing.Groups = mergeStrings(existing.Groups, v.Groups)
			if !contains(existing.Portals, portalRef) {
				existing.Portals = append(existing.Portals, portalRef)
				sort.Strings(existing.Portals)
			}
		}
		if s.perPortalCount[k] == nil {
			s.perPortalCount[k] = map[string]int{}
		}
		if _, hadBefore := prev.keys[k]; !hadBefore || portalChanged {
			s.perPortalCount[k][portalRef]++
		}

		if s.byPortal[portalRef] == nil {
			s.byPortal[portalRef] = map[FQDNKey]struct{}{}
		}
		s.byPortal[portalRef][k] = struct{}{}
	}

	s.byRecord[recordKey] = recordContribution{
		keys:         newKeys,
		portalRef:    portalRef,
		dnsNamespace: prev.dnsNamespace,
		dnsName:      prev.dnsName,
	}

	go s.broadcast()
	return nil
}

// AnnotateOwner records the DNS owner of a DNSRecord. Called by the
// DNSRecord controller after a successful Replace.
func (s *FQDNStore) AnnotateOwner(recordKey, dnsNamespace, dnsName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.byRecord[recordKey]
	if !ok {
		return
	}
	c.dnsNamespace = dnsNamespace
	c.dnsName = dnsName
	s.byRecord[recordKey] = c
}

// Conflicts returns conflict events whose loser DNSRecord is owned by the
// given DNS. Pass empty strings to return all events.
// NOTE: Snapshot is taken before acquiring s.mu to preserve lock ordering
// (ring lock → store read lock, never the other way).
func (s *FQDNStore) Conflicts(dnsNamespace, dnsName string) []domaindns.ConflictEvent {
	all := s.conflicts.Snapshot()
	if dnsNamespace == "" && dnsName == "" {
		return all
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domaindns.ConflictEvent, 0)
	for _, e := range all {
		c, ok := s.byRecord[e.LoserRecord]
		if !ok {
			continue
		}
		if c.dnsNamespace == dnsNamespace && c.dnsName == dnsName {
			out = append(out, e)
		}
	}
	return out
}

// Delete removes all FQDNs contributed by a single DNSRecord.
func (s *FQDNStore) Delete(ctx context.Context, recordKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	contrib, ok := s.byRecord[recordKey]
	if !ok {
		return nil
	}
	for k := range contrib.keys {
		s.decrementContribution(k, contrib.portalRef)
	}
	delete(s.byRecord, recordKey)

	go s.broadcast()
	return nil
}

// List returns FQDNs matching the given filters, sorted by (Name, RecordType).
func (s *FQDNStore) List(ctx context.Context, f domaindns.FQDNFilters) ([]domaindns.FQDNView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listLocked(f), nil
}

// Get returns the FQDN matching name (case-insensitive) and recordType. RecordType "" matches any.
func (s *FQDNStore) Get(ctx context.Context, name, recordType string) (domaindns.FQDNView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lname := strings.ToLower(name)
	for k, v := range s.fqdns {
		if strings.ToLower(k.Name) != lname {
			continue
		}
		if recordType == "" || k.RecordType == recordType {
			return cloneFQDNView(v), nil
		}
	}
	return domaindns.FQDNView{}, fmt.Errorf("%w: %s/%s", domaindns.ErrFQDNNotFound, name, recordType)
}

// Count returns the number of FQDNs matching the given filters.
func (s *FQDNStore) Count(ctx context.Context, f domaindns.FQDNFilters) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.listLocked(f)), nil
}

// listLocked applies filters and sorting. Caller must hold s.mu (read or write).
// Returned views are deep-copied so callers cannot observe in-place mutations
// of slice fields by a subsequent Replace.
func (s *FQDNStore) listLocked(f domaindns.FQDNFilters) []domaindns.FQDNView {
	var pool []*domaindns.FQDNView
	if f.Portal != "" {
		set := s.byPortal[f.Portal]
		pool = make([]*domaindns.FQDNView, 0, len(set))
		for k := range set {
			if v := s.fqdns[k]; v != nil {
				pool = append(pool, v)
			}
		}
	} else {
		pool = make([]*domaindns.FQDNView, 0, len(s.fqdns))
		for _, v := range s.fqdns {
			pool = append(pool, v)
		}
	}

	searchLower := strings.ToLower(f.Search)
	out := make([]domaindns.FQDNView, 0, len(pool))
	for _, v := range pool {
		if f.Namespace != "" && v.Namespace != f.Namespace {
			continue
		}
		if f.Source != "" && string(v.Source) != f.Source {
			continue
		}
		if f.Search != "" && !strings.Contains(strings.ToLower(v.Name), searchLower) {
			continue
		}
		out = append(out, cloneFQDNView(v))
	}
	slices.SortFunc(out, func(a, b domaindns.FQDNView) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return cmp.Compare(a.RecordType, b.RecordType)
	})
	return out
}

// cloneFQDNView returns a value copy whose slice fields share no backing array
// with the source. The store's writers append to Portals/Groups/Targets in
// place when capacity allows, so callers must hold their own copies to be
// safe across subsequent mutations.
func cloneFQDNView(v *domaindns.FQDNView) domaindns.FQDNView {
	out := *v
	if v.Targets != nil {
		out.Targets = append([]string(nil), v.Targets...)
	}
	if v.Groups != nil {
		out.Groups = append([]string(nil), v.Groups...)
	}
	if v.Portals != nil {
		out.Portals = append([]string(nil), v.Portals...)
	}
	return out
}

// Subscribe returns a channel closed on the next store mutation.
func (s *FQDNStore) Subscribe() <-chan struct{} {
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()
	return s.notifyCh
}

// broadcast swaps notifyCh under notifyMu and closes the previous channel.
// MUST NOT be called while holding s.mu — broadcast acquires notifyMu and a
// future Subscribe caller that walks back into a path holding s.mu would
// deadlock. Lock order: s.mu first, then release before calling broadcast.
func (s *FQDNStore) broadcast() {
	s.notifyMu.Lock()
	old := s.notifyCh
	s.notifyCh = make(chan struct{})
	s.notifyMu.Unlock()
	close(old)
}

func (s *FQDNStore) decrementContribution(k FQDNKey, portalRef string) {
	counts := s.perPortalCount[k]
	if counts == nil {
		return
	}
	counts[portalRef]--
	if counts[portalRef] <= 0 {
		delete(counts, portalRef)
		if set := s.byPortal[portalRef]; set != nil {
			delete(set, k)
			if len(set) == 0 {
				delete(s.byPortal, portalRef)
			}
		}
		if v := s.fqdns[k]; v != nil {
			v.Portals = removeString(v.Portals, portalRef)
		}
	}
	if len(counts) == 0 {
		delete(s.perPortalCount, k)
		delete(s.fqdns, k)
	}
}

func sameTargets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mergeStrings(a, b []string) []string {
	out := append([]string(nil), a...)
	for _, x := range b {
		if !contains(out, x) {
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

func contains(haystack []string, needle string) bool {
	for _, x := range haystack {
		if x == needle {
			return true
		}
	}
	return false
}

func removeString(a []string, s string) []string {
	out := make([]string, 0, len(a))
	for _, x := range a {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}
