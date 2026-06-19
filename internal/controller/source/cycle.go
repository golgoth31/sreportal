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
	"errors"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/adapter"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/metrics"
	sourcepkg "github.com/golgoth31/sreportal/internal/source"
	"github.com/golgoth31/sreportal/internal/source/externaldns"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// Cycle is the global producer loop body, exported for testability.
// Caller (Start) is responsible for the time.Ticker. Cycle is idempotent and
// safe to call sequentially from a single goroutine; concurrent invocations
// against the same store are NOT supported.
//
// When provider is non-nil, kinds it natively handles (ingress, service,
// istio-gateway) are discovered through the external-dns source library — full
// extraction from spec.rules/tls, every service type, gateway servers — instead
// of the hand-rolled resolvers (which stay registered for the remaining kinds).
// Pass nil to use the resolver path for every kind.
func Cycle(
	ctx context.Context,
	c client.Client,
	reg *registry.Registry,
	provider *externaldns.Provider,
	store domainsource.SourceEndpointWriter,
	prev map[registry.SourceType]bool,
) map[registry.SourceType]bool {
	logger := log.FromContext(ctx).WithName("source.cycle")

	dnsList, err := listLocalDNS(ctx, c)
	if err != nil {
		logger.Error(err, "failed to list DNS CRs; skipping cycle")
		return prev
	}
	enabled := enabledKindsFromDNS(dnsList)
	var effCfgs map[registry.SourceType]*externaldns.EffectiveConfig
	if provider != nil {
		effCfgs = externaldns.BuildEffectiveConfigs(dnsList)
	}

	for kind := range enabled {
		// Native external-dns path for the kinds the provider handles.
		if provider != nil && externaldns.Handles(kind) {
			collectNativeInto(ctx, c, provider, store, kind, effCfgs[kind], logger)
			continue
		}

		resolver, ok := reg.Get(kind)
		if !ok {
			logger.Info("no resolver registered", "kind", kind)
			continue
		}
		list := resolver.ObjectList()
		if err := c.List(ctx, list); err != nil {
			// CRD not installed (meta.NoKindMatchError) — surfaced as NotFound
			// here. Treat as benign: stop counting the kind as active, but do
			// not wipe previously cached entries (ReplaceKind/DeleteKind is
			// skipped) so transient API outages don't erase good state.
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				logger.Info("CRD not installed; skipping kind", "kind", kind)
				metrics.SourceKindActive.WithLabelValues(string(kind)).Set(0)
				continue
			}
			logger.Error(err, "list failed; preserving previous state", "kind", kind)
			metrics.SourceErrorsTotal.WithLabelValues(string(kind)).Inc()
			continue
		}
		items, skipped := extractItems(list)
		if skipped > 0 {
			// Should never happen for registered source types; surface it rather
			// than silently shrink discovery (which would also skew the
			// atomic-wipe guard below).
			logger.Error(nil, "skipped list elements that are not client.Object",
				"kind", kind, "skipped", skipped)
		}
		entries := make([]domainsource.EnrichedEndpoint, 0, len(items))
		resolveErrs := 0
		for _, obj := range items {
			eps, rerr := resolver.ResolveObject(ctx, obj)
			if rerr != nil {
				resolveErrs++
				logger.Error(rerr, "resolve failed", "kind", kind, "name", obj.GetName(), "ns", obj.GetNamespace())
				metrics.SourceErrorsTotal.WithLabelValues(string(kind)).Inc()
				continue
			}
			for _, ep := range eps {
				// Most resolvers don't set the external-dns "resource" label
				// themselves; fill it in here from the provenance we already
				// have (kind/namespace/name) so DNSRecordEntry.OriginRef has
				// something to carry downstream. A resolver-set value (e.g.
				// crossplanescalewayrecord, which uses the K8s Kind rather
				// than the registry.SourceType) takes precedence.
				if ep.Labels == nil {
					ep.Labels = endpoint.NewLabels()
				}
				if _, ok := ep.Labels[endpoint.ResourceLabelKey]; !ok {
					ep.Labels[endpoint.ResourceLabelKey] = fmt.Sprintf("%s/%s/%s", kind, obj.GetNamespace(), obj.GetName())
				}
				// Fold the allowlisted sreportal annotations onto the endpoint
				// labels via the shared enrichment helper. On the auto DNS path
				// only sreportal.io/groups is consumed downstream (carried into
				// spec.entries -> status -> UI grouping); the other allowlisted
				// annotations ride along but are inert here. ep is freshly
				// resolved (owned here, not yet shared via the store), so
				// mutating it is safe.
				adapter.EnrichEndpointLabels(ep, obj.GetAnnotations())
				entries = append(entries, domainsource.EnrichedEndpoint{
					Endpoint:          ep,
					Kind:              kind,
					Namespace:         obj.GetNamespace(),
					Name:              obj.GetName(),
					SourceLabels:      obj.GetLabels(),
					SourceAnnotations: obj.GetAnnotations(),
				})
			}
		}
		// Guard against atomic wipe: if we had items but every one of them
		// failed to resolve, an upstream bug (resolver wired to wrong type,
		// transient parse error) could otherwise clear every FQDN for the
		// kind. Preserve the previously cached state instead and rely on
		// metrics/logs to surface the failure.
		if len(items) > 0 && resolveErrs == len(items) {
			logger.Error(nil, "all objects failed to resolve; preserving previous state", "kind", kind, "items", len(items))
			metrics.SourceKindActive.WithLabelValues(string(kind)).Set(1)
			continue
		}
		store.ReplaceKind(kind, entries)
		metrics.SourceEndpointsCollected.WithLabelValues(string(kind)).Set(float64(len(entries)))
		metrics.SourceKindActive.WithLabelValues(string(kind)).Set(1)
	}

	for k := range prev {
		if !enabled[k] {
			store.DeleteKind(k)
			if provider != nil && externaldns.Handles(k) {
				// Stop the native source's long-lived informer so a no-longer-used
				// kind doesn't keep a watch open.
				provider.Forget(k)
			}
			metrics.SourceEndpointsCollected.DeleteLabelValues(string(k))
			metrics.SourceKindActive.WithLabelValues(string(k)).Set(0)
		}
	}
	return enabled
}

