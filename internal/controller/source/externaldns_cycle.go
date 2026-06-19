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
	"strings"

	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	externaldnsv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/golgoth31/sreportal/internal/adapter"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/source/externaldns"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// collectNative discovers endpoints for a kind via the native external-dns
// source library (full extraction: ingress spec.rules/tls, all service types,
// istio gateway servers), then enriches each endpoint with the source object's
// labels and annotations.
//
// external-dns returns flat []*endpoint.Endpoint with no per-object K8s
// metadata, so we recover provenance from the "resource" label external-dns
// stamps ("<kind>/<namespace>/<name>") and re-fetch the object from the
// controller-runtime cache to obtain SourceLabels (read-side labelFilter) and
// SourceAnnotations (sreportal.io/groups enrichment, OriginRef). A failed
// re-fetch never drops the endpoint — it is kept without group metadata (§6).
//
// ctx must be the long-lived manager context: the Provider's informers live for
// its lifetime.
func collectNative(
	ctx context.Context,
	c client.Client,
	p *externaldns.Provider,
	kind registry.SourceType,
	cfg *externaldns.EffectiveConfig,
) ([]domainsource.EnrichedEndpoint, error) {
	logger := log.FromContext(ctx).WithName("source.cycle.externaldns")

	eps, err := p.Endpoints(ctx, kind, cfg)
	if err != nil {
		return nil, err
	}

	type sourceMeta struct {
		labels map[string]string
		anns   map[string]string
		ok     bool
	}
	metaCache := map[string]sourceMeta{}

	entries := make([]domainsource.EnrichedEndpoint, 0, len(eps))
	for _, ep := range eps {
		ref := ep.Labels[endpoint.ResourceLabelKey]
		ns, name := parseResourceRef(ref)
		if name == "" {
			// external-dns always stamps a well-formed "<kind>/<ns>/<name>" for the
			// kinds we handle; a non-empty-but-unparseable ref signals a library
			// regression worth surfacing (the endpoint is still kept, unbucketed).
			if ref != "" {
				logger.V(1).Info("malformed external-dns resource label; keeping endpoint without metadata",
					"kind", kind, "resource", ref)
				metrics.SourceEnrichmentFailures.WithLabelValues(string(kind), "label").Inc()
			}
		}
		key := ns + "/" + name
		m, seen := metaCache[key]
		if !seen {
			if obj := newNativeObject(kind); obj != nil && name != "" {
				if gerr := c.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, obj); gerr != nil {
					// Keep the endpoint without group metadata rather than drop it;
					// a transient cache miss must never erase a discovered FQDN.
					logger.V(1).Info("source object re-fetch failed; keeping endpoint without group metadata",
						"kind", kind, "namespace", ns, "name", name, "err", gerr.Error())
					metrics.SourceEnrichmentFailures.WithLabelValues(string(kind), "fetch").Inc()
				} else {
					m = sourceMeta{labels: obj.GetLabels(), anns: obj.GetAnnotations(), ok: true}
				}
			}
			metaCache[key] = m
		}

		if m.ok {
			// Fold the allowlisted sreportal annotations (notably sreportal.io/groups)
			// onto the endpoint labels. ep is freshly returned by external-dns and
			// owned here (not yet shared via the store), so mutation is safe.
			adapter.EnrichEndpointLabels(ep, m.anns)
		}

		entries = append(entries, domainsource.EnrichedEndpoint{
			Endpoint:          ep,
			Kind:              kind,
			Namespace:         ns,
			Name:              name,
			SourceLabels:      m.labels,
			SourceAnnotations: m.anns,
		})
	}
	return entries, nil
}

// parseResourceRef splits the external-dns "resource" label
// ("<kind>/<namespace>/<name>") into namespace and name. A malformed or missing
// value yields ("", "") — the endpoint is still kept, just unbucketed by
// namespace (acceptable: the natively-handled kinds are always namespaced and
// external-dns always stamps the label).
func parseResourceRef(ref string) (namespace, name string) {
	parts := strings.SplitN(ref, "/", 3)
	if len(parts) != 3 {
		return "", ""
	}
	return parts[1], parts[2]
}

// newNativeObject returns a fresh empty typed object for re-fetching a
// natively-handled kind's source object from the cache.
func newNativeObject(kind registry.SourceType) client.Object {
	switch kind {
	case externaldns.KindService:
		return &corev1.Service{}
	case externaldns.KindIngress:
		return &networkingv1.Ingress{}
	case externaldns.KindIstioGateway:
		return &istionetworkingv1.Gateway{}
	case externaldns.KindIstioVirtualService:
		return &istionetworkingv1.VirtualService{}
	case externaldns.KindGatewayHTTPRoute:
		return &gwapiv1.HTTPRoute{}
	case externaldns.KindGatewayGRPCRoute:
		return &gwapiv1.GRPCRoute{}
	case externaldns.KindGatewayTCPRoute:
		return &gwapiv1alpha2.TCPRoute{}
	case externaldns.KindGatewayTLSRoute:
		return &gwapiv1alpha2.TLSRoute{}
	case externaldns.KindGatewayUDPRoute:
		return &gwapiv1alpha2.UDPRoute{}
	case externaldns.KindDNSEndpoint:
		return &externaldnsv1alpha1.DNSEndpoint{}
	}
	return nil
}
