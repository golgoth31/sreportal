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

package source

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
)

func newPortal(name string, main bool, remote *sreportalv1alpha1.RemotePortalSpec, features *sreportalv1alpha1.PortalFeatures) *sreportalv1alpha1.Portal {
	return &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
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
	mainPortal := newPortal("main", true, nil, nil)
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{"main": mainPortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal},
	}
	ep := &endpoint.Endpoint{DNSName: "api.example.com"}

	name, target := resolveEndpointPortal(ctxWithLogger(), ep, idx)

	assert.Equal(t, "main", name)
	require.NotNil(t, target)
	assert.Equal(t, "main", target.Name)
}

func TestResolveEndpointPortal_NoAnnotation_NoMain(t *testing.T) {
	otherPortal := newPortal("team-a", false, nil, nil)
	idx := &PortalIndex{
		Main:   nil,
		ByName: map[string]*sreportalv1alpha1.Portal{"team-a": otherPortal},
		Local:  []*sreportalv1alpha1.Portal{otherPortal},
	}
	ep := &endpoint.Endpoint{DNSName: "api.example.com"}

	name, target := resolveEndpointPortal(ctxWithLogger(), ep, idx)

	assert.Empty(t, name)
	assert.Nil(t, target)
}

func TestResolveEndpointPortal_AnnotatedPortalNotFound(t *testing.T) {
	mainPortal := newPortal("main", true, nil, nil)
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{"main": mainPortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal},
	}
	ep := &endpoint.Endpoint{
		DNSName: "api.example.com",
		Labels:  map[string]string{adapter.PortalAnnotationKey: "nonexistent"},
	}

	name, target := resolveEndpointPortal(ctxWithLogger(), ep, idx)

	assert.Empty(t, name)
	assert.Nil(t, target)
}

func TestResolveEndpointPortal_AnnotatedPortalRemote(t *testing.T) {
	mainPortal := newPortal("main", true, nil, nil)
	remotePortal := newPortal("remote-portal", false, &sreportalv1alpha1.RemotePortalSpec{URL: "https://remote.example.com"}, nil)
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{"main": mainPortal, "remote-portal": remotePortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal},
	}
	ep := &endpoint.Endpoint{
		DNSName: "api.example.com",
		Labels:  map[string]string{adapter.PortalAnnotationKey: "remote-portal"},
	}

	name, target := resolveEndpointPortal(ctxWithLogger(), ep, idx)

	assert.Empty(t, name)
	assert.Nil(t, target)
}

func TestResolveEndpointPortal_AnnotatedPortalDNSDisabled(t *testing.T) {
	mainPortal := newPortal("main", true, nil, nil)
	noDNSPortal := newPortal("no-dns", false, nil, &sreportalv1alpha1.PortalFeatures{DNS: new(bool)})
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{"main": mainPortal, "no-dns": noDNSPortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal, noDNSPortal},
	}
	ep := &endpoint.Endpoint{
		DNSName: "api.example.com",
		Labels:  map[string]string{adapter.PortalAnnotationKey: "no-dns"},
	}

	name, target := resolveEndpointPortal(ctxWithLogger(), ep, idx)

	assert.Empty(t, name)
	assert.Nil(t, target)
}

func TestResolveEndpointPortal_AnnotatedPortalValid(t *testing.T) {
	mainPortal := newPortal("main", true, nil, nil)
	teamPortal := newPortal("team-a", false, nil, nil)
	idx := &PortalIndex{
		Main:   mainPortal,
		ByName: map[string]*sreportalv1alpha1.Portal{"main": mainPortal, "team-a": teamPortal},
		Local:  []*sreportalv1alpha1.Portal{mainPortal, teamPortal},
	}
	ep := &endpoint.Endpoint{
		DNSName: "api.example.com",
		Labels:  map[string]string{adapter.PortalAnnotationKey: "team-a"},
	}

	name, target := resolveEndpointPortal(ctxWithLogger(), ep, idx)

	assert.Equal(t, "team-a", name)
	require.NotNil(t, target)
	assert.Equal(t, "team-a", target.Name)
}
