package image

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"
	"sync"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

// Store is an in-memory image projection keyed by portalRef and then by
// WorkloadKey so that a single workload event touches only its own slot.
type Store struct {
	mu   sync.RWMutex
	data map[string]map[domainimage.WorkloadKey][]domainimage.ImageView

	notifyMu sync.Mutex
	notifyCh chan struct{}
}

var (
	_ domainimage.ImageReader = (*Store)(nil)
	_ domainimage.ImageWriter = (*Store)(nil)
)

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{
		data:     make(map[string]map[domainimage.WorkloadKey][]domainimage.ImageView),
		notifyCh: make(chan struct{}),
	}
}

// ReplaceWorkload implements ImageWriter.
func (s *Store) ReplaceWorkload(_ context.Context, portalRef string, wk domainimage.WorkloadKey, images []domainimage.ImageView) error {
	s.mu.Lock()
	portal, ok := s.data[portalRef]
	if !ok {
		portal = make(map[domainimage.WorkloadKey][]domainimage.ImageView)
		s.data[portalRef] = portal
	}
	if len(images) == 0 {
		delete(portal, wk)
	} else {
		portal[wk] = images
	}
	s.mu.Unlock()

	s.broadcast()
	return nil
}

// DeleteWorkloadAllPortals implements ImageWriter.
func (s *Store) DeleteWorkloadAllPortals(_ context.Context, wk domainimage.WorkloadKey) error {
	s.mu.Lock()
	for _, portal := range s.data {
		delete(portal, wk)
	}
	s.mu.Unlock()

	s.broadcast()
	return nil
}

// ReplaceAll implements ImageWriter.
func (s *Store) ReplaceAll(_ context.Context, portalRef string, byWorkload map[domainimage.WorkloadKey][]domainimage.ImageView) error {
	// Defensive copy so the caller can't mutate the stored map after the call.
	copyMap := make(map[domainimage.WorkloadKey][]domainimage.ImageView, len(byWorkload))
	maps.Copy(copyMap, byWorkload)

	s.mu.Lock()
	s.data[portalRef] = copyMap
	s.mu.Unlock()

	s.broadcast()
	return nil
}

// DeletePortal implements ImageWriter.
func (s *Store) DeletePortal(_ context.Context, portalRef string) error {
	s.mu.Lock()
	delete(s.data, portalRef)
	s.mu.Unlock()

	s.broadcast()
	return nil
}

// List implements ImageReader. Returns a deduplicated, sorted view of all
// workload contributions that match the filters.
func (s *Store) List(_ context.Context, filters domainimage.ImageFilters) ([]domainimage.ImageView, error) {
	s.mu.RLock()
	collected := make([]domainimage.ImageView, 0)
	for portalRef, byWorkload := range s.data {
		if filters.Portal != "" && portalRef != filters.Portal {
			continue
		}
		for _, views := range byWorkload {
			collected = append(collected, views...)
		}
	}
	s.mu.RUnlock()

	out := make([]domainimage.ImageView, 0, len(collected))
	search := strings.ToLower(filters.Search)
	for _, img := range collected {
		if filters.Registry != "" && img.Registry != filters.Registry {
			continue
		}
		if filters.TagType != "" && string(img.TagType) != filters.TagType {
			continue
		}
		if filters.Search != "" && !strings.Contains(strings.ToLower(img.Repository), search) {
			continue
		}
		out = append(out, img)
	}

	out = deduplicate(out)

	slices.SortFunc(out, func(a, b domainimage.ImageView) int {
		if c := cmp.Compare(a.Registry, b.Registry); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Repository, b.Repository); c != 0 {
			return c
		}
		return cmp.Compare(a.Tag, b.Tag)
	})
	return out, nil
}

// Count implements ImageReader.
func (s *Store) Count(ctx context.Context, filters domainimage.ImageFilters) (int, error) {
	out, err := s.List(ctx, filters)
	return len(out), err
}

// Subscribe returns a channel closed on the next mutation.
func (s *Store) Subscribe() <-chan struct{} {
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()
	return s.notifyCh
}

func (s *Store) broadcast() {
	s.notifyMu.Lock()
	old := s.notifyCh
	s.notifyCh = make(chan struct{})
	s.notifyMu.Unlock()
	close(old)
}

func deduplicate(images []domainimage.ImageView) []domainimage.ImageView {
	type k struct{ registry, repository, tag string }
	seen := make(map[k]int, len(images))
	out := make([]domainimage.ImageView, 0, len(images))
	for _, img := range images {
		key := k{registry: img.Registry, repository: img.Repository, tag: img.Tag}
		if idx, ok := seen[key]; ok {
			out[idx].Workloads = append(out[idx].Workloads, img.Workloads...)
			continue
		}
		seen[key] = len(out)
		out = append(out, img)
	}
	return out
}
