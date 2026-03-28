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
	releasereadstore "github.com/golgoth31/sreportal/internal/readstore/release"
)

const (
	defaultTTL = 30 * 24 * time.Hour
	testPortal = "main"
	testDay    = "2026-03-21"
)

func newReleaseScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = sreportalv1alpha1.AddToScheme(s)
	return s
}

func testPortalCR() *sreportalv1alpha1.Portal {
	return &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: testPortal, Namespace: "default"},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Main"},
	}
}

func TestReleaseReconciler_Reconcile_PushesEntriesToStore(t *testing.T) {
	ctx := context.Background()
	existing := &sreportalv1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{Name: "release-2026-03-21", Namespace: "default"},
		Spec: sreportalv1alpha1.ReleaseSpec{
			PortalRef: testPortal,
			Entries: []sreportalv1alpha1.ReleaseEntry{
				{
					Type:    "deployment",
					Version: "v1.0.0",
					Origin:  "ci/cd",
					Date:    metav1.NewTime(time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)),
					Author:  "alice",
				},
				{
					Type:    "rollback",
					Version: "v0.9.0",
					Origin:  "manual",
					Date:    metav1.NewTime(time.Date(2026, 3, 21, 14, 0, 0, 0, time.UTC)),
				},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(newReleaseScheme()).WithObjects(testPortalCR(), existing).Build()
	store := releasereadstore.NewReleaseStore()
	r := controller.NewReleaseReconciler(c, defaultTTL)
	r.SetReleaseWriter(store)

	result, err := r.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "release-2026-03-21", Namespace: "default"},
	})

	require.NoError(t, err)
	assert.Equal(t, 12*time.Hour, result.RequeueAfter)

	entries, err := store.ListEntries(ctx, testDay, testPortal)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "deployment", entries[0].Type)
	assert.Equal(t, "v1.0.0", entries[0].Version)
	assert.Equal(t, "alice", entries[0].Author)
	assert.Equal(t, "rollback", entries[1].Type)
}

func TestReleaseReconciler_Reconcile_ReleasesDisabled_SkipsProjection(t *testing.T) {
	ctx := context.Background()
	releasesFalse := false
	portal := testPortalCR()
	portal.Spec.Features = &sreportalv1alpha1.PortalFeatures{Releases: &releasesFalse}
	rel := &sreportalv1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{Name: "release-2026-03-21", Namespace: "default"},
		Spec: sreportalv1alpha1.ReleaseSpec{
			PortalRef: testPortal,
			Entries: []sreportalv1alpha1.ReleaseEntry{
				{Type: "deployment", Origin: "ci", Date: metav1.NewTime(time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC))},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(newReleaseScheme()).WithObjects(portal, rel).Build()
	store := releasereadstore.NewReleaseStore()

	r := controller.NewReleaseReconciler(c, defaultTTL)
	r.SetReleaseWriter(store)

	result, err := r.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "release-2026-03-21", Namespace: "default"},
	})
	require.NoError(t, err)
	assert.Equal(t, 12*time.Hour, result.RequeueAfter)

	// Controller does not call Replace when disabled — store stays empty
	entries, err := store.ListEntries(ctx, testDay, testPortal)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestReleaseReconciler_Reconcile_DeletedCR_RemovesFromStore(t *testing.T) {
	ctx := context.Background()
	c := fake.NewClientBuilder().WithScheme(newReleaseScheme()).Build()
	store := releasereadstore.NewReleaseStore()

	r := controller.NewReleaseReconciler(c, defaultTTL)
	r.SetReleaseWriter(store)

	result, err := r.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "release-2026-03-21", Namespace: "default"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	entries, err := store.ListEntries(ctx, testDay, "")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestReleaseReconciler_Reconcile_SkipsUnparseableName(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newReleaseScheme()).Build()
	store := releasereadstore.NewReleaseStore()
	r := controller.NewReleaseReconciler(c, defaultTTL)
	r.SetReleaseWriter(store)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "not-a-release", Namespace: "default"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	days, err := store.ListDays(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, days)
}

func TestReleaseReconciler_Reconcile_DeletesExpiredCR(t *testing.T) {
	existing := &sreportalv1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{Name: "release-2025-01-01", Namespace: "default"},
		Spec:       sreportalv1alpha1.ReleaseSpec{PortalRef: testPortal},
	}
	c := fake.NewClientBuilder().WithScheme(newReleaseScheme()).WithObjects(testPortalCR(), existing).Build()
	store := releasereadstore.NewReleaseStore()
	r := controller.NewReleaseReconciler(c, defaultTTL)
	r.SetReleaseWriter(store)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "release-2025-01-01", Namespace: "default"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	var rel sreportalv1alpha1.Release
	err = c.Get(context.Background(), types.NamespacedName{Name: "release-2025-01-01", Namespace: "default"}, &rel)
	assert.True(t, err != nil, "expected CR to be deleted")
}

func TestReleaseReconciler_Reconcile_PreservesNonExpiredCR(t *testing.T) {
	ctx := context.Background()
	today := time.Now().UTC().Format("2006-01-02")
	crName := "release-" + today
	existing := &sreportalv1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: "default"},
		Spec:       sreportalv1alpha1.ReleaseSpec{PortalRef: testPortal},
	}
	c := fake.NewClientBuilder().WithScheme(newReleaseScheme()).WithObjects(testPortalCR(), existing).Build()
	store := releasereadstore.NewReleaseStore()
	r := controller.NewReleaseReconciler(c, defaultTTL)
	r.SetReleaseWriter(store)

	result, err := r.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: crName, Namespace: "default"},
	})

	require.NoError(t, err)
	assert.Equal(t, 12*time.Hour, result.RequeueAfter)

	var rel sreportalv1alpha1.Release
	err = c.Get(ctx, types.NamespacedName{Name: crName, Namespace: "default"}, &rel)
	require.NoError(t, err)
}

func TestReleaseReconciler_Reconcile_PortalNotFound_ReturnsError(t *testing.T) {
	ctx := context.Background()
	rel := &sreportalv1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{Name: "release-2026-03-21", Namespace: "default"},
		Spec: sreportalv1alpha1.ReleaseSpec{
			PortalRef: "missing",
			Entries: []sreportalv1alpha1.ReleaseEntry{
				{Type: "deployment", Origin: "ci", Date: metav1.NewTime(time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC))},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(newReleaseScheme()).WithObjects(rel).Build()
	r := controller.NewReleaseReconciler(c, defaultTTL)
	r.SetReleaseWriter(releasereadstore.NewReleaseStore())

	_, err := r.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "release-2026-03-21", Namespace: "default"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get portal")
}
