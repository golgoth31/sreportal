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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/adapter"
	"github.com/golgoth31/sreportal/internal/config"
	srcfactory "github.com/golgoth31/sreportal/internal/source"
)

const (
	// PortalAnnotationKey is the annotation key used on external-dns endpoints
	// to route them to a specific portal.
	PortalAnnotationKey = "sreportal.io/portal"

	// maxSourceConsecutiveFailures is the threshold after which a persistently
	// failing source is surfaced as a NotReady condition on its DNSRecord.
	maxSourceConsecutiveFailures = 5
)

// portalSourceKey identifies a unique (portal, sourceType) pair.
type portalSourceKey struct {
	portalName string
	sourceType srcfactory.SourceType
}

// SourceReconciler reconciles external-dns sources and updates DNSRecord CRs.
type SourceReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	KubeClient    kubernetes.Interface
	RestConfig    *rest.Config
	Config        *config.OperatorConfig
	sourceFactory *srcfactory.Factory
	dynamicClient dynamic.Interface

	mu           sync.RWMutex
	typedSources []srcfactory.TypedSource

	// sourceFailures tracks consecutive endpoint-collection failures per source
	// type. It is only accessed from the Start() goroutine (reconcile is called
	// sequentially from the ticker loop), so no synchronization is required.
	sourceFailures map[srcfactory.SourceType]int
}

// NewSourceReconciler creates a new SourceReconciler.
func NewSourceReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	kubeClient kubernetes.Interface,
	restConfig *rest.Config,
	cfg *config.OperatorConfig,
) *SourceReconciler {
	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		ctrl.Log.WithName("source").Error(err, "failed to create dynamic client, annotation enrichment disabled")
	}

	return &SourceReconciler{
		Client:         c,
		Scheme:         scheme,
		KubeClient:     kubeClient,
		RestConfig:     restConfig,
		Config:         cfg,
		sourceFactory:  srcfactory.NewFactory(kubeClient, restConfig),
		dynamicClient:  dynClient,
		sourceFailures: make(map[srcfactory.SourceType]int),
	}
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="discovery.k8s.io",resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch
// +kubebuilder:rbac:groups=externaldns.k8s.io,resources=dnsendpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.istio.io,resources=gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=get;list;watch

// Start implements manager.Runnable interface to run periodic reconciliation.
func (r *SourceReconciler) Start(ctx context.Context) error {
	log := ctrl.Log.WithName("source")

	// Build sources at startup
	if err := r.rebuildSources(ctx); err != nil {
		log.Error(err, "failed to build sources at startup")
		// Continue anyway - sources may become available later
	}

	// Run periodic reconciliation
	ticker := time.NewTicker(r.Config.Reconciliation.Interval.Duration())
	defer ticker.Stop()

	// Run once immediately
	if err := r.reconcile(ctx); err != nil {
		log.Error(err, "initial reconciliation failed")
	}

	for {
		select {
		case <-ctx.Done():
			log.Info("stopping source reconciler")
			return nil
		case <-ticker.C:
			if err := r.reconcile(ctx); err != nil {
				log.Error(err, "periodic reconciliation failed")
			}
		}
	}
}

