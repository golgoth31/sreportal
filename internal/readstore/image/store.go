package image

import (
	"cmp"
	"context"
	"slices"
	"strings"

	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/readstore"
)

// Store implements ImageReader and ImageWriter over in-memory storage.
type Store struct {
	store *readstore.Store[domainimage.ImageView]
}

var (
	_ domainimage.ImageReader = (*Store)(nil)
	_ domainimage.ImageWriter = (*Store)(nil)
)

func NewStore() *Store {
	return &Store{store: readstore.New[domainimage.ImageView]()}
}

func (s *Store) Replace(_ context.Context, portalRef string, images []domainimage.ImageView) error {
	s.store.Replace(portalRef, deduplicate(images))
	return nil
}

func (s *Store) Delete(_ context.Context, portalRef string) error {
	s.store.Delete(portalRef)
	return nil
}

func (s *Store) List(_ context.Context, filters domainimage.ImageFilters) ([]domainimage.ImageView, error) {
	all := s.store.All()
	out := make([]domainimage.ImageView, 0, len(all))
	search := strings.ToLower(filters.Search)
	for _, img := range all {
		if filters.Portal != "" && img.PortalRef != filters.Portal {
			continue
		}
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

func (s *Store) Count(ctx context.Context, filters domainimage.ImageFilters) (int, error) {
	out, err := s.List(ctx, filters)
	return len(out), err
}

func (s *Store) Subscribe() <-chan struct{} {
	return s.store.Subscribe()
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
