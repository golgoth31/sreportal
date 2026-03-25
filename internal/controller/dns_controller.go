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
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/controller/dns"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/reconciler"
	"github.com/golgoth31/sreportal/internal/source"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

const (
	// DefaultRequeueAfter is the default requeue interval
	DefaultRequeueAfter = 5 * time.Minute
)

// DNSReconciler reconciles a DNS object
type DNSReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	chain      *reconciler.Chain[*sreportalv1alpha1.DNS, dns.ChainData]
	fqdnWriter domaindns.FQDNWriter
}

// SetFQDNWriter sets the FQDN read-store writer. When set, the reconciler
// pushes pre-aggregated FQDNViews into the store after each successful reconciliation.
func (r *DNSReconciler) SetFQDNWriter(w domaindns.FQDNWriter) {
	r.fqdnWriter = w
}

// NewDNSReconciler creates a new DNSReconciler with the handler chain.
// builders is the same list used by the source factory (e.g. source.DefaultBuilders())
// so that priority filtering and enabled sources stay in sync.
// When cfg.Reconciliation.DisableDNSCheck is true, the ResolveDNSHandler step
// is omitted and FQDNs will not carry a SyncStatus.
func NewDNSReconciler(c client.Client, scheme *runtime.Scheme, cfg *config.OperatorConfig, builders []registry.Builder) *DNSReconciler {
	var groupMappingConfig *config.GroupMappingConfig
	var sourcePriority []string
	disableDNSCheck := false
	if cfg != nil {
		groupMappingConfig = &cfg.GroupMapping
		sourcePriority = source.FilterPriorityOrder(cfg.Sources.Priority, builders, cfg)
		disableDNSCheck = cfg.Reconciliation.DisableDNSCheck
	}

	handlers := []reconciler.Handler[*sreportalv1alpha1.DNS, dns.ChainData]{
		dns.NewAggregateDNSRecordsHandler(c, groupMappingConfig, sourcePriority),
		dns.NewCollectManualEntriesHandler(),
		dns.NewAggregateFQDNsHandler(),
	}
	if !disableDNSCheck {
		handlers = append(handlers, dns.NewResolveDNSHandler(dns.NewNetResolver()))
	}
	handlers = append(handlers, dns.NewUpdateStatusHandler(c))

	return &DNSReconciler{
		Client: c,
		Scheme: scheme,
		chain:  reconciler.NewChain(handlers...),
	}
}

