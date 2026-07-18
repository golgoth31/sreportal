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

package chain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/controller/portal/chain"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const sourcesMigratedAnnotation = "sreportal.io/sources-migrated"

func newDNSSchemeAndClient(t *testing.T, objs ...client.Object) (*runtime.Scheme, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, sreportalv1alpha1.AddToScheme(scheme))
	require.NoError(t, sreportalv1alpha2.AddToScheme(scheme))
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithIndex(&sreportalv1alpha2.DNS{}, portalfeatures.FieldIndexPortalRef, func(o client.Object) []string {
			dns := o.(*sreportalv1alpha2.DNS)
			if dns.Spec.PortalRef == "" {
				return nil
			}
			return []string{dns.Spec.PortalRef}
		}).
		Build()
	return scheme, cli
}

func mainPortal() *sreportalv1alpha1.Portal {
	return &sreportalv1alpha1.Portal{
		ObjectMeta: metav1.ObjectMeta{Name: tPortalMain, Namespace: nsDefault, UID: "portal-uid"},
		Spec:       sreportalv1alpha1.PortalSpec{Title: tTitleMain, Main: true},
	}
}

func handle(t *testing.T, h *chain.EnsureMainDNSHandler, portal *sreportalv1alpha1.Portal) {
	t.Helper()
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.Portal, chain.ChainData]{Resource: portal}
	require.NoError(t, h.Handle(context.Background(), rc))
}

// Fresh install: no DNS CR, no legacy config -> created with embedded defaults.
func TestEnsureMainDNS_CreatesWithDefaults(t *testing.T) {
	scheme, cli := newDNSSchemeAndClient(t)
	h := chain.NewEnsureMainDNSHandler(cli, scheme, nil)

	handle(t, h, mainPortal())

	var dns sreportalv1alpha2.DNS
	require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: tPortalMain, Namespace: nsDefault}, &dns))
	require.Equal(t, tPortalMain, dns.Spec.PortalRef)
	require.False(t, dns.Spec.IsRemote)
	require.NotNil(t, dns.Spec.Sources.Service)
	require.True(t, dns.Spec.Sources.Service.Enabled)
	require.NotNil(t, dns.Spec.Sources.Ingress)
	require.NotNil(t, dns.Spec.Sources.GatewayHTTPRoute)
	require.NotNil(t, dns.Spec.Sources.GatewayGRPCRoute)
	require.Nil(t, dns.Spec.Sources.GatewayTLSRoute, "TLS route is not part of the default set")
	require.Equal(t, "true", dns.Annotations[sourcesMigratedAnnotation])
	require.Len(t, dns.OwnerReferences, 1)
	require.Equal(t, tPortalMain, dns.OwnerReferences[0].Name)
}

// Migration: operator loaded legacy sources -> those go in verbatim, not defaults.
func TestEnsureMainDNS_UsesLegacyConfigWhenLoaded(t *testing.T) {
	scheme, cli := newDNSSchemeAndClient(t)
	cfg := &config.OperatorConfig{
		Sources: config.SourcesConfig{
			Service: &config.ServiceConfig{Enabled: true, Namespace: "prod"},
		},
	}
	h := chain.NewEnsureMainDNSHandler(cli, scheme, cfg)

	handle(t, h, mainPortal())

	var dns sreportalv1alpha2.DNS
	require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: tPortalMain, Namespace: nsDefault}, &dns))
	require.NotNil(t, dns.Spec.Sources.Service)
	require.Equal(t, "prod", dns.Spec.Sources.Service.Namespace)
	require.Nil(t, dns.Spec.Sources.Ingress, "legacy config had no ingress; defaults must not leak in")
	require.Nil(t, dns.Spec.Sources.GatewayHTTPRoute)
}

// Legacy configs often list a full priority order including sources that are
// not enabled. The v1alpha2 DNS webhook rejects priority entries for disabled
// sources, so the mapper must drop them — keeping the order of the enabled ones.
func TestEnsureMainDNS_FiltersDisabledSourcesFromPriority(t *testing.T) {
	scheme, cli := newDNSSchemeAndClient(t)
	cfg := &config.OperatorConfig{
		Sources: config.SourcesConfig{
			Service:     &config.ServiceConfig{Enabled: true},
			Ingress:     &config.IngressConfig{Enabled: true},
			DNSEndpoint: &config.DNSEndpointConfig{Enabled: false}, // listed but disabled
			Priority:    []string{"ingress", "dnsendpoint", "service"},
		},
	}
	h := chain.NewEnsureMainDNSHandler(cli, scheme, cfg)

	handle(t, h, mainPortal())

	var dns sreportalv1alpha2.DNS
	require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: tPortalMain, Namespace: nsDefault}, &dns))
	require.Equal(t,
		[]sreportalv1alpha2.SourceType{sreportalv1alpha2.SourceTypeIngress, sreportalv1alpha2.SourceTypeService},
		dns.Spec.Sources.Priority,
		"disabled dnsendpoint must be filtered from priority, enabled order preserved")
}