func (r *SourceReconciler) reconcile(ctx context.Context) error {
	log := logf.FromContext(ctx).WithName("source")

	// Ensure sources are built
	r.mu.RLock()
	sourcesBuilt := len(r.typedSources) > 0
	r.mu.RUnlock()

	if !sourcesBuilt {
		if err := r.rebuildSources(ctx); err != nil {
			return err
		}
	}

	// List all Portals
	var portalList sreportalv1alpha1.PortalList
	if err := r.List(ctx, &portalList); err != nil {
		log.Error(err, "failed to list Portal resources")
		return err
	}

	if len(portalList.Items) == 0 {
		log.Info("no portals found, skipping reconciliation")
		return nil
	}

	// Find the main portal and filter out remote portals
	var mainPortal *sreportalv1alpha1.Portal
	portalsByName := make(map[string]*sreportalv1alpha1.Portal)
	localPortals := make([]*sreportalv1alpha1.Portal, 0)

	for i := range portalList.Items {
		p := &portalList.Items[i]
		portalsByName[p.Name] = p

		// Skip remote portals for local source collection
		if p.Spec.Remote != nil {
			log.V(1).Info("skipping remote portal for source collection", "name", p.Name, "url", p.Spec.Remote.URL)
			continue
		}

		localPortals = append(localPortals, p)
		if p.Spec.Main {
			mainPortal = p
		}
	}

	if mainPortal == nil {
		// Fallback: use first local portal if no main portal found
		if len(localPortals) > 0 {
			mainPortal = localPortals[0]
			log.Info("no main portal found, using first local portal as fallback", "name", mainPortal.Name)
		} else {
			log.Info("no local portals found, skipping source reconciliation")
			return nil
		}
	}

	// Collect all endpoints from all sources
	r.mu.RLock()
	typedSources := r.typedSources
	r.mu.RUnlock()

	// Group endpoints by (portalName, sourceType)
	endpointsByPortalSource := make(map[portalSourceKey][]*endpoint.Endpoint)

	for _, ts := range typedSources {
		endpoints, err := ts.Source.Endpoints(ctx)
		if err != nil {
			r.sourceFailures[ts.Type]++
			count := r.sourceFailures[ts.Type]
			log.Error(err, "failed to get endpoints from source",
				"sourceType", ts.Type, "consecutiveFailures", count)
			if count >= maxSourceConsecutiveFailures {
				r.markSourceDegraded(ctx, mainPortal, ts.Type, err, count)
			}
			continue
		}

		if prev := r.sourceFailures[ts.Type]; prev > 0 {
			log.Info("source recovered after consecutive failures",
				"sourceType", ts.Type, "previousFailures", prev)
			r.sourceFailures[ts.Type] = 0
		}

		log.V(1).Info("collected endpoints from source",
			"sourceType", ts.Type,
			"count", len(endpoints))

		// Enrich endpoints with sreportal annotations from K8s resources
		r.enrichEndpoints(ctx, ts.Type, endpoints)

		// Route each endpoint to the appropriate portal
		for _, ep := range endpoints {
			portalName := adapter.ResolvePortal(ep)
			targetPortal := mainPortal

			if portalName == "" {
				// No annotation -> route to main portal
				portalName = mainPortal.Name
			} else if p, exists := portalsByName[portalName]; !exists {
				// Annotated portal doesn't exist -> route to main portal
				log.V(1).Info("portal not found, routing to main portal",
					"annotatedPortal", portalName,
					"endpoint", ep.DNSName)
				portalName = mainPortal.Name
			} else if p.Spec.Remote != nil {
				// Annotated portal is a remote portal -> route to main portal
				log.V(1).Info("portal is remote, routing to main portal",
					"annotatedPortal", portalName,
					"endpoint", ep.DNSName)
				portalName = mainPortal.Name
			} else {
				targetPortal = p
			}

			// Ensure we have a valid local portal
			if targetPortal == nil || targetPortal.Spec.Remote != nil {
				log.V(1).Info("no valid local portal for endpoint, skipping",
					"endpoint", ep.DNSName)
				continue
			}

			key := portalSourceKey{portalName: portalName, sourceType: ts.Type}
			endpointsByPortalSource[key] = append(endpointsByPortalSource[key], ep)
		}
	}

	// Ensure a DNS resource exists for each local portal so that the
	// DNSController can aggregate DNSRecord endpoints into DNS.Status.Groups.
	for _, portal := range localPortals {
		if err := r.ensureDNSResource(ctx, portal); err != nil {
			log.Error(err, "failed to ensure DNS resource", "portal", portal.Name)
		}
	}

	// Create/update DNSRecords for each (portal, sourceType) pair
	for key, endpoints := range endpointsByPortalSource {
		portal := portalsByName[key.portalName]
		// Skip remote portals
		if portal == nil || portal.Spec.Remote != nil {
			continue
		}
		if err := r.reconcileDNSRecord(ctx, portal, key.sourceType, endpoints); err != nil {
			log.Error(err, "failed to reconcile DNSRecord",
				"portal", key.portalName, "sourceType", key.sourceType)
		}
	}

	// Clean up orphaned DNSRecords (only for local portals)
	for _, portal := range localPortals {
		if err := r.deleteOrphanedDNSRecords(ctx, portal, endpointsByPortalSource); err != nil {
			log.Error(err, "failed to delete orphaned DNSRecords", "portal", portal.Name)
		}
	}

	return nil
}

