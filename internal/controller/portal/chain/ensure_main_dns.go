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
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/config"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
	sourcepkg "github.com/golgoth31/sreportal/internal/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// annotationSourcesMigrated marks a DNS CR whose source configuration has been
// seeded by EnsureMainDNSHandler. Once present, the handler never touches the
// CR's sources again — the CR is the source of truth and the legacy ConfigMap
// can be decommissioned.
const (
	annotationSourcesMigrated = "sreportal.io/sources-migrated"
	// sourcesMigratedValue is the marker annotation's set value.
	sourcesMigratedValue = "true"
)

// EnsureMainDNSHandler ensures the main (local) portal owns a DNS CR carrying
// the discovery source configuration. The desired configuration is resolved
// once at construction: the legacy ConfigMap sources if the operator loaded
// them, otherwise the built-in defaults.
//
// Behaviour (main local portal, DNS feature enabled), keyed on the
// sources-migrated marker:
//   - no DNS CR for the portal        -> create one, seeded + marked
//   - DNS CR exists, no marker, empty -> backfill sources, mark (migration)
//   - DNS CR exists, no marker, set   -> just mark (user configured pre-upgrade)
//   - DNS CR exists, marker present   -> no-op (CR is the source of truth)
type EnsureMainDNSHandler struct {
	client         client.Client
	scheme         *runtime.Scheme
	sources        sreportalv1alpha2.SourcesSpec
	groupMapping   sreportalv1alpha2.GroupMappingSpec
	reconciliation sreportalv1alpha2.ReconciliationSpec
}

// NewEnsureMainDNSHandler resolves the desired DNS source configuration from the
// (optional) legacy operator config and returns a handler that applies it to the
// main portal's DNS CR.
func NewEnsureMainDNSHandler(c client.Client, scheme *runtime.Scheme, cfg *config.OperatorConfig) *EnsureMainDNSHandler {
	sources, groupMapping, reconciliation, droppedPriority := resolveDesiredDNSConfig(cfg)
	if len(droppedPriority) > 0 {
		// Surface stale/typo'd legacy priority entries (sources not enabled),
		// which are silently filtered out to satisfy the DNS webhook. Logged
		// once at startup since the config is resolved at construction time.
		ctrl.Log.WithName("ensure-main-dns").Info(
			"dropped legacy spec.sources.priority entries for sources that are not enabled",
			"dropped", droppedPriority)
	}
	return &EnsureMainDNSHandler{
		client:         c,
		scheme:         scheme,
		sources:        sources,
		groupMapping:   groupMapping,
		reconciliation: reconciliation,
	}
}

// Handle implements reconciler.Handler.
func (h *EnsureMainDNSHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.Portal, ChainData]) error {
	portal := rc.Resource

	// Only the main, local portal owns the discovery DNS CR. Remote DNS CRs are
	// managed by SyncRemoteDNSHandler and must not be touched here.
	if portal.Spec.Remote != nil || !portal.Spec.Main || !portal.Spec.Features.IsDNSEnabled() {
		return nil
	}

	logger := log.FromContext(ctx).WithName("ensure-main-dns")

	existing, err := h.findLocalDNS(ctx, portal)
	if err != nil {
		return fmt.Errorf("find local DNS for portal %q: %w", portal.Name, err)
	}

	if existing == nil {
		return h.createMainDNS(ctx, portal)
	}

	// Already migrated: the CR owns its configuration, leave it alone.
	if existing.Annotations[annotationSourcesMigrated] == sourcesMigratedValue {
		return nil
	}

	base := existing.DeepCopy()
	// Backfill only when sources were never configured (e.g. a CR freshly
	// converted from v1alpha1, whose sources are zero). Never clobber a config
	// the user set before the upgrade.
	if sourcesEmpty(existing.Spec.Sources) {
		existing.Spec.Sources = h.sources
		existing.Spec.GroupMapping = h.groupMapping
		existing.Spec.Reconciliation = h.reconciliation
		logger.Info("backfilled DNS sources from legacy config/defaults", "name", existing.Name)
	}
	if existing.Annotations == nil {
		existing.Annotations = map[string]string{}
	}
	existing.Annotations[annotationSourcesMigrated] = sourcesMigratedValue
	if err := h.client.Patch(ctx, existing, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("mark/backfill DNS %q: %w", existing.Name, err)
	}
	return nil
}

// findLocalDNS returns the local (non-remote) DNS CR for the portal, chosen
// deterministically by lowest name when several exist, or nil if none.
func (h *EnsureMainDNSHandler) findLocalDNS(ctx context.Context, portal *sreportalv1alpha1.Portal) (*sreportalv1alpha2.DNS, error) {
	var list sreportalv1alpha2.DNSList
	if err := h.client.List(ctx, &list,
		client.InNamespace(portal.Namespace),
		client.MatchingFields{portalfeatures.FieldIndexPortalRef: portal.Name},
	); err != nil {
		return nil, err
	}
	var picked *sreportalv1alpha2.DNS
	for i := range list.Items {
		if list.Items[i].Spec.IsRemote {
			continue
		}
		if picked == nil || list.Items[i].Name < picked.Name {
			picked = &list.Items[i]
		}
	}
	return picked, nil
}

