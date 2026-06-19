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

// Package externaldns discovers DNS endpoints using the native external-dns
// source library (sigs.k8s.io/external-dns/source), as opposed to the
// hand-rolled resolvers. It builds one source per kind from an effective
// config derived (union / most-permissive) from the DNS CRs.
package externaldns

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	externaldnssource "sigs.k8s.io/external-dns/source"
	"sigs.k8s.io/external-dns/source/template"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// Source kinds discovered natively by external-dns in this package. These are
// the canonical SourceType identifiers — consumers (DNS chain, components) key
// on them regardless of how the kind is discovered.
const (
	KindIngress             registry.SourceType = "ingress"
	KindService             registry.SourceType = "service"
	KindIstioGateway        registry.SourceType = "istio-gateway"
	KindIstioVirtualService registry.SourceType = "istio-virtualservice"
	KindGatewayHTTPRoute    registry.SourceType = "gateway-httproute"
	KindGatewayGRPCRoute    registry.SourceType = "gateway-grpcroute"
	KindGatewayTCPRoute     registry.SourceType = "gateway-tcproute"
	KindGatewayTLSRoute     registry.SourceType = "gateway-tlsroute"
	KindGatewayUDPRoute     registry.SourceType = "gateway-udproute"
	KindDNSEndpoint         registry.SourceType = "dnsendpoint"
)

// Handles reports whether the kind is discovered via native external-dns here
// (as opposed to a hand-rolled resolver).
func Handles(kind registry.SourceType) bool {
	switch kind {
	case KindIngress, KindService, KindIstioGateway,
		KindIstioVirtualService,
		KindGatewayHTTPRoute, KindGatewayGRPCRoute, KindGatewayTCPRoute,
		KindGatewayTLSRoute, KindGatewayUDPRoute,
		KindDNSEndpoint:
		return true
	}
	return false
}

// EffectiveConfig accumulates the discovery config for one kind across every
// DNS CR that enables it. Discovery is a SUPER-SET: when DNS CRs disagree the
// most-permissive value wins so we never under-discover; per-DNS narrowing
// (namespace, labelFilter) happens later at read time.
type EffectiveConfig struct {
	clusterWide       bool                // true if any contributor is cluster-wide ("")
	namespaces        map[string]struct{} // distinct non-empty namespaces seen
	annotationFilters map[string]struct{} // distinct non-empty annotation filters seen
	labelFilters      map[string]struct{} // distinct non-empty label filters seen
	fqdnTemplates     map[string]struct{} // distinct non-empty fqdn templates seen
	ingressClasses    map[string]struct{}
	serviceTypes      map[string]struct{}
	combineFQDN       bool // OR (combine fqdn template with annotation)
	publishInternal   bool // OR across contributors
	publishHostIP     bool // OR
	// ignore* are true only when EVERY contributor sets them (most permissive).
	ignoreHostnameSet   bool
	ignoreHostnameAll   bool
	ignoreIngressTLSSet bool
	ignoreIngressTLSAll bool
	ignoreIngressRSet   bool
	ignoreIngressRAll   bool
}

func newEffectiveConfig() *EffectiveConfig {
	return &EffectiveConfig{
		namespaces:        map[string]struct{}{},
		annotationFilters: map[string]struct{}{},
		labelFilters:      map[string]struct{}{},
		fqdnTemplates:     map[string]struct{}{},
		ingressClasses:    map[string]struct{}{},
		serviceTypes:      map[string]struct{}{},
		ignoreHostnameAll: true, ignoreIngressTLSAll: true, ignoreIngressRAll: true,
	}
}

// addCommon folds one DNS CR's CommonSourceSpec into the effective config.
// FQDNTemplate / CombineFQDNAndAnnotation are propagated to Config.TemplateEngine
// in toConfig (external-dns v0.21 drives templating through that engine).
func (c *EffectiveConfig) addCommon(s sreportalv1alpha2.CommonSourceSpec) {
	if strings.TrimSpace(s.Namespace) == "" {
		c.clusterWide = true
	} else {
		c.namespaces[s.Namespace] = struct{}{}
	}
	if s.AnnotationFilter != "" {
		c.annotationFilters[s.AnnotationFilter] = struct{}{}
	}
	if s.LabelFilter != "" {
		c.labelFilters[s.LabelFilter] = struct{}{}
	}
	if s.FQDNTemplate != "" {
		c.fqdnTemplates[s.FQDNTemplate] = struct{}{}
	}
	c.combineFQDN = c.combineFQDN || s.CombineFQDNAndAnnotation
	c.ignoreHostnameSet = true
	c.ignoreHostnameAll = c.ignoreHostnameAll && s.IgnoreHostnameAnnotation
}