func (r *SourceReconciler) rebuildSources(ctx context.Context) error {
	log := ctrl.Log.WithName("source")
	log.Info("rebuilding sources from config")

	typedSources, err := r.sourceFactory.BuildTypedSources(ctx, r.Config)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.typedSources = typedSources
	r.mu.Unlock()

	log.Info("sources rebuilt", "count", len(typedSources))
	return nil
}

// ensureDNSResource creates a DNS resource for the given portal if one does not
// already exist. This is required so the DNSController can aggregate DNSRecord
// endpoints into DNS.Status.Groups (which is what ListFQDNs reads).
func (r *SourceReconciler) ensureDNSResource(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
) error {
	log := logf.FromContext(ctx).WithName("source")

	name := portal.Name
	key := client.ObjectKey{Namespace: portal.Namespace, Name: name}

	var existing sreportalv1alpha1.DNS
	if err := r.Get(ctx, key, &existing); err == nil {
		// Already exists — nothing to do.
		return nil
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("check DNS resource: %w", err)
	}

	dns := &sreportalv1alpha1.DNS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: portal.Namespace,
		},
		Spec: sreportalv1alpha1.DNSSpec{
			PortalRef: portal.Name,
		},
	}

	if err := r.Create(ctx, dns); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create DNS resource: %w", err)
	}

	log.Info("created DNS resource for portal", "name", name, "namespace", portal.Namespace)
	return nil
}

// reconcileDNSRecord creates or updates a DNSRecord for a specific portal and source type.
func (r *SourceReconciler) reconcileDNSRecord(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	sourceType srcfactory.SourceType,
	endpoints []*endpoint.Endpoint,
) error {
	log := logf.FromContext(ctx).WithName("source")

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
		log.Error(err, "failed to create/update DNSRecord",
			"name", name, "portal", portal.Name)
		return err
	}

	log.V(1).Info("DNSRecord reconciled",
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
		setDNSRecordCondition(&dnsRecord.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "EndpointsCollected",
			Message:            fmt.Sprintf("Collected %d endpoints from %s source", len(endpoints), sourceType),
			LastTransitionTime: now,
		})

		return r.Status().Update(ctx, dnsRecord)
	})
	if err != nil {
		log.Error(err, "failed to update DNSRecord status",
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
	log := logf.FromContext(ctx).WithName("source")

	// Get enabled source types
	enabledTypes := srcfactory.EnabledSourceTypes(r.Config)
	enabledSet := make(map[srcfactory.SourceType]bool)
	for _, t := range enabledTypes {
		enabledSet[t] = true
	}

	// List DNSRecords for this portal
	var dnsRecordList sreportalv1alpha1.DNSRecordList
	if err := r.List(ctx, &dnsRecordList,
		client.InNamespace(portal.Namespace),
		client.MatchingFields{"spec.portalRef": portal.Name},
	); err != nil {
		return err
	}

	// Delete DNSRecords for disabled sources or portals with no endpoints
	for i := range dnsRecordList.Items {
		rec := &dnsRecordList.Items[i]
		sourceType := srcfactory.SourceType(rec.Spec.SourceType)

		key := portalSourceKey{portalName: portal.Name, sourceType: sourceType}

		if !enabledSet[sourceType] || activeKeys[key] == nil {
			log.Info("deleting orphaned DNSRecord",
				"name", rec.Name,
				"sourceType", rec.Spec.SourceType,
				"portal", portal.Name)

			if err := r.Delete(ctx, rec); err != nil {
				log.Error(err, "failed to delete orphaned DNSRecord",
					"name", rec.Name)
				return err
			}
		}
	}

	return nil
}

// setDNSRecordCondition sets or updates a condition in the conditions slice.
func setDNSRecordCondition(conditions *[]metav1.Condition, newCondition metav1.Condition) {
	if conditions == nil {
		return
	}

	for i, c := range *conditions {
		if c.Type == newCondition.Type {
			if c.Status != newCondition.Status {
				(*conditions)[i] = newCondition
			} else {
				newCondition.LastTransitionTime = c.LastTransitionTime
				(*conditions)[i] = newCondition
			}
			return
		}
	}

	*conditions = append(*conditions, newCondition)
}

// enrichEndpoints looks up the original K8s resources and copies sreportal annotations
// (sreportal.io/portal, sreportal.io/groups) to endpoint labels.
func (r *SourceReconciler) enrichEndpoints(ctx context.Context, sourceType srcfactory.SourceType, endpoints []*endpoint.Endpoint) {
	if r.dynamicClient == nil {
		return
	}

	gvr, ok := gvrForSourceType(sourceType)
	if !ok {
		return
	}

	log := logf.FromContext(ctx).WithName("source")

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
			log.V(2).Info("failed to get resource for annotation enrichment",
				"resource", res, "error", err)
			continue
		}

		for _, ep := range eps {
			adapter.EnrichEndpointLabels(ep, obj.GetAnnotations())
		}
	}
}

