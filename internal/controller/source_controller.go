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

package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

const (
	// maxSourceConsecutiveFailures is the threshold after which a persistently
	// failing source is surfaced as a NotReady condition on its DNSRecord.
	maxSourceConsecutiveFailures = 5
)

// portalSourceKey identifies a unique (portal, sourceType) pair.
type portalSourceKey struct {
	portalName string
	sourceType registry.SourceType
}

// SourceReconciler reconciles external-dns sources and updates DNSRecord CRs.
type SourceReconciler struct {
	client.Client
	config          *config.OperatorConfig
	sourceFactory   *source.Factory
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface

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
	//
	// External visibility: when failures exceed maxSourceConsecutiveFailures the
	// state is surfaced as a NotReady condition on the DNSRecord via markSourceDegraded,
	// giving operators a Kubernetes-native signal without requiring an in-memory dashboard.
	sourceFailures map[registry.SourceType]int
}

// NewSourceReconciler creates a new SourceReconciler.
func NewSourceReconciler(
	c client.Client,
	kubeClient kubernetes.Interface,
	restConfig *rest.Config,
	cfg *config.OperatorConfig,
	builders []registry.Builder,
) *SourceReconciler {
	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		log.Default().WithName("source").Error(err, "failed to create dynamic client, annotation enrichment disabled")
	}
	discoClient := discovery.NewDiscoveryClientForConfigOrDie(restConfig)

	return &SourceReconciler{
		Client:          c,
		config:          cfg,
		sourceFactory:   source.NewFactory(kubeClient, restConfig, builders),
		dynamicClient:   dynClient,
		discoveryClient: discoClient,
		gvrCache:        make(map[schema.GroupResource]schema.GroupVersionResource),
		sourceFailures:  make(map[registry.SourceType]int),
	}
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="discovery.k8s.io",resources=endpointslices,verbs=get;list;watch