// namespace returns the effective namespace: "" (cluster-wide) when contributors
// are cluster-wide or span multiple namespaces (super-set), else the single one.
func (c *EffectiveConfig) namespace() string {
	if c.clusterWide || len(c.namespaces) != 1 {
		return ""
	}
	for ns := range c.namespaces {
		return ns
	}
	return ""
}

// single returns the value when exactly one distinct non-empty value was seen,
// else "" (most permissive: no filter).
func single(m map[string]struct{}) string {
	if len(m) != 1 {
		return ""
	}
	for v := range m {
		return v
	}
	return ""
}

func sortedKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		// Return nil (not an empty non-nil slice): external-dns treats a non-nil
		// IngressClassNames as "class filtering requested" and rejects it as
		// mutually exclusive with an annotation filter — so an empty-but-non-nil
		// slice would spuriously trip that guard when no class is configured.
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// toConfig maps the accumulated config to an external-dns *Config for the kind.
func (c *EffectiveConfig) toConfig(kind registry.SourceType) (*externaldnssource.Config, error) {
	lf := labels.Everything()
	if s := single(c.labelFilters); s != "" {
		sel, err := labels.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("parse labelFilter %q for %s: %w", s, kind, err)
		}
		lf = sel
	}
	cfg := &externaldnssource.Config{
		Namespace:        c.namespace(),
		AnnotationFilter: single(c.annotationFilters),
		LabelFilter:      lf,
	}
	// FQDN templating (when configured) applies to every source kind that
	// supports it; an empty template leaves TemplateEngine nil (no-op).
	if ft := single(c.fqdnTemplates); ft != "" {
		eng, err := template.NewEngine(ft, "", "", c.combineFQDN)
		if err != nil {
			return nil, fmt.Errorf("parse fqdnTemplate %q for %s: %w", ft, kind, err)
		}
		cfg.TemplateEngine = eng
	}
	switch kind {
	case KindService:
		cfg.ServiceTypeFilter = sortedKeys(c.serviceTypes)
		cfg.PublishInternal = c.publishInternal
		cfg.PublishHostIP = c.publishHostIP
		cfg.IgnoreHostnameAnnotation = c.ignoreHostnameSet && c.ignoreHostnameAll
	case KindIngress:
		cfg.IngressClassNames = sortedKeys(c.ingressClasses)
		cfg.IgnoreHostnameAnnotation = c.ignoreHostnameSet && c.ignoreHostnameAll
		cfg.IgnoreIngressTLSSpec = c.ignoreIngressTLSSet && c.ignoreIngressTLSAll
		cfg.IgnoreIngressRulesSpec = c.ignoreIngressRSet && c.ignoreIngressRAll
	case KindIstioGateway, KindIstioVirtualService:
		cfg.IgnoreHostnameAnnotation = c.ignoreHostnameSet && c.ignoreHostnameAll
	case KindDNSEndpoint:
		// external-dns' NewCRDSource (v0.21) hardwires the DNSEndpoint type
		// (externaldns.k8s.io/v1alpha1) via its scheme and consumes only
		// Namespace + LabelFilter from cfg — nothing kind-specific to set here.
	}
	return cfg, nil
}

// hash is a stable fingerprint of the resulting external-dns config; the
// provider rebuilds a source only when its hash changes.
func (c *EffectiveConfig) hash(kind registry.SourceType) string {
	cfg, err := c.toConfig(kind)
	if err != nil {
		return "err:" + err.Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ns=%q;af=%q;lf=%q;it=%v;pi=%t;ph=%t;icn=%v;ihn=%t;itls=%t;irul=%t;ft=%q;cf=%t",
		cfg.Namespace, cfg.AnnotationFilter, cfg.LabelFilter.String(), cfg.ServiceTypeFilter,
		cfg.PublishInternal, cfg.PublishHostIP,
		cfg.IngressClassNames, cfg.IgnoreHostnameAnnotation, cfg.IgnoreIngressTLSSpec, cfg.IgnoreIngressRulesSpec,
		single(c.fqdnTemplates), c.combineFQDN)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:8])
}