// gvrForSourceType returns the GroupVersionResource for a given source type.
func gvrForSourceType(sourceType srcfactory.SourceType) (schema.GroupVersionResource, bool) {
	switch sourceType {
	case srcfactory.SourceTypeService:
		return schema.GroupVersionResource{Version: "v1", Resource: "services"}, true
	case srcfactory.SourceTypeIngress:
		return schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, true
	case srcfactory.SourceTypeIstioGateway:
		return schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1", Resource: "gateways"}, true
	case srcfactory.SourceTypeIstioVirtualService:
		return schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1", Resource: "virtualservices"}, true
	default:
		return schema.GroupVersionResource{}, false
	}
}

// markSourceDegraded sets a NotReady condition on the DNSRecord that corresponds
// to the given portal and source type, surfacing a persistent collection failure.
// If the DNSRecord does not yet exist no action is taken (it will be created
// once the source recovers).
func (r *SourceReconciler) markSourceDegraded(
	ctx context.Context,
	portal *sreportalv1alpha1.Portal,
	sourceType srcfactory.SourceType,
	cause error,
	count int,
) {
	log := logf.FromContext(ctx).WithName("source")
	name := fmt.Sprintf("%s-%s", portal.Name, sourceType)

	var rec sreportalv1alpha1.DNSRecord
	if err := r.Get(ctx, client.ObjectKey{Namespace: portal.Namespace, Name: name}, &rec); err != nil {
		// DNSRecord may not exist yet; skip condition update.
		return
	}

	now := metav1.Now()
	setDNSRecordCondition(&rec.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "SourceUnavailable",
		Message:            fmt.Sprintf("Source failed %d consecutive times: %v", count, cause),
		LastTransitionTime: now,
	})

	if err := r.Status().Update(ctx, &rec); err != nil {
		log.Error(err, "failed to update degraded condition on DNSRecord", "dnsRecord", name)
	}
}

// SetupWithManager adds the reconciler as a runnable to the manager.
func (r *SourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return mgr.Add(manager.RunnableFunc(r.Start))
}

// SetTypedSources sets the typed sources directly. This is useful for testing.
func (r *SourceReconciler) SetTypedSources(sources []srcfactory.TypedSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.typedSources = sources
}

// GetTypedSources returns the current typed sources. This is useful for testing.
func (r *SourceReconciler) GetTypedSources() []srcfactory.TypedSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.typedSources
}
