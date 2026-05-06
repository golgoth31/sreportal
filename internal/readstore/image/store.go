package image

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
)

// scopeKey identifies a single (portal, host, namespace) contribution scope.
type scopeKey struct{ Portal, Host, Namespace string }

// entryKey identifies a deduplicated image entry inside a portal.
type entryKey struct{ Registry, Repo, Tag string }

// Store is an in-memory image projection. It maintains:
//   - per-scope contributions, indexed by (portal, host, namespace),
//   - aggregated views per portal, where contributions to the same
//     (registry, repo, tag) are merged across namespaces.
//
// The single writer in production is the ImageRegistry controller; remote
// portals use the same writer interface to project shadow contributions.
type Store struct {
	mu sync.RWMutex

	// contributions[scope][entryKey] = ImageView contributed by that scope.
	contributions map[scopeKey]map[entryKey]domainimage.ImageView
	// aggregated[portalRef][entryKey] = ImageView merged across all scopes
	// of that portal (workload list union; latest-version metadata wins by
	// LatestCheckedAt).
	aggregated map[string]map[entryKey]domainimage.ImageView

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
		contributions: make(map[scopeKey]map[entryKey]domainimage.ImageView),
		aggregated:    make(map[string]map[entryKey]domainimage.ImageView),
		notifyCh:      make(chan struct{}),
	}
}

// ReplaceForNamespace implements ImageWriter.
//
// It replaces all contributions of (portalRef, host, namespace) with `views`
// and re-aggregates the affected portal. Empty `views` means "drop the scope".
func (s *Store) ReplaceForNamespace(_ context.Context, portalRef, host, namespace string, views []domainimage.ImageView) error {
	scope := scopeKey{Portal: portalRef, Host: host, Namespace: namespace}

	s.mu.Lock()
	if len(views) == 0 {
		delete(s.contributions, scope)
	} else {
		bucket := make(map[entryKey]domainimage.ImageView, len(views))
		for _, v := range views {
			k := entryKey{Registry: v.Registry, Repo: v.Repository, Tag: v.Tag}
			// Merge duplicates inside the same scope (defensive).
			if existing, ok := bucket[k]; ok {
				bucket[k] = mergeViews(existing, v)
				continue
			}
			bucket[k] = v
		}
		s.contributions[scope] = bucket
	}
	s.recomputePortalLocked(portalRef)
	s.mu.Unlock()

	s.broadcast()
	return nil
}

// RemoveForNamespace implements ImageWriter. It is a convenience wrapper for
// ReplaceForNamespace(..., nil).
func (s *Store) RemoveForNamespace(ctx context.Context, portalRef, host, namespace string) error {
	return s.ReplaceForNamespace(ctx, portalRef, host, namespace, nil)
}

// recomputePortalLocked rebuilds aggregated[portalRef] from scratch by walking
// every contribution scope of that portal. Caller must hold s.mu.
func (s *Store) recomputePortalLocked(portalRef string) {
	agg := make(map[entryKey]domainimage.ImageView)
	for scope, bucket := range s.contributions {
		if scope.Portal != portalRef {
			continue
		}
		for k, v := range bucket {
			if existing, ok := agg[k]; ok {
				agg[k] = mergeViews(existing, v)
				continue
			}
			// Defensive copy of Workloads slice so later merges don't mutate
			// the contribution-side bucket.
			cp := v
			cp.Workloads = append([]domainimage.WorkloadRef(nil), v.Workloads...)
			agg[k] = cp
		}
	}
	if len(agg) == 0 {
		delete(s.aggregated, portalRef)
		return
	}
	s.aggregated[portalRef] = agg
}

// mergeViews unions workloads of two views with the same entryKey and picks
// the most recent latest-version metadata (by LatestCheckedAt). All other
// scalar fields default to `b`'s value when `a`'s is empty.
func mergeViews(a, b domainimage.ImageView) domainimage.ImageView {
	out := a
	out.Workloads = append([]domainimage.WorkloadRef(nil), a.Workloads...)
	out.Workloads = append(out.Workloads, b.Workloads...)

	// Prefer non-empty scalar fields.
	if out.OriginalImage == "" {
		out.OriginalImage = b.OriginalImage
	}
	if out.MutatedImage == "" {
		out.MutatedImage = b.MutatedImage
	}
	if out.ChangeType == "" {
		out.ChangeType = b.ChangeType
	}
	if out.TagType == "" {
		out.TagType = b.TagType
	}

	// Latest-version metadata: the contribution with the most recent
	// LastCheckedAt wins. If only one side has a timestamp, that side wins.
	switch {
	case a.LatestCheckedAt == nil && b.LatestCheckedAt != nil:
		out.LatestVersion = b.LatestVersion
		out.LatestCheckedAt = b.LatestCheckedAt
		out.LatestError = b.LatestError
		out.UpgradeAvailable = b.UpgradeAvailable
	case a.LatestCheckedAt != nil && b.LatestCheckedAt != nil && b.LatestCheckedAt.After(*a.LatestCheckedAt):
		out.LatestVersion = b.LatestVersion
		out.LatestCheckedAt = b.LatestCheckedAt
		out.LatestError = b.LatestError
		out.UpgradeAvailable = b.UpgradeAvailable
	}
	return out
}

// List implements ImageReader. It returns the deduplicated and sorted image
// projections matching the filters.
func (s *Store) List(_ context.Context, filters domainimage.ImageFilters) ([]domainimage.ImageView, error) {
	s.mu.RLock()
	collected := make([]domainimage.ImageView, 0)
	for portalRef, bucket := range s.aggregated {
		if filters.Portal != "" && portalRef != filters.Portal {
			continue
		}
		for _, v := range bucket {
			collected = append(collected, copyView(v))
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

// Subscribe returns a channel that is closed on the next mutation.
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

// copyView returns a deep-enough copy: scalar fields are copied by value, the
// Workloads slice is reallocated so callers may not mutate the cache.
func copyView(v domainimage.ImageView) domainimage.ImageView {
	out := v
	out.Workloads = append([]domainimage.WorkloadRef(nil), v.Workloads...)
	return out
}