// collectNativeInto discovers a kind via the external-dns source library and
// applies it to the store under the producer's safety invariants:
//   - §1 conditional replace: on any collection error (including a not-yet-synced
//     informer or an absent CRD) the previous good state is preserved.
//   - §3 anti-collapse: a fresh empty result never overwrites a non-empty cache;
//     it is refused, logged, and counted (likely a transient discovery failure).
func collectNativeInto(
	ctx context.Context,
	c client.Client,
	provider *externaldns.Provider,
	store domainsource.SourceEndpointWriter,
	kind registry.SourceType,
	cfg *externaldns.EffectiveConfig,
	logger logr.Logger,
) {
	if cfg == nil {
		// Enabled but no effective config derived — a wiring/logic bug (the kind
		// is in `enabled` but BuildEffectiveConfigs produced nothing). Surface it
		// loudly; preserve the previous good state.
		logger.Error(nil, "no effective config for native kind; preserving previous state", "kind", kind)
		metrics.SourceErrorsTotal.WithLabelValues(string(kind)).Inc()
		return
	}
	entries, err := collectNative(ctx, c, provider, kind, cfg)
	if err != nil {
		if errors.Is(err, externaldns.ErrSourceNotReady) {
			// Normal during the initial cache sync — not a failure. Preserve the
			// previous good state and retry next cycle; don't count it as an error.
			logger.Info("source not ready yet (cache syncing); preserving previous state", "kind", kind)
			return
		}
		logger.Error(err, "native source collection failed; preserving previous state", "kind", kind)
		metrics.SourceErrorsTotal.WithLabelValues(string(kind)).Inc()
		return
	}
	if len(entries) == 0 && store.CountKind(kind) > 0 {
		logger.Error(nil, "drop guard: refusing to replace non-empty cache with empty collection; preserving previous state",
			"kind", kind, "prev", store.CountKind(kind))
		metrics.SourceDropGuardTriggered.WithLabelValues(string(kind)).Inc()
		metrics.SourceKindActive.WithLabelValues(string(kind)).Set(1)
		return
	}
	store.ReplaceKind(kind, entries)
	metrics.SourceEndpointsCollected.WithLabelValues(string(kind)).Set(float64(len(entries)))
	metrics.SourceKindActive.WithLabelValues(string(kind)).Set(1)
	metrics.SourceLastSuccessfulSync.WithLabelValues(string(kind)).SetToCurrentTime()
}

// listLocalDNS returns the non-remote DNS CRs that drive cluster-wide discovery.
func listLocalDNS(ctx context.Context, c client.Client) ([]sreportalv1alpha2.DNS, error) {
	var dnsList sreportalv1alpha2.DNSList
	if err := c.List(ctx, &dnsList); err != nil {
		return nil, err
	}
	out := make([]sreportalv1alpha2.DNS, 0, len(dnsList.Items))
	for i := range dnsList.Items {
		if dnsList.Items[i].Spec.IsRemote {
			continue
		}
		out = append(out, dnsList.Items[i])
	}
	return out, nil
}

// enabledKindsFromDNS unions the enabled source kinds across the given DNS CRs.
func enabledKindsFromDNS(dnsList []sreportalv1alpha2.DNS) map[registry.SourceType]bool {
	out := map[registry.SourceType]bool{}
	for i := range dnsList {
		for kind, on := range sourcepkg.EnabledKindsFromSpec(&dnsList[i].Spec.Sources) {
			if on {
				out[kind] = true
			}
		}
	}
	return out
}

// extractItems extracts client.Object slice from any *List via reflection.
func extractItems(list client.ObjectList) (items []client.Object, skipped int) {
	v := reflect.ValueOf(list).Elem().FieldByName("Items")
	if !v.IsValid() || v.Kind() != reflect.Slice {
		return nil, 0
	}
	out := make([]client.Object, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		// k8s core lists use value slices ([]T): take the element's address to
		// get *T. Other generated clients (e.g. istio client-go) use pointer
		// slices ([]*T): the element is already *T. Addr()-ing a pointer slice
		// yields **T, which is not a client.Object and used to panic here.
		if elem.Kind() != reflect.Pointer {
			elem = elem.Addr()
		}
		obj, ok := elem.Interface().(client.Object)
		if !ok {
			// Defensive: every source resolver's list element is a client.Object
			// once registered in the scheme. Skip rather than panic so a stray
			// type can't crash the SourceReconciler runnable; the caller logs
			// any skips.
			skipped++
			continue
		}
		out = append(out, obj)
	}
	return out, skipped
}
