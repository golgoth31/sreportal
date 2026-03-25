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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
)

const maxRetries = 5

// Service manages Release CRs via the K8s API. It only handles the write path
// (AddEntry). The read path is served by the ReadStore.
type Service struct {
	client    client.Client
	namespace string
}

// NewService creates a new release Service.
func NewService(c client.Client, namespace string) *Service {
	return &Service{
		client:    c,
		namespace: namespace,
	}
}

// AddEntry appends a release entry to the day's CR, creating it if needed.
// Returns the day key, the total entry count after the append, whether a new CR
// was created, and any error.
func (s *Service) AddEntry(ctx context.Context, entry domainrelease.Entry) (string, int, bool, error) {
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
	var created bool
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
				return "", 0, false, fmt.Errorf("create release CR: %w", createErr)
			}
			count = 1
			created = true
		} else if err != nil {
			return "", 0, false, fmt.Errorf("get release CR: %w", err)
		} else {
			// Append to existing CR
			rel.Spec.Entries = append(rel.Spec.Entries, k8sEntry)
			if updateErr := s.client.Update(ctx, &rel); updateErr != nil {
				if apierrors.IsConflict(updateErr) && attempt < maxRetries-1 {
					continue
				}
				return "", 0, false, fmt.Errorf("update release CR: %w", updateErr)
			}
			count = len(rel.Spec.Entries)
		}

		// Update status
		rel.Status.EntryCount = count
		_ = s.client.Status().Update(ctx, &rel)

		return day, count, created, nil
	}

	return "", 0, false, fmt.Errorf("add release entry: max retries exceeded for day %s", day)
}
