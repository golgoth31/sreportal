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

package release_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainrelease "github.com/golgoth31/sreportal/internal/domain/release"
	releaseservice "github.com/golgoth31/sreportal/internal/release"
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = sreportalv1alpha1.AddToScheme(s)
	return s
}

func TestAddEntry_CreatesNewCR(t *testing.T) {
	ctx := context.Background()
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()
	svc := releaseservice.NewService(k8sClient, "default")

	entry, err := domainrelease.NewEntry("deployment", "v1.0.0", "ci/cd", time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	day, count, created, err := svc.AddEntry(ctx, entry)

	require.NoError(t, err)
	assert.Equal(t, "2026-03-21", day)
	assert.Equal(t, 1, count)
	assert.True(t, created)

	// Verify CR was created
	var rel sreportalv1alpha1.Release
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "release-2026-03-21", Namespace: "default"}, &rel)
	require.NoError(t, err)
	assert.Len(t, rel.Spec.Entries, 1)
	assert.Equal(t, "deployment", rel.Spec.Entries[0].Type)
	assert.Equal(t, "v1.0.0", rel.Spec.Entries[0].Version)
}

func TestAddEntry_AppendsToExistingCR(t *testing.T) {
	ctx := context.Background()
	scheme := newScheme()

	existing := &sreportalv1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "release-2026-03-21",
			Namespace: "default",
		},
		Spec: sreportalv1alpha1.ReleaseSpec{
			Entries: []sreportalv1alpha1.ReleaseEntry{
				{
					Type:    "deployment",
					Version: "v1.0.0",
					Origin:  "ci/cd",
					Date:    metav1.NewTime(time.Date(2026, 3, 21, 8, 0, 0, 0, time.UTC)),
				},
			},
		},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).WithStatusSubresource(&sreportalv1alpha1.Release{}).Build()
	svc := releaseservice.NewService(k8sClient, "default")

	entry, err := domainrelease.NewEntry("rollback", "v0.9.0", "manual", time.Date(2026, 3, 21, 14, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	day, count, created, err := svc.AddEntry(ctx, entry)

	require.NoError(t, err)
	assert.Equal(t, "2026-03-21", day)
	assert.Equal(t, 2, count)
	assert.False(t, created)

	// Verify CR was updated
	var rel sreportalv1alpha1.Release
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "release-2026-03-21", Namespace: "default"}, &rel)
	require.NoError(t, err)
	assert.Len(t, rel.Spec.Entries, 2)
	assert.Equal(t, "rollback", rel.Spec.Entries[1].Type)
}