// BuildEffectiveConfigs derives, for each externaldns-handled kind enabled by at
// least one (local) DNS CR, the accumulated EffectiveConfig.
func BuildEffectiveConfigs(dnsList []sreportalv1alpha2.DNS) map[registry.SourceType]*EffectiveConfig {
	out := map[registry.SourceType]*EffectiveConfig{}
	get := func(k registry.SourceType) *EffectiveConfig {
		if out[k] == nil {
			out[k] = newEffectiveConfig()
		}
		return out[k]
	}
	for i := range dnsList {
		if dnsList[i].Spec.IsRemote {
			continue
		}
		s := &dnsList[i].Spec.Sources
		if s.Service != nil && s.Service.Enabled {
			c := get(KindService)
			c.addCommon(s.Service.CommonSourceSpec)
			c.publishInternal = c.publishInternal || s.Service.PublishInternal
			c.publishHostIP = c.publishHostIP || s.Service.PublishHostIP
			for _, t := range s.Service.ServiceTypeFilter {
				c.serviceTypes[t] = struct{}{}
			}
		}
		if s.Ingress != nil && s.Ingress.Enabled {
			c := get(KindIngress)
			c.addCommon(s.Ingress.CommonSourceSpec)
			for _, n := range s.Ingress.IngressClassNames {
				c.ingressClasses[n] = struct{}{}
			}
			// IngressSourceSpec exposes no ignore-TLS / ignore-rules fields, so no
			// contributor ever asks to ignore them: leave *Set false → toConfig
			// yields IgnoreIngressTLSSpec=false / IgnoreIngressRulesSpec=false,
			// i.e. discover from spec.rules[].host AND spec.tls[].hosts (the 194-FQDN case).
		}
		if s.IstioGateway != nil && s.IstioGateway.Enabled {
			get(KindIstioGateway).addCommon(s.IstioGateway.CommonSourceSpec)
		}
		if s.IstioVirtualService != nil && s.IstioVirtualService.Enabled {
			get(KindIstioVirtualService).addCommon(s.IstioVirtualService.CommonSourceSpec)
		}
		if s.GatewayHTTPRoute != nil && s.GatewayHTTPRoute.Enabled {
			get(KindGatewayHTTPRoute).addCommon(s.GatewayHTTPRoute.CommonSourceSpec)
		}
		if s.GatewayGRPCRoute != nil && s.GatewayGRPCRoute.Enabled {
			get(KindGatewayGRPCRoute).addCommon(s.GatewayGRPCRoute.CommonSourceSpec)
		}
		if s.GatewayTCPRoute != nil && s.GatewayTCPRoute.Enabled {
			get(KindGatewayTCPRoute).addCommon(s.GatewayTCPRoute.CommonSourceSpec)
		}
		if s.GatewayTLSRoute != nil && s.GatewayTLSRoute.Enabled {
			get(KindGatewayTLSRoute).addCommon(s.GatewayTLSRoute.CommonSourceSpec)
		}
		if s.GatewayUDPRoute != nil && s.GatewayUDPRoute.Enabled {
			get(KindGatewayUDPRoute).addCommon(s.GatewayUDPRoute.CommonSourceSpec)
		}
		if s.DNSEndpoint != nil && s.DNSEndpoint.Enabled {
			// DNSEndpointSpec doesn't embed CommonSourceSpec — synthesise the
			// subset it exposes (no fqdnTemplate / annotationFilter for CRDs).
			get(KindDNSEndpoint).addCommon(sreportalv1alpha2.CommonSourceSpec{
				Enabled:     s.DNSEndpoint.Enabled,
				Namespace:   s.DNSEndpoint.Namespace,
				LabelFilter: s.DNSEndpoint.LabelFilter,
			})
		}
	}
	return out
}
