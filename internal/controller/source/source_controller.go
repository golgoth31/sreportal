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
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/config"
	sourcechain "github.com/golgoth31/sreportal/internal/controller/source/chain"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// Compile-time interface checks.
var (
	_ sourcechain.SourceProvider       = (*SourceReconciler)(nil)
	_ sourcechain.EndpointEnricher     = (*SourceReconciler)(nil)
	_ sourcechain.SourceFailureTracker = (*SourceReconciler)(nil)
	_ sourcechain.EnabledSourcesLister = (*SourceReconciler)(nil)
)

// SourceReconciler reconciles external-dns sources and updates DNSRecord CRs.
type SourceReconciler struct {
	client.Client
	config          *config.OperatorConfig
	sourceFactory   *source.Factory
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface

	chain *reconciler.Chain[struct{}, sourcechain.ChainData]

	mu           sync.RWMutex
	typedSources []registry.TypedSource

	// gvrCache resolves Group+Resource to full GVR when Version is empty (discovery).
	gvrCacheMu sync.RWMutex
	gvrCache   map[schema.GroupResource]schema.GroupVersionResource

	// sourceFailures tracks consecutive endpoint-collection failures per source type.
	//
	// Threading invariant: reconcile() is called sequentially from the single
	// goroutine that runs the ticker loop inside Start(). No concurrent access
	// to sourceFailures can occur, so no mutex is needed here.
	sourceFailures map[registry.SourceType]int
}

// NewSourceReconciler creates a new SourceReconciler.
func NewSourceReconciler(
	c client.Client,
	kubeClient kubernetes.Interface,
	restConfig *rest.Config,
	cfg *config.OperatorConfig,
	builders []registry.Builder,
	sourcePriority []string,
) *SourceReconciler {
	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		log.Default().WithName("source").Error(err, "failed to create dynamic client, annotation enrichment disabled")
	}
	discoClient := discovery.NewDiscoveryClientForConfigOrDie(restConfig)

	r := &SourceReconciler{
		Client:          c,
		config:          cfg,
		sourceFactory:   source.NewFactory(kubeClient, restConfig, builders),
		dynamicClient:   dynClient,
		discoveryClient: discoClient,
		gvrCache:        make(map[schema.GroupResource]schema.GroupVersionResource),
		sourceFailures:  make(map[registry.SourceType]int),
	}

	r.chain = reconciler.NewChain[struct{}, sourcechain.ChainData](
		sourcechain.NewRebuildSourcesHandler(r),
		sourcechain.NewBuildPortalIndexHandler(c),
		sourcechain.NewCollectEndpointsHandler(r, r),
		sourcechain.NewDeduplicateHandler(sourcePriority),
		sourcechain.NewReconcileDNSRecordsHandler(c),
		sourcechain.NewReconcileComponentsHandler(c),
		sourcechain.NewDeleteOrphanedHandler(c, r),
	)

	return r
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="discovery.k8s.io",resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=sreportal.io,resources=components,verbs=get;list;watch;create;update;delete

// Start implements manager.Runnable to run periodic source reconciliation.
//
// Error propagation: reconcile() errors are deliberately logged rather than
// returned to the manager. A transient failure (e.g. temporary API unavailability)
// should not stop the operator — the next tick will retry automatically. Persistent
// failures are surfaced as NotReady conditions on the relevant DNSRecord via
// MarkDegraded, giving operators a Kubernetes-native signal.
func (r *SourceReconciler) Start(ctx context.Context) error {
	logger := log.Default().WithName("source")

	// Best-effort source initialisation at startup — sources may become available
	// later (e.g. CRDs not yet installed), so failures are non-fatal.
	if err := r.RebuildSources(ctx); err != nil {
		logger.Error(err, "failed to build sources at startup, will retry on next tick")
	}

	ticker := time.NewTicker(r.config.Reconciliation.Interval.Duration())
	defer ticker.Stop()

	// Run once immediately so the first reconciliation does not wait a full interval.
	if err := r.reconcile(ctx); err != nil {
		logger.Error(err, "initial reconciliation failed")
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping source reconciler")
			return nil
		case <-ticker.C:
			if err := r.reconcile(ctx); err != nil {
				logger.Error(err, "periodic reconciliation failed")
			}
		}
	}
}

func (r *SourceReconciler) reconcile(ctx context.Context) error {
	rc := &reconciler.ReconcileContext[struct{}, sourcechain.ChainData]{}
	return r.chain.Execute(ctx, rc)
}

// ---------------------------------------------------------------------------
// SourceProvider implementation
// ---------------------------------------------------------------------------

// GetTypedSources returns the current typed sources.
func (r *SourceReconciler) GetTypedSources() []registry.TypedSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.typedSources
}

// RebuildSources rebuilds external-dns sources from the operator config.
func (r *SourceReconciler) RebuildSources(ctx context.Context) error {
	logger := ctrl.Log.WithName("source")
	logger.Info("rebuilding sources from config")

	typedSources, err := r.sourceFactory.BuildTypedSources(ctx, r.config)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.typedSources = typedSources
	r.mu.Unlock()

	logger.Info("sources rebuilt", "count", len(typedSources))
	return nil
}

// ---------------------------------------------------------------------------
// EndpointEnricher implementation
// ---------------------------------------------------------------------------

