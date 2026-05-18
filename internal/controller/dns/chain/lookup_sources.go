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

package dns

import (
	"context"
	"errors"
	"sort"

	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/reconciler"
	sourcepkg "github.com/golgoth31/sreportal/internal/source"
	"github.com/golgoth31/sreportal/internal/source/crossplanescalewayrecord"
	"github.com/golgoth31/sreportal/internal/source/dnsendpoint"
	"github.com/golgoth31/sreportal/internal/source/gatewaygrpcroute"
	"github.com/golgoth31/sreportal/internal/source/gatewayhttproute"
	"github.com/golgoth31/sreportal/internal/source/gatewaytcproute"
	"github.com/golgoth31/sreportal/internal/source/gatewaytlsroute"
	"github.com/golgoth31/sreportal/internal/source/gatewayudproute"
	"github.com/golgoth31/sreportal/internal/source/ingress"
	"github.com/golgoth31/sreportal/internal/source/istiogateway"
	"github.com/golgoth31/sreportal/internal/source/istiovirtualservice"
	"github.com/golgoth31/sreportal/internal/source/registry"
	"github.com/golgoth31/sreportal/internal/source/service"
)

// LookupSourcesHandler queries the SourceEndpointStore for each enabled kind
// in the DNS CR, applying the effective (namespace, labelFilter) computed
// from spec.sources.<k> ∪ spec.defaults. The result is stored in
// ChainData.EndpointsByKind keyed by SourceType, and ChainData.PriorityOrder
// carries the iteration order downstream handlers must respect.
type LookupSourcesHandler struct {
	Source domainsource.SourceEndpointReader
}

// ErrNilSourceReader is returned when the handler is invoked without a wired
// SourceEndpointReader. Treated as a hard wiring error: returning nil silently
// would clear all auto FQDNs downstream (desiredKinds becomes empty), which is
// indistinguishable from an intentional empty config.
var ErrNilSourceReader = errors.New("LookupSourcesHandler: Source reader is nil (wiring bug)")

// Handle implements reconciler.Handler.
func (h *LookupSourcesHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha2.DNS, ChainData]) error {
	if h.Source == nil {
		return ErrNilSourceReader
	}

	dns := rc.Resource
	enabled := sourcepkg.EnabledKindsFromSpec(&dns.Spec.Sources)
	rc.Data.EndpointsByKind = make(map[registry.SourceType][]*endpoint.Endpoint, len(enabled))
	rc.Data.PriorityOrder = orderedKinds(dns, enabled)

	for _, kind := range rc.Data.PriorityOrder {
		ns, lbl := effectiveFilter(dns, kind)
		entries, err := h.Source.Lookup(kind, ns, lbl)
		if err != nil {
			return err
		}
		eps := make([]*endpoint.Endpoint, 0, len(entries))
		for _, e := range entries {
			eps = append(eps, e.Endpoint)
		}
		rc.Data.EndpointsByKind[kind] = eps
	}
	return nil
}

// effectiveFilter returns the (namespace, labelFilter) pair to apply for a
// given kind, using the per-kind spec when set and spec.defaults otherwise.
func effectiveFilter(dns *sreportalv1alpha2.DNS, kind registry.SourceType) (string, string) {
	src := perKindCommonSpec(&dns.Spec.Sources, kind)
	ns := firstNonEmpty(src.Namespace, dns.Spec.Defaults.Namespace)
	lbl := firstNonEmpty(src.LabelFilter, dns.Spec.Defaults.LabelFilter)
	return ns, lbl
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// perKindCommonSpec returns the CommonSourceSpec carried by the per-kind
// typed pointer in SourcesSpec. DNSEndpoint and CrossplaneScalewayRecord do
// not embed CommonSourceSpec — synthesise an equivalent view so the lookup
// path stays uniform.
func perKindCommonSpec(s *sreportalv1alpha2.SourcesSpec, kind registry.SourceType) sreportalv1alpha2.CommonSourceSpec {
	switch kind {
	case service.SourceTypeService:
		if s.Service != nil {
			return s.Service.CommonSourceSpec
		}
	case ingress.SourceTypeIngress:
		if s.Ingress != nil {
			return s.Ingress.CommonSourceSpec
		}
	case dnsendpoint.SourceTypeDNSEndpoint:
		if s.DNSEndpoint != nil {
			return sreportalv1alpha2.CommonSourceSpec{
				Enabled:     s.DNSEndpoint.Enabled,
				Namespace:   s.DNSEndpoint.Namespace,
				LabelFilter: s.DNSEndpoint.LabelFilter,
			}
		}
	case istiogateway.SourceTypeIstioGateway:
		if s.IstioGateway != nil {
			return s.IstioGateway.CommonSourceSpec
		}
	case istiovirtualservice.SourceTypeIstioVirtualService:
		if s.IstioVirtualService != nil {
			return s.IstioVirtualService.CommonSourceSpec
		}
	case gatewayhttproute.SourceTypeGatewayHTTPRoute:
		if s.GatewayHTTPRoute != nil {
			return s.GatewayHTTPRoute.CommonSourceSpec
		}
	case gatewaygrpcroute.SourceTypeGatewayGRPCRoute:
		if s.GatewayGRPCRoute != nil {
			return s.GatewayGRPCRoute.CommonSourceSpec
		}
	case gatewaytcproute.SourceTypeGatewayTCPRoute:
		if s.GatewayTCPRoute != nil {
			return s.GatewayTCPRoute.CommonSourceSpec
		}
	case gatewaytlsroute.SourceTypeGatewayTLSRoute:
		if s.GatewayTLSRoute != nil {
			return s.GatewayTLSRoute.CommonSourceSpec
		}
	case gatewayudproute.SourceTypeGatewayUDPRoute:
		if s.GatewayUDPRoute != nil {
			return s.GatewayUDPRoute.CommonSourceSpec
		}
	case crossplanescalewayrecord.SourceTypeCrossplaneScalewayRecord:
		if s.CrossplaneScalewayRecord != nil {
			return sreportalv1alpha2.CommonSourceSpec{
				Enabled:     s.CrossplaneScalewayRecord.Enabled,
				Namespace:   s.CrossplaneScalewayRecord.Namespace,
				LabelFilter: s.CrossplaneScalewayRecord.LabelFilter,
			}
		}
	}
	return sreportalv1alpha2.CommonSourceSpec{}
}

// orderedKinds returns enabled kinds in spec.sources.priority order, with
// any leftover enabled kinds appended in deterministic SourceType order.
func orderedKinds(dns *sreportalv1alpha2.DNS, enabled map[registry.SourceType]bool) []registry.SourceType {
	out := make([]registry.SourceType, 0, len(enabled))
	seen := map[registry.SourceType]bool{}
	for _, k := range dns.Spec.Sources.Priority {
		st := registry.SourceType(k)
		if enabled[st] && !seen[st] {
			out = append(out, st)
			seen[st] = true
		}
	}
	rest := make([]registry.SourceType, 0, len(enabled))
	for k := range enabled {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i] < rest[j] })
	out = append(out, rest...)
	return out
}
