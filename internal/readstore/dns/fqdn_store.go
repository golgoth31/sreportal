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
	// losing maps each FQDNKey this record currently loses (its targets
	// disagree with the primary) to a fingerprint of those targets. Used to
	// emit a conflict event/metric only on transition — when a key newly
	// enters conflict or its losing targets change — instead of on every
	// idempotent Replace.
	losing map[FQDNKey]string
}

// FQDNStore is the in-memory implementation of dns.FQDNReader and dns.FQDNWriter.
type FQDNStore struct {
	mu        sync.RWMutex
	fqdns     map[FQDNKey]*domaindns.FQDNView
	byPortal  map[string]map[FQDNKey]struct{}
	byRecord  map[string]recordContribution
	winners   map[FQDNKey]string // FQDNKey -> recordKey of the primary contributor
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
		winners:   map[FQDNKey]string{},
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

	s.observeRefCounts(affected)
	s.updateDedupRatio(portalRef)
	if prev.portalRef != "" && prev.portalRef != portalRef {
		s.updateDedupRatio(prev.portalRef)
	}

	// Conflict detection: a contribution loses iff its targets disagree with
	// the recomputed primary. Lowest-seq contributor is primary; any
	// later-seq contributor with different targets is a loser.
	//
	// Emit an event/metric only on transition: a key newly losing, or losing
	// with different targets than last time. Without this guard a stable
	// conflict would be re-pushed on every (idempotent) Replace — inflating
	// the counter and evicting distinct conflicts from the bounded ring.
	newLosing := make(map[FQDNKey]string)
	for k, v := range newContributions {
		primary, ok := s.fqdns[k]
		if !ok {
			continue
		}
		if sameTargets(primary.Targets, v.Targets) {
			continue
		}
		fp := targetsKey(v.Targets)
		newLosing[k] = fp
		if prev.losing[k] == fp {
			continue // already reported with these targets — no transition
		}
		s.conflicts.Push(domaindns.ConflictEvent{
			FQDNKey:      domaindns.ConflictFQDNKey{Name: k.Name, RecordType: k.RecordType},
			WinnerRecord: s.winners[k],
			LoserRecord:  recordKey,
			PortalRef:    portalRef,
			At:           time.Now(),
		})
		metrics.DNSTargetsConflictTotal.WithLabelValues(portalRef).Inc()
	}
	// Persist the losing set so the next Replace can detect transitions.
	c := s.byRecord[recordKey]
	c.losing = newLosing
	s.byRecord[recordKey] = c

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
	affected := make(map[FQDNKey]struct{}, len(contrib.contributions))
	for k := range contrib.contributions {
		affected[k] = struct{}{}
		s.recomputeFQDN(k)
	}

	s.observeRefCounts(affected)
	s.updateDedupRatio(contrib.portalRef)

	go s.broadcast()
	return nil
}

// observeRefCounts samples the contributor count of each surviving key into
// the refcount histogram. Keys that no longer have contributors after
// recompute are skipped (they were purged from s.fqdns).
func (s *FQDNStore) observeRefCounts(keys map[FQDNKey]struct{}) {
	if len(keys) == 0 {
		return
	}
	counts := make(map[FQDNKey]int, len(keys))
	for _, rec := range s.byRecord {
		for k := range rec.contributions {
			if _, ok := keys[k]; ok {
				counts[k]++
			}
		}
	}
	for _, n := range counts {
		metrics.DNSFQDNRefCount.Observe(float64(n))
	}
}

// updateDedupRatio refreshes the per-portal dedup gauge:
// (raw_writes - unique_keys) / raw_writes, where raw_writes is the total
// number of contributions from DNSRecords with portalRef==p and unique_keys
// is the number of distinct (name, recordType) entries exposed for p.
// The gauge is removed when the portal no longer has contributions.
func (s *FQDNStore) updateDedupRatio(portalRef string) {
	if portalRef == "" {
		return
	}
	raw := 0
	for _, rec := range s.byRecord {
		if rec.portalRef != portalRef {
			continue
		}
		raw += len(rec.contributions)
	}
	if raw == 0 {
		metrics.DNSFQDNDedupRatio.DeleteLabelValues(portalRef)
		return
	}
	unique := len(s.byPortal[portalRef])
	metrics.DNSFQDNDedupRatio.WithLabelValues(portalRef).Set(float64(raw-unique) / float64(raw))
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
		recordKey string
	}
	var contributors []contrib
	portalsForKey := map[string]struct{}{}
	for recordKey, rec := range s.byRecord {
		if v, ok := rec.contributions[k]; ok {
			contributors = append(contributors, contrib{seq: rec.seq, view: v, portalRef: rec.portalRef, recordKey: recordKey})
			portalsForKey[rec.portalRef] = struct{}{}
		}
	}

	if len(contributors) == 0 {
		delete(s.fqdns, k)
		delete(s.winners, k)
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

	s.winners[k] = contributors[0].recordKey
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

// targetsKey returns an order-sensitive fingerprint of a target set, matching
// sameTargets semantics (targets are deterministic/sorted upstream). Used to
// detect when a losing record's targets change between reconciles. The NUL
// separator cannot appear in a DNS target, so distinct sets never collide.
func targetsKey(targets []string) string {
	return strings.Join(targets, "\x00")
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