// EnrichEndpoints looks up the original K8s resources and copies sreportal annotations
// (sreportal.io/portal, sreportal.io/groups) to endpoint labels.
func (r *SourceReconciler) EnrichEndpoints(ctx context.Context, sourceType registry.SourceType, endpoints []*endpoint.Endpoint) {
	if r.dynamicClient == nil {
		return
	}

	gvr, ok := r.sourceFactory.GVRForSourceType(sourceType)
	if !ok {
		return
	}

	logger := log.FromContext(ctx).WithName("source")
	if gvr.Version == "" {
		resolved, err := r.resolveGVR(gvr)
		if err != nil {
			logger.V(2).Info("skip annotation enrichment, could not resolve GVR", "gvr", gvr, "error", err)
			return
		}
		gvr = resolved
	}

	// Group endpoints by resource reference to avoid duplicate lookups
	byResource := make(map[string][]*endpoint.Endpoint)
	for _, ep := range endpoints {
		if res, ok := ep.Labels[endpoint.ResourceLabelKey]; ok && res != "" {
			byResource[res] = append(byResource[res], ep)
		}
	}

	for res, eps := range byResource {
		// Parse resource label: "kind/namespace/name"
		parts := strings.SplitN(res, "/", 3)
		if len(parts) < 3 {
			continue
		}
		ns, name := parts[1], parts[2]

		obj, err := r.dynamicClient.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			logger.V(2).Info("failed to get resource for annotation enrichment",
				"resource", res, "error", err)
			continue
		}

		for _, ep := range eps {
			adapter.EnrichEndpointLabels(ep, obj.GetAnnotations())
		}
	}
}

// ---------------------------------------------------------------------------
// SourceFailureTracker implementation
// ---------------------------------------------------------------------------

// RecordFailure increments the consecutive failure counter for a source type
// and returns the new count.
func (r *SourceReconciler) RecordFailure(sourceType registry.SourceType) int {
	r.sourceFailures[sourceType]++
	return r.sourceFailures[sourceType]
}

// RecordRecovery resets the failure counter for a source type and returns the
// previous count (0 if the source was not failing).
func (r *SourceReconciler) RecordRecovery(sourceType registry.SourceType) int {
	prev := r.sourceFailures[sourceType]
	if prev > 0 {
		r.sourceFailures[sourceType] = 0
	}
	return prev
}

// MarkDegraded sets a NotReady condition on the DNSRecord that corresponds
// to the given portal and source type, surfacing a persistent collection failure.
// If the DNSRecord does not yet exist no action is taken.
func (r *SourceReconciler) MarkDegraded(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	sourceType registry.SourceType,
	cause error,
	count int,
) {
	logger := log.FromContext(ctx).WithName("source")
	name := fmt.Sprintf("%s-%s", portal.Name, sourceType)

	var rec sreportalv1alpha1.DNSRecord
	if err := r.Get(ctx, client.ObjectKey{Namespace: portal.Namespace, Name: name}, &rec); err != nil {
		return
	}

	base := rec.DeepCopy()
	now := metav1.Now()
	meta.SetStatusCondition(&rec.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "SourceUnavailable",
		Message:            fmt.Sprintf("Source failed %d consecutive times: %v", count, cause),
		LastTransitionTime: now,
	})

	if err := r.Status().Patch(ctx, &rec, client.MergeFrom(base)); err != nil {
		logger.Error(err, "failed to patch degraded condition on DNSRecord", "dnsRecord", name)
	}
}

// ---------------------------------------------------------------------------
// EnabledSourcesLister implementation
// ---------------------------------------------------------------------------

// EnabledSourceTypes returns the list of source types currently enabled in config.
func (r *SourceReconciler) EnabledSourceTypes() []registry.SourceType {
	return r.sourceFactory.EnabledSourceTypes(r.config)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// resolveGVR returns a full GVR. If gvr.Version is empty, it uses discovery to resolve
// the preferred version and caches the result.
func (r *SourceReconciler) resolveGVR(gvr schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	if gvr.Version != "" {
		return gvr, nil
	}
	gr := gvr.GroupResource()
	r.gvrCacheMu.RLock()
	if resolved, ok := r.gvrCache[gr]; ok {
		r.gvrCacheMu.RUnlock()
		return resolved, nil
	}
	r.gvrCacheMu.RUnlock()

	lists, err := r.discoveryClient.ServerPreferredResources()
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("discovery: %w", err)
	}
	for _, list := range lists {
		if list.GroupVersion == "" {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil || gv.Group != gvr.Group {
			continue
		}
		for i := range list.APIResources {
			if list.APIResources[i].Name == gvr.Resource {
				resolved := schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: gvr.Resource,
				}
				r.gvrCacheMu.Lock()
				r.gvrCache[gr] = resolved
				r.gvrCacheMu.Unlock()
				return resolved, nil
			}
		}
	}
	return schema.GroupVersionResource{}, fmt.Errorf("no version found for %s/%s", gvr.Group, gvr.Resource)
}

// SetupWithManager adds the reconciler as a runnable to the manager.
func (r *SourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return mgr.Add(manager.RunnableFunc(r.Start))
}

// SetTypedSources sets the typed sources directly. This is useful for testing.
func (r *SourceReconciler) SetTypedSources(sources []registry.TypedSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.typedSources = sources
}
