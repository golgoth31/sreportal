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

package chain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(s))
	return s
}

func TestBuildPortalIndex_NoMainPortal_IndexMainIsNil(t *testing.T) {
	// Two local portals, neither has spec.main=true
	portalA := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a", Namespace: tNsDefault},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Team A"},
	}
	portalB := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "team-b", Namespace: tNsDefault},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Team B"},
	}

	scheme := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portalA, portalB).Build()
	handler := NewBuildPortalIndexHandler(c)

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)

	// Index should be populated but Main must be nil (no fallback)
	require.NotNil(t, rc.Data.Index, "index should be set when local portals exist")
	assert.Nil(t, rc.Data.Index.Main, "main must be nil when no portal has spec.main=true")
	assert.Len(t, rc.Data.Index.Local, 2)
	assert.Contains(t, rc.Data.Index.ByName, "team-a")
	assert.Contains(t, rc.Data.Index.ByName, "team-b")
}

func TestBuildPortalIndex_MainPortalExists_IndexMainIsSet(t *testing.T) {
	mainPortal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain, Namespace: tNsDefault},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Main Portal", Main: true},
	}
	otherPortal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a", Namespace: tNsDefault},
		Spec:       sreportalv1alpha1.PortalSpec{Title: "Team A"},
	}

	scheme := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mainPortal, otherPortal).Build()
	handler := NewBuildPortalIndexHandler(c)

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)

	require.NotNil(t, rc.Data.Index)
	require.NotNil(t, rc.Data.Index.Main)
	assert.Equal(t, tPortalMain, rc.Data.Index.Main.Name)
	assert.Len(t, rc.Data.Index.Local, 2)
}

func TestBuildPortalIndex_NoPortals_IndexNil(t *testing.T) {
	scheme := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	handler := NewBuildPortalIndexHandler(c)

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)
	assert.Nil(t, rc.Data.Index, "index should be nil when no portals exist")
}

func TestBuildPortalIndex_OnlyRemotePortals_IndexNilAndReturnsEarly(t *testing.T) {
	remotePortal := &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: "remote", Namespace: tNsDefault},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:  "Remote",
			Remote: &sreportalv1alpha1.RemotePortalSpec{URL: "https://remote.example.com"},
		},
	}

	scheme := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(remotePortal).Build()
	handler := NewBuildPortalIndexHandler(c)

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)
	// Remote-only → no local portals → returns early, index nil
	assert.Nil(t, rc.Data.Index)
}