// Start implements manager.Runnable to run periodic source reconciliation.
//
// Error propagation: reconcile() errors are deliberately logged rather than
// returned to the manager. A transient failure (e.g. temporary API unavailability)
// should not stop the operator — the next tick will retry automatically. Persistent
// failures are surfaced as NotReady conditions on the relevant DNSRecord via
// markSourceDegraded, giving operators a Kubernetes-native signal.
func (r *SourceReconciler) Start(ctx context.Context) error {
	logger := log.Default().WithName("source")

	// Best-effort source initialisation at startup — sources may become available
	// later (e.g. CRDs not yet installed), so failures are non-fatal.
	if err := r.rebuildSources(ctx); err != nil {
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

// portalIndex is a pre-computed lookup structure built from the portal list.
type portalIndex struct {
	main   *sreportalv1alpha1.Portal
	byName map[string]*sreportalv1alpha1.Portal
	local  []*sreportalv1alpha1.Portal
}

func (r *SourceReconciler) reconcile(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("source")

	r.mu.RLock()
	sourcesBuilt := len(r.typedSources) > 0
	r.mu.RUnlock()

	if !sourcesBuilt {
		if err := r.rebuildSources(ctx); err != nil {
			return err
		}
	}

	idx, err := r.buildPortalIndex(ctx)
	if err != nil {
		return err
	}
	if idx == nil {
		return nil // no local portals to reconcile
	}

	r.mu.RLock()
	typedSources := r.typedSources
	r.mu.RUnlock()

	endpointsByPortalSource := r.collectByPortalSource(ctx, typedSources, idx)

	// Create/update DNSRecords for each (portal, sourceType) pair.
	for key, endpoints := range endpointsByPortalSource {
		portal := idx.byName[key.portalName]
		if portal == nil || portal.Spec.Remote != nil {
			continue
		}
		if err := r.reconcileDNSRecord(ctx, portal, key.sourceType, endpoints); err != nil {
			logger.Error(err, "failed to reconcile DNSRecord",
				"portal", key.portalName, "sourceType", key.sourceType)
		}
	}

	// Clean up orphaned DNSRecords for local portals.
	for _, portal := range idx.local {
		if err := r.deleteOrphanedDNSRecords(ctx, portal, endpointsByPortalSource); err != nil {
			logger.Error(err, "failed to delete orphaned DNSRecords", "portal", portal.Name)
		}
	}

	return nil
}

// buildPortalIndex lists all Portals and builds a lookup index for the reconciliation loop.
// Returns nil (without error) when there are no local portals to reconcile.
func (r *SourceReconciler) buildPortalIndex(ctx context.Context) (*portalIndex, error) {
	logger := log.FromContext(ctx).WithName("source")

	var portalList sreportalv1alpha1.PortalList
	if err := r.List(ctx, &portalList); err != nil {
		logger.Error(err, "failed to list Portal resources")
		return nil, err
	}

	if len(portalList.Items) == 0 {
		logger.Info("no portals found, skipping reconciliation")
		return nil, nil
	}

	idx := &portalIndex{
		byName: make(map[string]*sreportalv1alpha1.Portal, len(portalList.Items)),
		local:  make([]*sreportalv1alpha1.Portal, 0),
	}

	for i := range portalList.Items {
		p := &portalList.Items[i]
		idx.byName[p.Name] = p

		if p.Spec.Remote != nil {
			logger.V(1).Info("skipping remote portal for source collection", "name", p.Name, "url", p.Spec.Remote.URL)
			continue
		}

		idx.local = append(idx.local, p)
		if p.Spec.Main {
			idx.main = p
		}
	}

	if idx.main == nil {
		if len(idx.local) > 0 {
			idx.main = idx.local[0]
			logger.Info("no main portal found, using first local portal as fallback", "name", idx.main.Name)
		} else {
			logger.Info("no local portals found, skipping source reconciliation")
			return nil, nil
		}
	}

	return idx, nil
}

// collectByPortalSource gathers endpoints from every configured source and routes each
// endpoint to the appropriate (portalName, sourceType) bucket. Source failures are tracked
// via r.sourceFailures (single-goroutine access — see field comment) and surfaced as K8s
// conditions once maxSourceConsecutiveFailures is reached.
func (r *SourceReconciler) collectByPortalSource(
	ctx context.Context,
	typedSources []registry.TypedSource,
	idx *portalIndex,
) map[portalSourceKey][]*endpoint.Endpoint {
	logger := log.FromContext(ctx).WithName("source")

	result := make(map[portalSourceKey][]*endpoint.Endpoint)

	for _, ts := range typedSources {
		endpoints, err := ts.Source.Endpoints(ctx)
		if err != nil {
			r.sourceFailures[ts.Type]++
			count := r.sourceFailures[ts.Type]
			logger.Error(err, "failed to get endpoints from source",
				"sourceType", ts.Type, "consecutiveFailures", count)
			metrics.SourceErrorsTotal.WithLabelValues(string(ts.Type)).Inc()
			if count >= maxSourceConsecutiveFailures {
				r.markSourceDegraded(ctx, idx.main, ts.Type, err, count)
			}
			continue
		}

		if prev := r.sourceFailures[ts.Type]; prev > 0 {
			logger.Info("source recovered after consecutive failures",
				"sourceType", ts.Type, "previousFailures", prev)
			r.sourceFailures[ts.Type] = 0
		}

		metrics.SourceEndpointsCollected.WithLabelValues(string(ts.Type)).Set(float64(len(endpoints)))
		logger.V(1).Info("collected endpoints from source", "sourceType", ts.Type, "count", len(endpoints))

		r.enrichEndpoints(ctx, ts.Type, endpoints)

		for _, ep := range endpoints {
			portalName, target := r.resolveEndpointPortal(ctx, ep, idx)
			if target == nil {
				continue
			}
			key := portalSourceKey{portalName: portalName, sourceType: ts.Type}
			result[key] = append(result[key], ep)
		}
	}

	return result
}

// resolveEndpointPortal maps an endpoint to its target local portal.
// If the annotated portal is unknown or remote, the endpoint falls back to the main portal.
// Returns ("", nil) when the endpoint should be discarded.
func (r *SourceReconciler) resolveEndpointPortal(
	ctx context.Context,
	ep *endpoint.Endpoint,
	idx *portalIndex,
) (string, *sreportalv1alpha1.Portal) {
	logger := log.FromContext(ctx).WithName("source")

	portalName := adapter.ResolvePortal(ep)
	var target *sreportalv1alpha1.Portal

	if portalName == "" {
		portalName = idx.main.Name
		target = idx.main
	} else if p := idx.byName[portalName]; p == nil {
		logger.V(1).Info("portal not found, routing to main portal",
			"annotatedPortal", portalName, "endpoint", ep.DNSName)
		portalName = idx.main.Name
		target = idx.main
	} else if p.Spec.Remote != nil {
		logger.V(1).Info("portal is remote, routing to main portal",
			"annotatedPortal", portalName, "endpoint", ep.DNSName)
		portalName = idx.main.Name
		target = idx.main
	} else {
		target = p
	}

	if target == nil || target.Spec.Remote != nil {
		logger.V(1).Info("no valid local portal for endpoint, skipping", "endpoint", ep.DNSName)
		return "", nil
	}

	return portalName, target
}

func (r *SourceReconciler) rebuildSources(ctx context.Context) error {
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

// reconcileDNSRecord creates or updates a DNSRecord for a specific portal and source type.
func (r *SourceReconciler) reconcileDNSRecord(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	sourceType registry.SourceType,
	endpoints []*endpoint.Endpoint,
) error {
	logger := log.FromContext(ctx).WithName("source")

	name := fmt.Sprintf("%s-%s", portal.Name, sourceType)
	now := metav1.Now()

	// Create or update DNSRecord
	dnsRecord := &sreportalv1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: portal.Namespace,
		},
	}

	result, err := createOrUpdateSpec(ctx, r.Client, dnsRecord, func() error {
		dnsRecord.Spec.SourceType = string(sourceType)
		dnsRecord.Spec.PortalRef = portal.Name
		return nil
	})
	if err != nil {
		logger.Error(err, "failed to create/update DNSRecord",
			"name", name, "portal", portal.Name)
		return err
	}

	logger.V(1).Info("DNSRecord reconciled",
		"name", name,
		"result", result)

	// Update status with retry to handle cache sync delays and conflicts
	endpointStatus := adapter.ToEndpointStatus(endpoints)
	statusRetryBackoff := wait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    5,
	}

	err = retry.OnError(statusRetryBackoff, func(err error) bool {
		return apierrors.IsNotFound(err) || apierrors.IsConflict(err)
	}, func() error {
		dnsRecordKey := client.ObjectKey{Namespace: portal.Namespace, Name: name}
		if err := r.Get(ctx, dnsRecordKey, dnsRecord); err != nil {
			return err
		}

		dnsRecord.Status.Endpoints = endpointStatus
		dnsRecord.Status.LastReconcileTime = &now
		meta.SetStatusCondition(&dnsRecord.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "EndpointsCollected",
			Message:            fmt.Sprintf("Collected %d endpoints from %s source", len(endpoints), sourceType),
			LastTransitionTime: now,
		})

		return r.Status().Update(ctx, dnsRecord)
	})
	if err != nil {
		logger.Error(err, "failed to update DNSRecord status",
			"name", name, "portal", portal.Name)
		return err
	}

	return nil
}

