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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

func newPortal(name string, main bool, remote *sreportalv1alpha1.RemotePortalSpec, features *sreportalv1alpha1.PortalFeatures) *sreportalv1alpha1.Portal {
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

func TestResolveEndpointPortal_NoAnnotation_MainAvailable(t *testing.T) {
	mainPortal := newPortal(tPortalMain, true, nil, nil)
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: mainPortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal},
	}
	ep := &endpoint.Endpoint{DNSName: tFQDNAPI}

	name, target := resolveEndpointPortal(ctxWithLogger(), ep, idx)

	assert.Equal(t, tPortalMain, name)
	require.NotNil(t, target)
	assert.Equal(t, tPortalMain, target.Name)
}

func TestResolveEndpointPortal_NoAnnotation_NoMain(t *testing.T) {
	otherPortal := newPortal(tTeamA, false, nil, nil)
	idx := &PortalIndex{
		Main:   nil,
		ByName: map[string]*sreportalv1alpha1.Portal{tTeamA: otherPortal},
		Local:  []*sreportalv1alpha1.Portal{otherPortal},
	}
	ep := &endpoint.Endpoint{DNSName: tFQDNAPI}

	name, target := resolveEndpointPortal(ctxWithLogger(), ep, idx)

	assert.Empty(t, name)
	assert.Nil(t, target)
}

func TestResolveEndpointPortal_AnnotatedPortalNotFound(t *testing.T) {
	mainPortal := newPortal(tPortalMain, true, nil, nil)
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: mainPortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal},
	}
	ep := &endpoint.Endpoint{
		DNSName: tFQDNAPI,
		Labels:  map[string]string{adapter.PortalAnnotationKey: "nonexistent"},
	}

	name, target := resolveEndpointPortal(ctxWithLogger(), ep, idx)

	assert.Empty(t, name)
	assert.Nil(t, target)
}

func TestResolveEndpointPortal_AnnotatedPortalRemote(t *testing.T) {
	mainPortal := newPortal(tPortalMain, true, nil, nil)
	remotePortal := newPortal("remote-portal", false, &sreportalv1alpha1.RemotePortalSpec{URL: "https://remote.example.com"}, nil)
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: mainPortal, "remote-portal": remotePortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal},
	}
	ep := &endpoint.Endpoint{
		DNSName: tFQDNAPI,
		Labels:  map[string]string{adapter.PortalAnnotationKey: "remote-portal"},
	}

	name, target := resolveEndpointPortal(ctxWithLogger(), ep, idx)

	assert.Empty(t, name)
	assert.Nil(t, target)
}

func TestResolveEndpointPortal_AnnotatedPortalDNSDisabled(t *testing.T) {
	mainPortal := newPortal(tPortalMain, true, nil, nil)
	noDNSPortal := newPortal("no-dns", false, nil, &sreportalv1alpha1.PortalFeatures{DNS: new(bool)})
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: mainPortal, "no-dns": noDNSPortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal, noDNSPortal},
	}
	ep := &endpoint.Endpoint{
		DNSName: tFQDNAPI,
		Labels:  map[string]string{adapter.PortalAnnotationKey: "no-dns"},
	}

	name, target := resolveEndpointPortal(ctxWithLogger(), ep, idx)

	assert.Empty(t, name)
	assert.Nil(t, target)
}

func TestResolveEndpointPortal_AnnotatedPortalValid(t *testing.T) {
	mainPortal := newPortal(tPortalMain, true, nil, nil)
	teamPortal := newPortal(tTeamA, false, nil, nil)
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: mainPortal, tTeamA: teamPortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal, teamPortal},
	}
	ep := &endpoint.Endpoint{
		DNSName: tFQDNAPI,
		Labels:  map[string]string{adapter.PortalAnnotationKey: tTeamA},
	}

	name, target := resolveEndpointPortal(ctxWithLogger(), ep, idx)

	assert.Equal(t, tTeamA, name)
	require.NotNil(t, target)
	assert.Equal(t, tTeamA, target.Name)
}

// --- Component request collection tests ---

// fakeEnricher is a no-op enricher for tests.
type fakeEnricher struct{}

func (fakeEnricher) EnrichEndpoints(_ context.Context, _ registry.SourceType, _ []*endpoint.Endpoint) {
}

// fakeFailureTracker is a no-op failure tracker for tests.
type fakeFailureTracker struct{}

