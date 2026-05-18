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

package components

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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	readstoresource "github.com/golgoth31/sreportal/internal/readstore/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
	svcsrc "github.com/golgoth31/sreportal/internal/source/service"
	"github.com/golgoth31/sreportal/internal/statuspage"
)

const (
	tNsDefault  = "default"
	tPortalMain = "main"
	tPortalTeam = "team-a"
	tCompName   = "API Gateway"
	tCompGroup  = "Infrastructure"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(s))
	return s
}

func newTestPortal(name string, main bool, remote *sreportalv1alpha1.RemotePortalSpec, features *sreportalv1alpha1.PortalFeatures) *sreportalv1alpha1.Portal {
	return &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: tNsDefault},
		Spec: sreportalv1alpha1.PortalSpec{
			Title:    name,
			Main:     main,
			Remote:   remote,
			Features: features,
		},
	}
}

func ctxWithLogger() context.Context {
	return log.IntoContext(context.Background(), log.Log)
}

func newEnrichedEndpoint(kind registry.SourceType, ns, name string, annotations map[string]string) domainsource.EnrichedEndpoint {
	return domainsource.EnrichedEndpoint{
		Endpoint:          &endpoint.Endpoint{DNSName: name + ".example.com"},
		Kind:              kind,
		Namespace:         ns,
		Name:              name,
		SourceAnnotations: annotations,
	}
}

func TestReconciler_CreatesComponentFromAnnotations(t *testing.T) {
	scheme := newTestScheme(t)
	portal := newTestPortal(tPortalMain, true, nil, nil)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()

	store := readstoresource.NewStore()
	reg := registry.NewRegistry(svcsrc.NewResolver())

	store.ReplaceKind(svcsrc.SourceTypeService, []domainsource.EnrichedEndpoint{
		newEnrichedEndpoint(svcsrc.SourceTypeService, tNsDefault, "my-svc", map[string]string{
			adapter.ComponentAnnotationKey:            tCompName,
			adapter.ComponentGroupAnnotationKey:       tCompGroup,
			adapter.ComponentDescriptionAnnotationKey: "Main ingress",
			adapter.ComponentLinkAnnotationKey:        "https://grafana.internal",
			adapter.PortalAnnotationKey:               tPortalMain,
		}),
	})

	r := &Reconciler{
		Client:   c,
		Scheme:   scheme,
		Reader:   store,
		Registry: reg,
		Interval: time.Minute,
	}

	r.cycle(ctxWithLogger())

	var comp sreportalv1alpha1.Component
	expectedName := statuspage.GenerateCRName(tPortalMain, tCompName)
	err := c.Get(context.Background(), types.NamespacedName{Name: expectedName, Namespace: tNsDefault}, &comp)
	require.NoError(t, err)

	assert.Equal(t, tCompName, comp.Spec.DisplayName)
	assert.Equal(t, tCompGroup, comp.Spec.Group)
	assert.Equal(t, "Main ingress", comp.Spec.Description)
	assert.Equal(t, "https://grafana.internal", comp.Spec.Link)
	assert.Equal(t, tPortalMain, comp.Spec.PortalRef)
	assert.Equal(t, sreportalv1alpha1.ComponentStatusOperational, comp.Spec.Status)
	assert.Equal(t, adapter.ManagedBySourceController, comp.Labels[adapter.ManagedByLabelKey])
	assert.Equal(t, tPortalMain, comp.Labels[adapter.PortalAnnotationKey])

	require.Len(t, comp.OwnerReferences, 1, "component should have owner reference to Portal")
	assert.Equal(t, "Portal", comp.OwnerReferences[0].Kind)
	assert.Equal(t, tPortalMain, comp.OwnerReferences[0].Name)
	require.NotNil(t, comp.OwnerReferences[0].Controller)
	assert.True(t, *comp.OwnerReferences[0].Controller)
}

func TestReconciler_UpdatesComponentMetadataButNotStatus(t *testing.T) {
	scheme := newTestScheme(t)
	portal := newTestPortal(tPortalMain, true, nil, nil)

	compName := statuspage.GenerateCRName(tPortalMain, tCompName)
	existing := &sreportalv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      compName,
			Namespace: tNsDefault,
			Labels: map[string]string{
				adapter.ManagedByLabelKey:   adapter.ManagedBySourceController,
				adapter.PortalAnnotationKey: tPortalMain,
			},
		},
		Spec: sreportalv1alpha1.ComponentSpec{
			DisplayName: tCompName,
			Group:       "Old Group",
			Description: "Old desc",
			Link:        "https://old.link",
			PortalRef:   tPortalMain,
			Status:      sreportalv1alpha1.ComponentStatusDegraded,
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, existing).Build()

	store := readstoresource.NewStore()
	reg := registry.NewRegistry(svcsrc.NewResolver())

	store.ReplaceKind(svcsrc.SourceTypeService, []domainsource.EnrichedEndpoint{
		newEnrichedEndpoint(svcsrc.SourceTypeService, tNsDefault, "my-svc", map[string]string{
			adapter.ComponentAnnotationKey:            tCompName,
			adapter.ComponentGroupAnnotationKey:       "New Group",
			adapter.ComponentDescriptionAnnotationKey: "New desc",
			adapter.ComponentLinkAnnotationKey:        "https://new.link",
			adapter.PortalAnnotationKey:               tPortalMain,
		}),
	})

	r := &Reconciler{
		Client:   c,
		Scheme:   scheme,
		Reader:   store,
		Registry: reg,
		Interval: time.Minute,
	}

	r.cycle(ctxWithLogger())

	var comp sreportalv1alpha1.Component
	err := c.Get(context.Background(), types.NamespacedName{Name: compName, Namespace: tNsDefault}, &comp)
	require.NoError(t, err)

	assert.Equal(t, "New Group", comp.Spec.Group)
	assert.Equal(t, "New desc", comp.Spec.Description)
	assert.Equal(t, "https://new.link", comp.Spec.Link)
	// Status must NOT be overwritten — degraded must persist.
	assert.Equal(t, sreportalv1alpha1.ComponentStatusDegraded, comp.Spec.Status)
}

