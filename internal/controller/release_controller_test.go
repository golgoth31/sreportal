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

package controller_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller"
)

const defaultTTL = 30 * 24 * time.Hour

type fakeCacheInvalidator struct {
	invalidatedDays []string
	daysInvalidated int
}

func (f *fakeCacheInvalidator) InvalidateDay(day string) {
	f.invalidatedDays = append(f.invalidatedDays, day)
}

func (f *fakeCacheInvalidator) InvalidateDays() {
	f.daysInvalidated++
}

func newReleaseScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = sreportalv1alpha1.AddToScheme(s)
	return s
}

func TestReleaseReconciler_Reconcile_InvalidatesCacheAndRequeues(t *testing.T) {
	existing := &sreportalv1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{Name: "release-2026-03-21", Namespace: "default"},
	}
	c := fake.NewClientBuilder().WithScheme(newReleaseScheme()).WithObjects(existing).Build()
	cache := &fakeCacheInvalidator{}
	r := controller.NewReleaseReconciler(c, cache, defaultTTL)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "release-2026-03-21", Namespace: "default"},
	})

	require.NoError(t, err)
	assert.Equal(t, 12*time.Hour, result.RequeueAfter)
	assert.Equal(t, []string{"2026-03-21"}, cache.invalidatedDays)
	assert.Equal(t, 1, cache.daysInvalidated)
}

func TestReleaseReconciler_Reconcile_DeletedCR_InvalidatesCache(t *testing.T) {
	// CR does not exist (was deleted)
	c := fake.NewClientBuilder().WithScheme(newReleaseScheme()).Build()
	cache := &fakeCacheInvalidator{}
	r := controller.NewReleaseReconciler(c, cache, defaultTTL)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "release-2026-03-21", Namespace: "default"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.Equal(t, []string{"2026-03-21"}, cache.invalidatedDays)
	assert.Equal(t, 1, cache.daysInvalidated)
}

func TestReleaseReconciler_Reconcile_SkipsUnparseableName(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newReleaseScheme()).Build()
	cache := &fakeCacheInvalidator{}
	r := controller.NewReleaseReconciler(c, cache, defaultTTL)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "not-a-release", Namespace: "default"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.Empty(t, cache.invalidatedDays)
	assert.Equal(t, 0, cache.daysInvalidated)
}

func TestReleaseReconciler_Reconcile_DeletesExpiredCR(t *testing.T) {
	// Create a CR for a day that's 60 days ago
	existing := &sreportalv1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{Name: "release-2025-01-01", Namespace: "default"},
	}
	c := fake.NewClientBuilder().WithScheme(newReleaseScheme()).WithObjects(existing).Build()
	cache := &fakeCacheInvalidator{}
	r := controller.NewReleaseReconciler(c, cache, defaultTTL)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "release-2025-01-01", Namespace: "default"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result) // No requeue — CR was deleted

	// Verify CR was deleted
	var rel sreportalv1alpha1.Release
	err = c.Get(context.Background(), types.NamespacedName{Name: "release-2025-01-01", Namespace: "default"}, &rel)
	assert.True(t, err != nil, "expected CR to be deleted")
}

func TestReleaseReconciler_Reconcile_PreservesNonExpiredCR(t *testing.T) {
	// Create a CR for today
	today := time.Now().UTC().Format("2006-01-02")
	crName := "release-" + today
	existing := &sreportalv1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: "default"},
	}
	c := fake.NewClientBuilder().WithScheme(newReleaseScheme()).WithObjects(existing).Build()
	cache := &fakeCacheInvalidator{}
	r := controller.NewReleaseReconciler(c, cache, defaultTTL)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: crName, Namespace: "default"},
	})

	require.NoError(t, err)
	assert.Equal(t, 12*time.Hour, result.RequeueAfter) // Requeued, not deleted

	// Verify CR still exists
	var rel sreportalv1alpha1.Release
	err = c.Get(context.Background(), types.NamespacedName{Name: crName, Namespace: "default"}, &rel)
	require.NoError(t, err)
}