// +kubebuilder:rbac:groups=sreportal.io,resources=dns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=dns/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=dns/finalizers,verbs=update
// +kubebuilder:rbac:groups=sreportal.io,resources=dnsrecords,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=dnsrecords/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=dnsrecords/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DNSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	// Fetch the DNS resource
	var resource sreportalv1alpha1.DNS
	if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
		if client.IgnoreNotFound(err) == nil && r.fqdnWriter != nil {
			// Resource deleted — remove from read store
			resourceKey := req.Namespace + "/" + req.Name
			if wErr := r.fqdnWriter.Delete(ctx, resourceKey); wErr != nil {
				logger.Error(wErr, "failed to delete FQDN views from read store")
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip DNS resources owned by a Portal (these are managed by PortalReconciler for remote portals)
	for _, ownerRef := range resource.OwnerReferences {
		if ownerRef.Kind == "Portal" && ownerRef.Controller != nil && *ownerRef.Controller {
			logger.V(1).Info("skipping DNS resource owned by Portal (remote portal)",
				"name", resource.Name, "namespace", resource.Namespace, "portal", ownerRef.Name)
			return ctrl.Result{}, nil
		}
	}

	logger.Info("reconciling DNS resource", "name", resource.Name, "namespace", resource.Namespace)

	// Create reconcile context
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DNS, dns.ChainData]{
		Resource: &resource,
	}

	// Execute handler chain
	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation failed")
		metrics.ReconcileTotal.WithLabelValues("dns", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("dns").Observe(time.Since(start).Seconds())
		return ctrl.Result{}, err
	}

	// If no explicit requeue requested, requeue after default interval
	if rc.Result.RequeueAfter == 0 {
		rc.Result.RequeueAfter = DefaultRequeueAfter
	}

	// Update FQDN and group gauges
	portal := resource.Spec.PortalRef
	fqdnsBySource := make(map[string]int)
	for _, g := range resource.Status.Groups {
		fqdnsBySource[g.Source] += len(g.FQDNs)
	}
	for src, count := range fqdnsBySource {
		metrics.DNSFQDNsTotal.WithLabelValues(portal, src).Set(float64(count))
	}
	metrics.DNSGroupsTotal.WithLabelValues(portal).Set(float64(len(resource.Status.Groups)))

	metrics.ReconcileTotal.WithLabelValues("dns", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("dns").Observe(time.Since(start).Seconds())

	// Project aggregated groups into the FQDN read store
	if r.fqdnWriter != nil {
		resourceKey := resource.Namespace + "/" + resource.Name
		views := groupsToFQDNViews(&resource)
		if err := r.fqdnWriter.Replace(ctx, resourceKey, views); err != nil {
			logger.Error(err, "failed to update FQDN read store")
		}
	}

	logger.Info("reconciliation completed", "requeueAfter", rc.Result.RequeueAfter)
	return rc.Result, nil
}

// groupsToFQDNViews converts a DNS resource's status groups into a deduplicated,
// sorted slice of FQDNViews. This centralises the CRD → domain transformation
// that was previously duplicated across gRPC and MCP services.
func groupsToFQDNViews(resource *sreportalv1alpha1.DNS) []domaindns.FQDNView {
	seen := make(map[string]*domaindns.FQDNView)

	for _, group := range resource.Status.Groups {
		for _, fqdn := range group.FQDNs {
			key := fqdn.FQDN + "/" + fqdn.RecordType
			if existing, ok := seen[key]; ok {
				if !slices.Contains(existing.Groups, group.Name) {
					existing.Groups = append(existing.Groups, group.Name)
				}
			} else {
				view := domaindns.FQDNView{
					Name:        fqdn.FQDN,
					Source:      domaindns.Source(group.Source),
					Groups:      []string{group.Name},
					Description: fqdn.Description,
					RecordType:  fqdn.RecordType,
					Targets:     fqdn.Targets,
					LastSeen:    fqdn.LastSeen.Time,
					PortalName:  resource.Name,
					Namespace:   resource.Namespace,
					SyncStatus:  fqdn.SyncStatus,
				}
				if fqdn.OriginRef != nil {
					ref, _ := domaindns.ParseResourceRef(
						fqdn.OriginRef.Kind + "/" + fqdn.OriginRef.Namespace + "/" + fqdn.OriginRef.Name,
					)
					view.OriginRef = &ref
				}
				seen[key] = &view
			}
		}
	}

	views := make([]domaindns.FQDNView, 0, len(seen))
	for _, v := range seen {
		views = append(views, *v)
	}

	return views
}

// SetupWithManager sets up the controller with the Manager.
func (r *DNSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.DNS{}).
		Watches(&sreportalv1alpha1.DNSRecord{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				dnsRecord, ok := obj.(*sreportalv1alpha1.DNSRecord)
				if !ok {
					return nil
				}

				// Find DNS resources that share the same portalRef
				var dnsList sreportalv1alpha1.DNSList
				if err := r.List(ctx, &dnsList,
					client.InNamespace(dnsRecord.Namespace),
					client.MatchingFields{"spec.portalRef": dnsRecord.Spec.PortalRef},
				); err != nil {
					return nil
				}

				requests := make([]reconcile.Request, 0, len(dnsList.Items))
				for _, d := range dnsList.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      d.Name,
							Namespace: d.Namespace,
						},
					})
				}
				return requests
			},
		)).
		Named("dns").
		Complete(r)
}
