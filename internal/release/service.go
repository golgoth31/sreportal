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

package release

import (
	"context"
	"fmt"
	"sort"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
)

const maxRetries = 5

// CachedDay holds cached entries for a single day.
type CachedDay struct {
	Entries         []sreportalv1alpha1.ReleaseEntry
	ResourceVersion string
}

// Service manages Release CRs with in-memory caching and thread-safe appends.
type Service struct {
	client    client.Client
	namespace string

	mu    sync.RWMutex
	cache map[string]*CachedDay

	daysMu    sync.RWMutex
	daysCache []string
	daysValid bool
}

// NewService creates a new release Service.
func NewService(c client.Client, namespace string) *Service {
	return &Service{
		client:    c,
		namespace: namespace,
		cache:     make(map[string]*CachedDay),
	}
}

// InvalidateDay removes a single day from the entries cache.
// Called by the Release controller when a CR is created, updated, or deleted.
func (s *Service) InvalidateDay(day string) {
	s.mu.Lock()
	delete(s.cache, day)
	s.mu.Unlock()
}

// InvalidateDays marks the days list cache as stale.
// Called by the Release controller when any CR changes.
func (s *Service) InvalidateDays() {
	s.daysMu.Lock()
	s.daysValid = false
	s.daysMu.Unlock()
}

// AddEntry appends a release entry to the day's CR, creating it if needed.
// Returns the day key and the total entry count after the append.
func (s *Service) AddEntry(ctx context.Context, entry domainrelease.Entry) (string, int, error) {
	day := entry.DateKey()
	crName := entry.CRName()
	nn := types.NamespacedName{Name: crName, Namespace: s.namespace}

	k8sEntry := sreportalv1alpha1.ReleaseEntry{
		Type:    entry.Type,
		Version: entry.Version,
		Origin:  entry.Origin,
		Date:    metav1.NewTime(entry.Date),
		Author:  entry.Author,
		Message: entry.Message,
		Link:    entry.Link,
	}

	var count int
	for attempt := range maxRetries {
		var rel sreportalv1alpha1.Release
		err := s.client.Get(ctx, nn, &rel)

		if apierrors.IsNotFound(err) {
			// Create new CR
			rel = sreportalv1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crName,
					Namespace: s.namespace,
				},
				Spec: sreportalv1alpha1.ReleaseSpec{
					Entries: []sreportalv1alpha1.ReleaseEntry{k8sEntry},
				},
			}
			if createErr := s.client.Create(ctx, &rel); createErr != nil {
				if apierrors.IsAlreadyExists(createErr) {
					continue // Race: another request created it, retry as append
				}
				return "", 0, fmt.Errorf("create release CR: %w", createErr)
			}
			count = 1
		} else if err != nil {
			return "", 0, fmt.Errorf("get release CR: %w", err)
		} else {
			// Append to existing CR
			rel.Spec.Entries = append(rel.Spec.Entries, k8sEntry)
			if updateErr := s.client.Update(ctx, &rel); updateErr != nil {
				if apierrors.IsConflict(updateErr) && attempt < maxRetries-1 {
					continue
				}
				return "", 0, fmt.Errorf("update release CR: %w", updateErr)
			}
			count = len(rel.Spec.Entries)
		}

		// Update status
		rel.Status.EntryCount = count
		_ = s.client.Status().Update(ctx, &rel)

		// Invalidate cache
		s.mu.Lock()
		delete(s.cache, day)
		s.mu.Unlock()

		return day, count, nil
	}

	return "", 0, fmt.Errorf("add release entry: max retries exceeded for day %s", day)
}

// ListEntries returns all entries for a given day (YYYY-MM-DD). Cache-first.
func (s *Service) ListEntries(ctx context.Context, day string) ([]sreportalv1alpha1.ReleaseEntry, error) {
	// Check cache first
	s.mu.RLock()
	cached, ok := s.cache[day]
	s.mu.RUnlock()
	if ok {
		return cached.Entries, nil
	}

	// Fetch from K8s
	crName := "release-" + day
	nn := types.NamespacedName{Name: crName, Namespace: s.namespace}
	var rel sreportalv1alpha1.Release
	if err := s.client.Get(ctx, nn, &rel); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("day %s: %w", day, domainrelease.ErrNotFound)
		}
		return nil, fmt.Errorf("get release CR: %w", err)
	}

	// Populate cache
	s.mu.Lock()
	s.cache[day] = &CachedDay{
		Entries:         rel.Spec.Entries,
		ResourceVersion: rel.ResourceVersion,
	}
	s.mu.Unlock()

	return rel.Spec.Entries, nil
}

// ListDays returns all days that have Release CRs, sorted ascending. Cache-first.
func (s *Service) ListDays(ctx context.Context) ([]string, error) {
	s.daysMu.RLock()
	if s.daysValid {
		days := s.daysCache
		s.daysMu.RUnlock()
		return days, nil
	}
	s.daysMu.RUnlock()

	var list sreportalv1alpha1.ReleaseList
	if err := s.client.List(ctx, &list, client.InNamespace(s.namespace)); err != nil {
		return nil, fmt.Errorf("list release CRs: %w", err)
	}

	days := make([]string, 0, len(list.Items))
	for _, rel := range list.Items {
		day, err := domainrelease.ParseDateFromCRName(rel.Name)
		if err != nil {
			continue // Skip malformed names
		}
		days = append(days, day)
	}
	sort.Strings(days)

	s.daysMu.Lock()
	s.daysCache = days
	s.daysValid = true
	s.daysMu.Unlock()

	return days, nil
}