// createMainDNS creates the main portal's DNS CR, seeded and marked, owned by
// the portal for cascade deletion.
func (h *EnsureMainDNSHandler) createMainDNS(ctx context.Context, portal *sreportalv1alpha1.Portal) error {
	logger := log.FromContext(ctx).WithName("ensure-main-dns")
	dns := &sreportalv1alpha2.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:        portal.Name,
			Namespace:   portal.Namespace,
			Annotations: map[string]string{annotationSourcesMigrated: sourcesMigratedValue},
		},
		Spec: sreportalv1alpha2.DNSSpec{
			PortalRef:      portal.Name,
			Sources:        h.sources,
			GroupMapping:   h.groupMapping,
			Reconciliation: h.reconciliation,
		},
	}
	if err := controllerutil.SetControllerReference(portal, dns, h.scheme); err != nil {
		return fmt.Errorf("set controller reference on DNS %q: %w", dns.Name, err)
	}
	if err := h.client.Create(ctx, dns); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Race with another reconcile/cache; the next pass handles it.
			return nil
		}
		return fmt.Errorf("create DNS %q: %w", dns.Name, err)
	}
	logger.Info("created main portal DNS CR", "name", dns.Name)
	return nil
}

// sourcesEmpty reports whether no discovery source is configured (all source
// pointers nil and no priority order set).
func sourcesEmpty(s sreportalv1alpha2.SourcesSpec) bool {
	return s.Service == nil &&
		s.Ingress == nil &&
		s.DNSEndpoint == nil &&
		s.IstioGateway == nil &&
		s.IstioVirtualService == nil &&
		s.GatewayHTTPRoute == nil &&
		s.GatewayGRPCRoute == nil &&
		s.GatewayTLSRoute == nil &&
		s.GatewayTCPRoute == nil &&
		s.GatewayUDPRoute == nil &&
		s.CrossplaneScalewayRecord == nil &&
		len(s.Priority) == 0
}

// resolveDesiredDNSConfig returns the DNS configuration to seed: the legacy
// ConfigMap config when the operator loaded sources from it, otherwise the
// built-in defaults. "Loaded sources" is detected by at least one source being
// present (any non-nil source or a priority list) — a config without a sources
// section, or no ConfigMap at all, falls back to defaults.
func resolveDesiredDNSConfig(cfg *config.OperatorConfig) (
	sreportalv1alpha2.SourcesSpec,
	sreportalv1alpha2.GroupMappingSpec,
	sreportalv1alpha2.ReconciliationSpec,
	[]string, // priority entries dropped because their source is not enabled
) {
	if cfg != nil && hasLegacySources(&cfg.Sources) {
		sources, dropped := mapLegacySources(&cfg.Sources)
		return sources,
			mapLegacyGroupMapping(&cfg.GroupMapping),
			mapLegacyReconciliation(&cfg.Reconciliation),
			dropped
	}
	return sreportalv1alpha2.DefaultSourcesSpec(),
		sreportalv1alpha2.DefaultGroupMappingSpec(),
		sreportalv1alpha2.DefaultReconciliationSpec(),
		nil
}

// hasLegacySources reports whether the legacy config explicitly carried any
// source configuration.
func hasLegacySources(s *config.SourcesConfig) bool {
	return s.Service != nil ||
		s.Ingress != nil ||
		s.DNSEndpoint != nil ||
		s.IstioGateway != nil ||
		s.IstioVirtualService != nil ||
		s.GatewayHTTPRoute != nil ||
		s.GatewayGRPCRoute != nil ||
		s.GatewayTLSRoute != nil ||
		s.GatewayTCPRoute != nil ||
		s.GatewayUDPRoute != nil ||
		s.CrossplaneScalewayRecord != nil ||
		len(s.Priority) > 0
}

