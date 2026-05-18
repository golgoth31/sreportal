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

// recordContribution captures everything a single DNSRecord contributes.
// `seq` is monotonic per FIRST Replace and used to break ties when picking
// the primary contributor for an (FQDN, recordType): lowest-seq wins, which
// preserves "first writer wins" semantics across Replace order.
type recordContribution struct {
	seq           uint64
	contributions map[FQDNKey]domaindns.FQDNView
	portalRef     string
	dnsNamespace  string
	dnsName       string
}

// FQDNStore is the in-memory implementation of dns.FQDNReader and dns.FQDNWriter.
type FQDNStore struct {
	mu        sync.RWMutex
	fqdns     map[FQDNKey]*domaindns.FQDNView
	byPortal  map[string]map[FQDNKey]struct{}
	byRecord  map[string]recordContribution
	seqCount  uint64
	conflicts *conflictRing

	notifyMu sync.Mutex
	notifyCh chan struct{}
}

// NewFQDNStore returns an empty FQDNStore. Source priority is enforced
// upstream (per-DNS-CR at DNSRecord generation time); the store treats
// each Replace call as authoritative for its (recordKey, portalRef) tuple.
func NewFQDNStore() *FQDNStore {
	return &FQDNStore{
		fqdns:     map[FQDNKey]*domaindns.FQDNView{},
		byPortal:  map[string]map[FQDNKey]struct{}{},
		byRecord:  map[string]recordContribution{},
		conflicts: newConflictRing(256),
		notifyCh:  make(chan struct{}),
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

	prev := s.byRecord[recordKey]
	seq := prev.seq
	if prev.contributions == nil {
		s.seqCount++
		seq = s.seqCount
	}

	newContributions := make(map[FQDNKey]domaindns.FQDNView, len(fqdns))
	for _, v := range fqdns {
		k := FQDNKey{Name: v.Name, RecordType: v.RecordType}
		newContributions[k] = v
	}

	affected := make(map[FQDNKey]struct{}, len(prev.contributions)+len(newContributions))
	for k := range prev.contributions {
		affected[k] = struct{}{}
	}
	for k := range newContributions {
		affected[k] = struct{}{}
	}

	s.byRecord[recordKey] = recordContribution{
		seq:           seq,
		contributions: newContributions,
		portalRef:     portalRef,
		dnsNamespace:  prev.dnsNamespace,
		dnsName:       prev.dnsName,
	}

	for k := range affected {
		s.recomputeFQDN(k)
	}

	// Conflict detection: a contribution loses iff its targets disagree with
	// the recomputed primary. Lowest-seq contributor is primary; any
	// later-seq contributor with different targets is a loser.
	for k, v := range newContributions {
		primary, ok := s.fqdns[k]
		if !ok {
			continue
		}
		if !sameTargets(primary.Targets, v.Targets) {
			s.conflicts.Push(domaindns.ConflictEvent{
				FQDNKey:     domaindns.ConflictFQDNKey{Name: k.Name, RecordType: k.RecordType},
				LoserRecord: recordKey,
				PortalRef:   portalRef,
				At:          time.Now(),
			})
			metrics.DNSTargetsConflictTotal.WithLabelValues(portalRef).Inc()
		}
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
	delete(s.byRecord, recordKey)
	for k := range contrib.contributions {
		s.recomputeFQDN(k)
	}

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
// with the source. The store's writers rebuild Portals/Groups/Targets on every
// recompute, so callers must hold their own copies to be safe across
// subsequent mutations.
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

// recomputeFQDN rebuilds s.fqdns[k] from all current contributors. The
// primary (lowest seq) provides Targets/SyncStatus/OriginRef/Description and
// every other scalar field; Groups and Portals are derived from the full
// contributor set. If no contributors remain, the key is purged from fqdns
// and every byPortal index.
func (s *FQDNStore) recomputeFQDN(k FQDNKey) {
	type contrib struct {
		seq       uint64
		view      domaindns.FQDNView
		portalRef string
	}
	var contributors []contrib
	portalsForKey := map[string]struct{}{}
	for _, rec := range s.byRecord {
		if v, ok := rec.contributions[k]; ok {
			contributors = append(contributors, contrib{seq: rec.seq, view: v, portalRef: rec.portalRef})
			portalsForKey[rec.portalRef] = struct{}{}
		}
	}

	if len(contributors) == 0 {
		delete(s.fqdns, k)
		for p, set := range s.byPortal {
			if _, in := set[k]; in {
				delete(set, k)
				if len(set) == 0 {
					delete(s.byPortal, p)
				}
			}
		}
		return
	}

	sort.Slice(contributors, func(i, j int) bool { return contributors[i].seq < contributors[j].seq })

	primary := contributors[0].view
	groupSet := map[string]struct{}{}
	for _, c := range contributors {
		for _, g := range c.view.Groups {
			groupSet[g] = struct{}{}
		}
	}
	primary.Groups = sortedKeys(groupSet)
	primary.Portals = sortedKeys(portalsForKey)
	s.fqdns[k] = &primary

	for p, set := range s.byPortal {
		if _, kept := portalsForKey[p]; kept {
			continue
		}
		if _, in := set[k]; in {
			delete(set, k)
			if len(set) == 0 {
				delete(s.byPortal, p)
			}
		}
	}
	for p := range portalsForKey {
		if s.byPortal[p] == nil {
			s.byPortal[p] = map[FQDNKey]struct{}{}
		}
		s.byPortal[p][k] = struct{}{}
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

func sortedKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