// createOrUpdateSpec creates or updates only the spec of a DNSRecord with conflict retry.
func createOrUpdateSpec(ctx context.Context, c client.Client, obj *sreportalv1alpha1.DNSRecord, mutate func() error) (string, error) {
	key := client.ObjectKeyFromObject(obj)
	var result string

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existing := &sreportalv1alpha1.DNSRecord{}
		err := c.Get(ctx, key, existing)
		if apierrors.IsNotFound(err) {
			if err := mutate(); err != nil {
				return err
			}
			result = "created"
			return c.Create(ctx, obj)
		}
		if err != nil {
			return err
		}

		// Update existing
		obj.ObjectMeta = existing.ObjectMeta
		if err := mutate(); err != nil {
			return err
		}
		result = "updated"
		return c.Update(ctx, obj)
	})

	return result, err
}

// deleteOrphanedDNSRecords deletes DNSRecord resources that are no longer needed for a portal.
func (r *SourceReconciler) deleteOrphanedDNSRecords(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	activeKeys map[portalSourceKey][]*endpoint.Endpoint,
) error {
	logger := log.FromContext(ctx).WithName("source")

	enabledTypes := r.sourceFactory.EnabledSourceTypes(r.config)
	enabledSet := make(map[registry.SourceType]bool)
	for _, t := range enabledTypes {
		enabledSet[t] = true
	}

	// List DNSRecords for this portal
	var dnsRecordList sreportalv1alpha1.DNSRecordList
	if err := r.List(ctx, &dnsRecordList,
		client.InNamespace(portal.Namespace),
		client.MatchingFields{FieldIndexPortalRef: portal.Name},
	); err != nil {
		return err
	}

	// Delete DNSRecords for disabled sources or portals with no endpoints
	for i := range dnsRecordList.Items {
		rec := &dnsRecordList.Items[i]
		sourceType := registry.SourceType(rec.Spec.SourceType)

		key := portalSourceKey{portalName: portal.Name, sourceType: sourceType}

		if !enabledSet[sourceType] || activeKeys[key] == nil {
			logger.Info("deleting orphaned DNSRecord",
				"name", rec.Name,
				"sourceType", rec.Spec.SourceType,
				"portal", portal.Name)

			if err := r.Delete(ctx, rec); err != nil {
				logger.Error(err, "failed to delete orphaned DNSRecord",
					"name", rec.Name)
				return err
			}
		}
	}

	return nil
}

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

// enrichEndpoints looks up the original K8s resources and copies sreportal annotations
// (sreportal.io/portal, sreportal.io/groups) to endpoint labels.
func (r *SourceReconciler) enrichEndpoints(ctx context.Context, sourceType registry.SourceType, endpoints []*endpoint.Endpoint) {
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

// markSourceDegraded sets a NotReady condition on the DNSRecord that corresponds
// to the given portal and source type, surfacing a persistent collection failure.
// If the DNSRecord does not yet exist no action is taken (it will be created
// once the source recovers).
func (r *SourceReconciler) markSourceDegraded(
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
		// DNSRecord may not exist yet; skip condition update.
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

// GetTypedSources returns the current typed sources. This is useful for testing.
func (r *SourceReconciler) GetTypedSources() []registry.TypedSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.typedSources
}