func TestReconciler_DeletesOrphanedComponent(t *testing.T) {
	scheme := newTestScheme(t)
	portal := newTestPortal(tPortalMain, true, nil, nil)

	orphanName := statuspage.GenerateCRName(tPortalMain, "Removed Service")
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
			Group:       tCompGroup,
			PortalRef:   tPortalMain,
			Status:      sreportalv1alpha1.ComponentStatusOperational,
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal, orphan).Build()

	store := readstoresource.NewStore()
	reg := registry.NewRegistry(svcsrc.NewResolver())
	// No entries in the store — orphan should be deleted.
	store.ReplaceKind(svcsrc.SourceTypeService, nil)

	r := &Reconciler{
		Client:   c,
		Scheme:   scheme,
		Reader:   store,
		Registry: reg,
		Interval: time.Minute,
	}

	r.cycle(ctxWithLogger())

	var comp sreportalv1alpha1.Component
	err := c.Get(context.Background(), types.NamespacedName{Name: orphanName, Namespace: tNsDefault}, &comp)
	assert.True(t, err != nil, "orphaned auto-managed component should be deleted")
}

func TestReconciler_SkipsWhenStatusPageDisabled(t *testing.T) {
	scheme := newTestScheme(t)
	disabled := false
	portal := newTestPortal(tPortalMain, true, nil, &sreportalv1alpha1.PortalFeatures{StatusPage: &disabled})
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()

	store := readstoresource.NewStore()
	reg := registry.NewRegistry(svcsrc.NewResolver())

	store.ReplaceKind(svcsrc.SourceTypeService, []domainsource.EnrichedEndpoint{
		newEnrichedEndpoint(svcsrc.SourceTypeService, tNsDefault, "my-svc", map[string]string{
			adapter.ComponentAnnotationKey: "API",
			adapter.PortalAnnotationKey:    tPortalMain,
		}),
	})

	r := &Reconciler{
		Client:   c,
		Scheme:   scheme,
		Reader:   store,
		Registry: reg,
		Interval: time.Minute,
	}

	r.cycle(ctxWithLogger())

	var list sreportalv1alpha1.ComponentList
	err := c.List(context.Background(), &list)
	require.NoError(t, err)
	assert.Empty(t, list.Items)
}

func TestReconciler_RoutesUnannotatedToMain(t *testing.T) {
	scheme := newTestScheme(t)
	portal := newTestPortal(tPortalMain, true, nil, nil)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()

	store := readstoresource.NewStore()
	reg := registry.NewRegistry(svcsrc.NewResolver())

	// No portal annotation — should route to main.
	store.ReplaceKind(svcsrc.SourceTypeService, []domainsource.EnrichedEndpoint{
		newEnrichedEndpoint(svcsrc.SourceTypeService, tNsDefault, "my-svc", map[string]string{
			adapter.ComponentAnnotationKey: tCompName,
			// No PortalAnnotationKey — routes to main.
		}),
	})

	r := &Reconciler{
		Client:   c,
		Scheme:   scheme,
		Reader:   store,
		Registry: reg,
		Interval: time.Minute,
	}

	r.cycle(ctxWithLogger())

	var list sreportalv1alpha1.ComponentList
	err := c.List(context.Background(), &list)
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, tPortalMain, list.Items[0].Spec.PortalRef)
	assert.Equal(t, tCompName, list.Items[0].Spec.DisplayName)
}

func TestReconciler_DedupesByPortalAndDisplayName(t *testing.T) {
	scheme := newTestScheme(t)
	portal := newTestPortal(tPortalMain, true, nil, nil)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(portal).Build()

	store := readstoresource.NewStore()
	reg := registry.NewRegistry(svcsrc.NewResolver())

	// Two endpoints with the same component DisplayName + same portal.
	store.ReplaceKind(svcsrc.SourceTypeService, []domainsource.EnrichedEndpoint{
		newEnrichedEndpoint(svcsrc.SourceTypeService, tNsDefault, "svc-1", map[string]string{
			adapter.ComponentAnnotationKey: tCompName,
			adapter.PortalAnnotationKey:    tPortalMain,
		}),
		newEnrichedEndpoint(svcsrc.SourceTypeService, tNsDefault, "svc-2", map[string]string{
			adapter.ComponentAnnotationKey: tCompName,
			adapter.PortalAnnotationKey:    tPortalMain,
		}),
	})

	r := &Reconciler{
		Client:   c,
		Scheme:   scheme,
		Reader:   store,
		Registry: reg,
		Interval: time.Minute,
	}

	r.cycle(ctxWithLogger())

	var list sreportalv1alpha1.ComponentList
	err := c.List(context.Background(), &list)
	require.NoError(t, err)
	assert.Len(t, list.Items, 1, "duplicate (portal, displayName) should produce only one Component CR")
}
