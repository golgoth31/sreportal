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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/controller/dns"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// DefaultRequeueAfter is the default requeue interval
	DefaultRequeueAfter = 5 * time.Minute
)

// DNSReconciler reconciles a DNS object
type DNSReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	chain  *reconciler.Chain[*sreportalv1alpha1.DNS]
}

// NewDNSReconciler creates a new DNSReconciler with the handler chain.
// When cfg.Reconciliation.DisableDNSCheck is true, the ResolveDNSHandler step
// is omitted and FQDNs will not carry a SyncStatus.
func NewDNSReconciler(c client.Client, scheme *runtime.Scheme, cfg *config.OperatorConfig) *DNSReconciler {
	var groupMappingConfig *config.GroupMappingConfig
	var sourcePriority []string
	disableDNSCheck := false
	if cfg != nil {
		groupMappingConfig = &cfg.GroupMapping
		sourcePriority = filteredSourcePriority(cfg)
		disableDNSCheck = cfg.Reconciliation.DisableDNSCheck
	}

	handlers := []reconciler.Handler[*sreportalv1alpha1.DNS]{
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

// filteredSourcePriority returns the configured Sources.Priority list filtered to only
// include enabled sources. When a source appears in the priority list but is disabled
// (or missing) in configuration, a warning is logged and the source is ignored.
func filteredSourcePriority(cfg *config.OperatorConfig) []string {
	if cfg == nil {
		return nil
	}

	log := ctrl.Log.WithName("dns").WithName("priority")

	enabled := func(name string) bool {
		switch name {
		case "service":
			return cfg.Sources.Service != nil && cfg.Sources.Service.Enabled
		case "ingress":
			return cfg.Sources.Ingress != nil && cfg.Sources.Ingress.Enabled
		case "dnsendpoint":
			return cfg.Sources.DNSEndpoint != nil && cfg.Sources.DNSEndpoint.Enabled
		case "istio-gateway":
			return cfg.Sources.IstioGateway != nil && cfg.Sources.IstioGateway.Enabled
		case "istio-virtualservice":
			return cfg.Sources.IstioVirtualService != nil && cfg.Sources.IstioVirtualService.Enabled
		default:
			// Unknown source type – keep it as-is; ApplySourcePriority will simply
			// ignore it if no endpoints are produced for that source.
			return true
		}
	}

	var filtered []string
	for _, src := range cfg.Sources.Priority {
		if !enabled(src) {
			log.Info("source in priority list is disabled and will be ignored",
				"source", src)
			continue
		}
		filtered = append(filtered, src)
	}
	return filtered
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
	log := logf.FromContext(ctx)

	// Fetch the DNS resource
	var resource sreportalv1alpha1.DNS
	if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip DNS resources owned by a Portal (these are managed by PortalReconciler for remote portals)
	for _, ownerRef := range resource.OwnerReferences {
		if ownerRef.Kind == "Portal" && ownerRef.Controller != nil && *ownerRef.Controller {
			log.V(1).Info("skipping DNS resource owned by Portal (remote portal)",
				"name", resource.Name, "namespace", resource.Namespace, "portal", ownerRef.Name)
			return ctrl.Result{}, nil
		}
	}

	log.Info("reconciling DNS resource", "name", resource.Name, "namespace", resource.Namespace)

	// Create reconcile context
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DNS]{
		Resource: &resource,
		Data:     make(map[string]any),
	}

	// Execute handler chain
	if err := r.chain.Execute(ctx, rc); err != nil {
		log.Error(err, "reconciliation failed")
		return ctrl.Result{}, err
	}

	// If no explicit requeue requested, requeue after default interval
	if rc.Result.RequeueAfter == 0 {
		rc.Result.RequeueAfter = DefaultRequeueAfter
	}

	log.Info("reconciliation completed", "requeueAfter", rc.Result.RequeueAfter)
	return rc.Result, nil
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
