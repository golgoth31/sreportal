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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

func TestReconcileComponentsHandler_CreatesComponent(t *testing.T) {
	scheme := newScheme(t)
	portal := newPortal(tPortalMain, true, nil, nil)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()

	handler := NewReconcileComponentsHandler(c)
	idx := &PortalIndex{
		Main:   portal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: portal},
		Local:  []*sreportalv1alpha1.Portal{portal},
	}

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{
			Index: idx,
			ComponentRequests: []ComponentRequest{
				{
					PortalName:  tPortalMain,
					DisplayName: tCompAPIGW,
					Group:       tCompInfra,
					Description: "Main API ingress",
					Link:        "https://grafana.internal/api",
					Status:      tStatusOp,
				},
			},
		},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)

	// Verify component was created
	var comp sreportalv1alpha1.Component
	key := types.NamespacedName{Namespace: tNsDefault, Name: componentCRName(tPortalMain, tCompAPIGW)}
	err = c.Get(context.Background(), key, &comp)
	require.NoError(t, err)

	assert.Equal(t, tCompAPIGW, comp.Spec.DisplayName)
	assert.Equal(t, tCompInfra, comp.Spec.Group)
	assert.Equal(t, "Main API ingress", comp.Spec.Description)
	assert.Equal(t, "https://grafana.internal/api", comp.Spec.Link)
	assert.Equal(t, tPortalMain, comp.Spec.PortalRef)
	assert.Equal(t, sreportalv1alpha1.ComponentStatusOperational, comp.Spec.Status)
	assert.Equal(t, adapter.ManagedBySourceController, comp.Labels[adapter.ManagedByLabelKey])
	assert.Equal(t, tPortalMain, comp.Labels[adapter.PortalAnnotationKey])
}

func TestReconcileComponentsHandler_DefaultsStatusToOperational(t *testing.T) {
	scheme := newScheme(t)
	portal := newPortal(tPortalMain, true, nil, nil)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()

	handler := NewReconcileComponentsHandler(c)
	idx := &PortalIndex{
		Main:   portal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: portal},
		Local:  []*sreportalv1alpha1.Portal{portal},
	}

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{
			Index: idx,
			ComponentRequests: []ComponentRequest{
				{PortalName: tPortalMain, DisplayName: "DB", Group: "Core", Status: ""},
			},
		},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)

	var comp sreportalv1alpha1.Component
	key := types.NamespacedName{Namespace: tNsDefault, Name: componentCRName(tPortalMain, "DB")}
	err = c.Get(context.Background(), key, &comp)
	require.NoError(t, err)
	assert.Equal(t, sreportalv1alpha1.ComponentStatusOperational, comp.Spec.Status)
}

func TestReconcileComponentsHandler_UpdatesMetadataButNotStatus(t *testing.T) {
	scheme := newScheme(t)
	portal := newPortal(tPortalMain, true, nil, nil)

	name := componentCRName(tPortalMain, tCompAPIGW)
	existing := &sreportalv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: tNsDefault,
			Labels: map[string]string{
				adapter.ManagedByLabelKey:   adapter.ManagedBySourceController,
				adapter.PortalAnnotationKey: tPortalMain,
			},
		},
		Spec: sreportalv1alpha1.ComponentSpec{
			DisplayName: tCompAPIGW,
			Group:       "Old Group",
			Description: "Old desc",
			Link:        "https://old.link",
			PortalRef:   tPortalMain,
			Status:      sreportalv1alpha1.ComponentStatusDegraded, // user changed this manually
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, existing).Build()
	handler := NewReconcileComponentsHandler(c)
	idx := &PortalIndex{
		Main:   portal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: portal},
		Local:  []*sreportalv1alpha1.Portal{portal},
	}

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{
			Index: idx,
			ComponentRequests: []ComponentRequest{
				{
					PortalName:  tPortalMain,
					DisplayName: tCompAPIGW,
					Group:       "New Group",
					Description: "New desc",
					Link:        "https://new.link",
					Status:      tStatusOp,
				},
			},
		},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)

	var comp sreportalv1alpha1.Component
	key := types.NamespacedName{Namespace: tNsDefault, Name: name}
	err = c.Get(context.Background(), key, &comp)
	require.NoError(t, err)

	// Metadata synced from annotation
	assert.Equal(t, "New Group", comp.Spec.Group)
	assert.Equal(t, "New desc", comp.Spec.Description)
	assert.Equal(t, "https://new.link", comp.Spec.Link)
	// Status NOT overwritten — user's manual change preserved
	assert.Equal(t, sreportalv1alpha1.ComponentStatusDegraded, comp.Spec.Status)
}