// When every priority entry references a disabled or unknown source, the
// resulting priority is empty (still webhook-valid) — the DNS is created, not
// rejected. Guards the boundary where filtering goes from "trim" to "empty".
func TestEnsureMainDNS_AllPriorityEntriesDropped(t *testing.T) {
	scheme, cli := newDNSSchemeAndClient(t)
	cfg := &config.OperatorConfig{
		Sources: config.SourcesConfig{
			Service:  &config.ServiceConfig{Enabled: true}, // enabled, but not in priority
			Priority: []string{"dnsendpoint", "ingres"},    // disabled + typo → all dropped
		},
	}
	h := chain.NewEnsureMainDNSHandler(cli, scheme, cfg)

	handle(t, h, mainPortal())

	var dns sreportalv1alpha2.DNS
	require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: tPortalMain, Namespace: nsDefault}, &dns))
	require.Empty(t, dns.Spec.Sources.Priority, "priority of only-disabled/unknown sources must end empty")
	require.NotNil(t, dns.Spec.Sources.Service, "enabled source still mapped")
}

// Existing cluster: a DNS CR converted from v1alpha1 (empty sources, no marker,
// arbitrary name) gets backfilled and marked.
func TestEnsureMainDNS_BackfillsConvertedDNS(t *testing.T) {
	converted := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-dns", Namespace: nsDefault},
		Spec:       sreportalv1alpha2.DNSSpec{PortalRef: tPortalMain},
	}
	scheme, cli := newDNSSchemeAndClient(t, converted)
	h := chain.NewEnsureMainDNSHandler(cli, scheme, nil)

	handle(t, h, mainPortal())

	var dns sreportalv1alpha2.DNS
	require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: "legacy-dns", Namespace: nsDefault}, &dns))
	require.NotNil(t, dns.Spec.Sources.Service, "empty sources must be backfilled")
	require.Equal(t, "true", dns.Annotations[sourcesMigratedAnnotation])

	// No duplicate "main" DNS was created.
	var list sreportalv1alpha2.DNSList
	require.NoError(t, cli.List(context.Background(), &list))
	require.Len(t, list.Items, 1)
}

// Once marked, the handler never touches sources again (even if empty).
func TestEnsureMainDNS_NoopWhenMarked(t *testing.T) {
	marked := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:        tPortalMain,
			Namespace:   nsDefault,
			Annotations: map[string]string{sourcesMigratedAnnotation: "true"},
		},
		Spec: sreportalv1alpha2.DNSSpec{PortalRef: tPortalMain},
	}
	scheme, cli := newDNSSchemeAndClient(t, marked)
	h := chain.NewEnsureMainDNSHandler(cli, scheme, nil)

	handle(t, h, mainPortal())

	var dns sreportalv1alpha2.DNS
	require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: tPortalMain, Namespace: nsDefault}, &dns))
	require.Nil(t, dns.Spec.Sources.Service, "marked CR with empty sources must stay empty")
}

func TestEnsureMainDNS_SkipsNonMainPortal(t *testing.T) {
	scheme, cli := newDNSSchemeAndClient(t)
	h := chain.NewEnsureMainDNSHandler(cli, scheme, nil)

	portal := mainPortal()
	portal.Spec.Main = false
	handle(t, h, portal)

	var list sreportalv1alpha2.DNSList
	require.NoError(t, cli.List(context.Background(), &list))
	require.Empty(t, list.Items)
}

func TestEnsureMainDNS_SkipsRemotePortal(t *testing.T) {
	scheme, cli := newDNSSchemeAndClient(t)
	h := chain.NewEnsureMainDNSHandler(cli, scheme, nil)

	portal := mainPortal()
	portal.Spec.Remote = &sreportalv1alpha1.RemotePortalSpec{URL: "https://remote", Portal: tPortalMain}
	handle(t, h, portal)

	var list sreportalv1alpha2.DNSList
	require.NoError(t, cli.List(context.Background(), &list))
	require.Empty(t, list.Items)
}