// mapLegacySources translates the legacy ConfigMap source configuration into the
// v1alpha2 SourcesSpec. A few advanced external-dns knobs present in the legacy
// schema have no v1alpha2 field (e.g. resolveLoadBalancerHostname,
// ignoreIngressTlsSpec) and are intentionally dropped.
// It also returns the priority entries dropped because their source is not
// enabled, so the caller can surface them (they usually signal a stale legacy
// config or a typo).
func mapLegacySources(s *config.SourcesConfig) (sreportalv1alpha2.SourcesSpec, []string) {
	out := sreportalv1alpha2.SourcesSpec{}
	if c := s.Service; c != nil {
		out.Service = &sreportalv1alpha2.ServiceSourceSpec{
			CommonSourceSpec:  common(c.Enabled, c.Namespace, c.AnnotationFilter, c.LabelFilter, c.FQDNTemplate, c.CombineFQDNAndAnnotation, c.IgnoreHostnameAnnotation),
			PublishInternal:   c.PublishInternal,
			PublishHostIP:     c.PublishHostIP,
			ServiceTypeFilter: c.ServiceTypeFilter,
		}
	}
	if c := s.Ingress; c != nil {
		out.Ingress = &sreportalv1alpha2.IngressSourceSpec{
			CommonSourceSpec:  common(c.Enabled, c.Namespace, c.AnnotationFilter, c.LabelFilter, c.FQDNTemplate, c.CombineFQDNAndAnnotation, c.IgnoreHostnameAnnotation),
			IngressClassNames: c.IngressClassNames,
		}
	}
	if c := s.DNSEndpoint; c != nil {
		out.DNSEndpoint = &sreportalv1alpha2.DNSEndpointSourceSpec{
			Enabled:   c.Enabled,
			Namespace: c.Namespace,
		}
	}
	if c := s.IstioGateway; c != nil {
		out.IstioGateway = &sreportalv1alpha2.IstioGatewaySourceSpec{
			CommonSourceSpec: common(c.Enabled, c.Namespace, c.AnnotationFilter, "", c.FQDNTemplate, c.CombineFQDNAndAnnotation, c.IgnoreHostnameAnnotation),
		}
	}
	if c := s.IstioVirtualService; c != nil {
		out.IstioVirtualService = &sreportalv1alpha2.IstioVirtualServiceSourceSpec{
			CommonSourceSpec: common(c.Enabled, c.Namespace, c.AnnotationFilter, "", c.FQDNTemplate, c.CombineFQDNAndAnnotation, c.IgnoreHostnameAnnotation),
		}
	}
	out.GatewayHTTPRoute = mapGatewayRoute(s.GatewayHTTPRoute)
	out.GatewayGRPCRoute = mapGatewayRoute(s.GatewayGRPCRoute)
	out.GatewayTLSRoute = mapGatewayRoute(s.GatewayTLSRoute)
	out.GatewayTCPRoute = mapGatewayRoute(s.GatewayTCPRoute)
	out.GatewayUDPRoute = mapGatewayRoute(s.GatewayUDPRoute)
	if c := s.CrossplaneScalewayRecord; c != nil {
		out.CrossplaneScalewayRecord = &sreportalv1alpha2.CrossplaneScalewayRecordSourceSpec{
			Enabled:       c.Enabled,
			Namespace:     c.Namespace,
			LabelFilter:   c.LabelFilter,
			ClusterScoped: c.ClusterScoped,
		}
	}
	var dropped []string
	if len(s.Priority) > 0 {
		// Keep only priority entries whose source is actually enabled: legacy
		// configs often list a full order including disabled sources, and the
		// v1alpha2 DNS webhook rejects priority entries for disabled sources.
		// EnabledKindsFromSpec uses the same criteria as that webhook check.
		enabled := sourcepkg.EnabledKindsFromSpec(&out)
		out.Priority = make([]sreportalv1alpha2.SourceType, 0, len(s.Priority))
		for _, p := range s.Priority {
			if enabled[registry.SourceType(p)] {
				out.Priority = append(out.Priority, sreportalv1alpha2.SourceType(p))
			} else {
				dropped = append(dropped, p)
			}
		}
	}
	return out, dropped
}

func mapGatewayRoute(c *config.GatewayRouteConfig) *sreportalv1alpha2.GatewayRouteSourceSpec {
	if c == nil {
		return nil
	}
	return &sreportalv1alpha2.GatewayRouteSourceSpec{
		CommonSourceSpec:   common(c.Enabled, c.Namespace, c.AnnotationFilter, c.LabelFilter, c.FQDNTemplate, c.CombineFQDNAndAnnotation, c.IgnoreHostnameAnnotation),
		GatewayName:        c.GatewayName,
		GatewayNamespace:   c.GatewayNamespace,
		GatewayLabelFilter: c.GatewayLabelFilter,
	}
}

func common(enabled bool, ns, annot, label, fqdnTmpl string, combine, ignoreHostname bool) sreportalv1alpha2.CommonSourceSpec {
	return sreportalv1alpha2.CommonSourceSpec{
		Enabled:                  enabled,
		Namespace:                ns,
		AnnotationFilter:         annot,
		LabelFilter:              label,
		FQDNTemplate:             fqdnTmpl,
		CombineFQDNAndAnnotation: combine,
		IgnoreHostnameAnnotation: ignoreHostname,
	}
}

func mapLegacyGroupMapping(g *config.GroupMappingConfig) sreportalv1alpha2.GroupMappingSpec {
	defaultGroup := g.DefaultGroup
	if defaultGroup == "" {
		defaultGroup = "Services" // CRD requires a non-empty default group
	}
	return sreportalv1alpha2.GroupMappingSpec{
		DefaultGroup: defaultGroup,
		LabelKey:     g.LabelKey,
		ByNamespace:  g.ByNamespace,
	}
}

func mapLegacyReconciliation(r *config.ReconciliationConfig) sreportalv1alpha2.ReconciliationSpec {
	interval := r.Interval.Duration()
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	retry := r.RetryOnError.Duration()
	if retry <= 0 {
		retry = 30 * time.Second
	}
	return sreportalv1alpha2.ReconciliationSpec{
		Interval:        metav1.Duration{Duration: interval},
		RetryOnError:    metav1.Duration{Duration: retry},
		DisableDNSCheck: r.DisableDNSCheck,
	}
}