func TestReconcileComponentsHandler_DeletesOrphanedAutoManaged(t *testing.T) {
	scheme := newScheme(t)
	portal := newPortal(tPortalMain, true, nil, nil)

	orphanName := componentCRName(tPortalMain, "Removed Service")
	orphan := &sreportalv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      orphanName,
			Namespace: tNsDefault,
			Labels: map[string]string{
				adapter.ManagedByLabelKey:   adapter.ManagedBySourceController,
				adapter.PortalAnnotationKey: tPortalMain,
			},
		},
		Spec: sreportalv1alpha1.ComponentSpec{
			DisplayName: "Removed Service",
			Group:       tCompInfra,
			PortalRef:   tPortalMain,
			Status:      sreportalv1alpha1.ComponentStatusOperational,
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, orphan).Build()
	handler := NewReconcileComponentsHandler(c)
	idx := &PortalIndex{
		Main:   portal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: portal},
		Local:  []*sreportalv1alpha1.Portal{portal},
	}

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{
			Index:             idx,
			ComponentRequests: []ComponentRequest{}, // no requests — orphan should be deleted
		},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)

	var comp sreportalv1alpha1.Component
	key := types.NamespacedName{Namespace: tNsDefault, Name: orphanName}
	err = c.Get(context.Background(), key, &comp)
	assert.Error(t, err, "orphaned auto-managed component should be deleted")
}

func TestReconcileComponentsHandler_DoesNotDeleteManuallyCreatedComponents(t *testing.T) {
	scheme := newScheme(t)
	portal := newPortal(tPortalMain, true, nil, nil)

	manualName := componentCRName(tPortalMain, "Manual Component")
	manual := &sreportalv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manualName,
			Namespace: tNsDefault,
			Labels: map[string]string{
				adapter.PortalAnnotationKey: tPortalMain,
				// No managed-by label — manually created
			},
		},
		Spec: sreportalv1alpha1.ComponentSpec{
			DisplayName: "Manual Component",
			Group:       "Apps",
			PortalRef:   tPortalMain,
			Status:      sreportalv1alpha1.ComponentStatusOperational,
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, manual).Build()
	handler := NewReconcileComponentsHandler(c)
	idx := &PortalIndex{
		Main:   portal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: portal},
		Local:  []*sreportalv1alpha1.Portal{portal},
	}

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{
			Index:             idx,
			ComponentRequests: []ComponentRequest{}, // no requests
		},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)

	var comp sreportalv1alpha1.Component
	key := types.NamespacedName{Namespace: tNsDefault, Name: manualName}
	err = c.Get(context.Background(), key, &comp)
	assert.NoError(t, err, "manually created component must NOT be deleted")
}

func TestReconcileComponentsHandler_SkipsWhenStatusPageDisabled(t *testing.T) {
	scheme := newScheme(t)
	disabled := false
	portal := newPortal(tPortalMain, true, nil, &sreportalv1alpha1.PortalFeatures{StatusPage: &disabled})
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()

	handler := NewReconcileComponentsHandler(c)
	idx := &PortalIndex{
		Main:   portal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: portal},
		Local:  []*sreportalv1alpha1.Portal{portal},
	}

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{
			Index: idx,
			ComponentRequests: []ComponentRequest{
				{PortalName: tPortalMain, DisplayName: "API", Group: "Infra"},
			},
		},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)

	// Should NOT have created the component
	var list sreportalv1alpha1.ComponentList
	err = c.List(context.Background(), &list)
	require.NoError(t, err)
	assert.Empty(t, list.Items)
}

func TestReconcileComponentsHandler_EmptyRequests(t *testing.T) {
	scheme := newScheme(t)
	portal := newPortal(tPortalMain, true, nil, nil)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()

	handler := NewReconcileComponentsHandler(c)

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{
			Index: &PortalIndex{
				Main:   portal,
				ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: portal},
				Local:  []*sreportalv1alpha1.Portal{portal},
			},
			ComponentRequests: nil,
		},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)
}
