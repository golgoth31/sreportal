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

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

func TestPortalReleasesFeatureWakeupPredicate_Create(t *testing.T) {
	p := PortalReleasesFeatureWakeupPredicate()

	t.Run("features nil defaults enabled", func(t *testing.T) {
		portal := &sreportalv1alpha1.Portal{
			ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
			Spec:       sreportalv1alpha1.PortalSpec{Title: "t"},
		}
		assert.True(t, p.Create(event.CreateEvent{Object: portal}))
	})

	t.Run("releases explicitly true", func(t *testing.T) {
		portal := &sreportalv1alpha1.Portal{
			ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
			Spec: sreportalv1alpha1.PortalSpec{
				Title:    "t",
				Features: &sreportalv1alpha1.PortalFeatures{Releases: new(true)},
			},
		}
		assert.True(t, p.Create(event.CreateEvent{Object: portal}))
	})

	t.Run("releases explicitly false", func(t *testing.T) {
		portal := &sreportalv1alpha1.Portal{
			ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
			Spec: sreportalv1alpha1.PortalSpec{
				Title:    "t",
				Features: &sreportalv1alpha1.PortalFeatures{Releases: new(false)},
			},
		}
		assert.False(t, p.Create(event.CreateEvent{Object: portal}))
	})

	t.Run("wrong type", func(t *testing.T) {
		assert.False(t, p.Create(event.CreateEvent{Object: &sreportalv1alpha1.Release{}}))
	})
}

func TestPortalReleasesFeatureWakeupPredicate_Update(t *testing.T) {
	p := PortalReleasesFeatureWakeupPredicate()

	baseOld := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", ResourceVersion: "1"},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:    "t",
			Features: &sreportalv1alpha1.PortalFeatures{Releases: new(false)},
		},
	}
	baseNew := baseOld.DeepCopy()
	baseNew.ResourceVersion = "2"
	baseNew.Spec.Features = &sreportalv1alpha1.PortalFeatures{Releases: new(true)}

	t.Run("disabled to enabled", func(t *testing.T) {
		assert.True(t, p.Update(event.UpdateEvent{ObjectOld: baseOld, ObjectNew: baseNew}))
	})

	t.Run("enabled to disabled", func(t *testing.T) {
		on := baseNew.DeepCopy()
		off := baseOld.DeepCopy()
		assert.False(t, p.Update(event.UpdateEvent{ObjectOld: on, ObjectNew: off}))
	})

	t.Run("stays disabled", func(t *testing.T) {
		a := baseOld.DeepCopy()
		b := baseOld.DeepCopy()
		b.ResourceVersion = "3"
		b.Spec.Title = "other"
		assert.False(t, p.Update(event.UpdateEvent{ObjectOld: a, ObjectNew: b}))
	})

	t.Run("stays enabled", func(t *testing.T) {
		a := baseNew.DeepCopy()
		b := baseNew.DeepCopy()
		b.ResourceVersion = "3"
		b.Spec.Title = "other"
		assert.False(t, p.Update(event.UpdateEvent{ObjectOld: a, ObjectNew: b}))
	})

	t.Run("nil features old disabled implied new enabled default", func(t *testing.T) {
		oldP := &sreportalv1alpha1.Portal{
			ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
			Spec: sreportalv1alpha1.PortalSpec{
				Title:    "t",
				Features: &sreportalv1alpha1.PortalFeatures{Releases: new(false)},
			},
		}
		newP := &sreportalv1alpha1.Portal{
			ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", ResourceVersion: "2"},
			Spec:       sreportalv1alpha1.PortalSpec{Title: "t"},
		}
		assert.True(t, p.Update(event.UpdateEvent{ObjectOld: oldP, ObjectNew: newP}))
	})
}

func TestPortalDNSEnabledWakeupPredicate_Update_DisabledToEnabled(t *testing.T) {
	p := PortalDNSEnabledWakeupPredicate()
	oldP := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", ResourceVersion: "1"},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:    "t",
			Features: &sreportalv1alpha1.PortalFeatures{DNS: new(false)},
		},
	}
	newP := oldP.DeepCopy()
	newP.ResourceVersion = "2"
	newP.Spec.Features = &sreportalv1alpha1.PortalFeatures{DNS: new(true)}
	assert.True(t, p.Update(event.UpdateEvent{ObjectOld: oldP, ObjectNew: newP}))
}

func TestPortalReleasesFeatureWakeupPredicate_DeleteAndGeneric(t *testing.T) {
	p := PortalReleasesFeatureWakeupPredicate()
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "t"},
	}
	assert.False(t, p.Delete(event.DeleteEvent{Object: portal}))
	assert.False(t, p.Generic(event.GenericEvent{Object: portal}))
}

func TestReleaseReconcileRequestsForPortal(t *testing.T) {
	ctx := context.Background()
	s := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(s))

	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Main"},
	}
	rel := &sreportalv1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{Name: "release-2026-03-21", Namespace: "default"},
		Spec:       sreportalv1alpha1.ReleaseSpec{PortalRef: "main"},
	}

	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(portal, rel).
		WithIndex(&sreportalv1alpha1.Release{}, FieldIndexPortalRef, func(obj client.Object) []string {
			r := obj.(*sreportalv1alpha1.Release)
			return []string{r.Spec.PortalRef}
		}).
		Build()

	reqs := releaseReconcileRequestsForPortal(ctx, c, portal)
	require.Len(t, reqs, 1)
	assert.Equal(t, types.NamespacedName{Namespace: "default", Name: "release-2026-03-21"}, reqs[0].NamespacedName)
}

func TestReleaseReconcileRequestsForPortal_ReleasesDisabled_ReturnsNil(t *testing.T) {
	ctx := context.Background()
	s := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(s))
	portal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:    "Main",
			Features: &sreportalv1alpha1.PortalFeatures{Releases: new(false)},
		},
	}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(portal).Build()
	assert.Nil(t, releaseReconcileRequestsForPortal(ctx, c, portal))
}