func (fakeFailureTracker) RecordFailure(_ registry.SourceType) int  { return 0 }
func (fakeFailureTracker) RecordRecovery(_ registry.SourceType) int { return 0 }
func (fakeFailureTracker) MarkDegraded(_ context.Context, _ *sreportalv1alpha1.Portal, _ registry.SourceType, _ error, _ int) {
}

// fakeSource returns pre-configured endpoints.
type fakeSource struct {
	endpoints []*endpoint.Endpoint
}

func (f *fakeSource) Endpoints(_ context.Context) ([]*endpoint.Endpoint, error) {
	return f.endpoints, nil
}

func (f *fakeSource) AddEventHandler(_ context.Context, _ func()) {}

func TestCollectEndpointsHandler_ComponentRequests_Populated(t *testing.T) {
	mainPortal := newPortal(tPortalMain, true, nil, nil)
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: mainPortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal},
	}
	handler := NewCollectEndpointsHandler(fakeEnricher{}, fakeFailureTracker{})

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{
			Index: idx,
			TypedSources: []registry.TypedSource{
				{
					Type: tSrcService,
					Source: &fakeSource{endpoints: []*endpoint.Endpoint{
						{
							DNSName: tFQDNAPI,
							Labels: map[string]string{
								adapter.ComponentAnnotationKey:            tCompAPIGW,
								adapter.ComponentGroupAnnotationKey:       tCompInfra,
								adapter.ComponentDescriptionAnnotationKey: "Main API",
								adapter.ComponentLinkAnnotationKey:        "https://grafana.internal/api",
								adapter.ComponentStatusAnnotationKey:      tStatusOp,
							},
						},
					}},
				},
			},
		},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)

	require.Len(t, rc.Data.ComponentRequests, 1)
	cr := rc.Data.ComponentRequests[0]
	assert.Equal(t, tPortalMain, cr.PortalName)
	assert.Equal(t, tCompAPIGW, cr.DisplayName)
	assert.Equal(t, tCompInfra, cr.Group)
	assert.Equal(t, "Main API", cr.Description)
	assert.Equal(t, "https://grafana.internal/api", cr.Link)
	assert.Equal(t, tStatusOp, cr.Status)
}

func TestCollectEndpointsHandler_ComponentRequests_DeduplicateByPortalAndName(t *testing.T) {
	mainPortal := newPortal(tPortalMain, true, nil, nil)
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: mainPortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal},
	}
	handler := NewCollectEndpointsHandler(fakeEnricher{}, fakeFailureTracker{})

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{
			Index: idx,
			TypedSources: []registry.TypedSource{
				{
					Type: tSrcService,
					Source: &fakeSource{endpoints: []*endpoint.Endpoint{
						{
							DNSName: tFQDNAPI,
							Labels: map[string]string{
								adapter.ComponentAnnotationKey:      tCompAPIGW,
								adapter.ComponentGroupAnnotationKey: "Infra",
							},
						},
						{
							DNSName: "api-v2.example.com",
							Labels: map[string]string{
								adapter.ComponentAnnotationKey:      tCompAPIGW, // same name
								adapter.ComponentGroupAnnotationKey: "Apps",     // different group — first wins
							},
						},
					}},
				},
			},
		},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)

	require.Len(t, rc.Data.ComponentRequests, 1, "duplicate (portal, displayName) should be deduped")
	assert.Equal(t, "Infra", rc.Data.ComponentRequests[0].Group, "first-seen should win")
}

func TestCollectEndpointsHandler_ComponentRequests_NotPopulatedWithoutAnnotation(t *testing.T) {
	mainPortal := newPortal(tPortalMain, true, nil, nil)
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{tPortalMain: mainPortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal},
	}
	handler := NewCollectEndpointsHandler(fakeEnricher{}, fakeFailureTracker{})

	rc := &reconciler.ReconcileContext[struct{}, ChainData]{
		Data: ChainData{
			Index: idx,
			TypedSources: []registry.TypedSource{
				{
					Type: tSrcService,
					Source: &fakeSource{endpoints: []*endpoint.Endpoint{
						{DNSName: tFQDNAPI, Labels: map[string]string{}},
					}},
				},
			},
		},
	}

	err := handler.Handle(ctxWithLogger(), rc)
	require.NoError(t, err)
	assert.Empty(t, rc.Data.ComponentRequests)
}
