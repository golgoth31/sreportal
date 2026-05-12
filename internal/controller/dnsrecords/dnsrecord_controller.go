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

package dnsrecords

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	dnsrecordchain "github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// DNSRecordReconciler reconciles a DNSRecord object and projects its endpoints
// directly into the FQDN read store via a Chain-of-Responsibility pipeline.
type DNSRecordReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	fqdnWriter      domaindns.FQDNWriter
	groupMapping    *config.GroupMappingConfig
	resolver        domaindns.Resolver
	disableDNSCheck bool
	chain           *reconciler.Chain[*sreportalv1alpha1.DNSRecord, dnsrecordchain.ChainData]
}

// NewDNSRecordReconciler creates a new DNSRecordReconciler.
// When disableDNSCheck is true, DNS resolution is skipped and SyncStatus remains empty.
func NewDNSRecordReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	groupMapping *config.GroupMappingConfig,
	resolver domaindns.Resolver,
	disableDNSCheck bool,
) *DNSRecordReconciler {
	r := &DNSRecordReconciler{
		Client:          c,
		Scheme:          scheme,
		groupMapping:    groupMapping,
		resolver:        resolver,
		disableDNSCheck: disableDNSCheck,
	}
	r.rebuildChain()
	return r
}

// SetFQDNWriter sets the FQDN read-store writer and rebuilds the chain so the
// ProjectStoreHandler picks up the new writer.
func (r *DNSRecordReconciler) SetFQDNWriter(w domaindns.FQDNWriter) {
	r.fqdnWriter = w
	r.rebuildChain()
}

func (r *DNSRecordReconciler) rebuildChain() {
	r.chain = reconciler.NewChain(
		"dnsrecord",
		dnsrecordchain.NewSyncEndpointsHashHandler(r.Client),
		dnsrecordchain.NewResolveDNSHandler(r.Client, r.resolver, r.disableDNSCheck),
		dnsrecordchain.NewProjectStoreHandler(r.fqdnWriter, r.groupMapping),
	)
}

// +kubebuilder:rbac:groups=sreportal.io,resources=portals,verbs=get;list;watch

// Reconcile loads the DNSRecord, handles the not-found / feature-disabled
// short-circuits inline, then runs the chain to sync the hash, resolve DNS,
// and project entries into the FQDN read store.
func (r *DNSRecordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	var record sreportalv1alpha1.DNSRecord
	if err := r.Get(ctx, req.NamespacedName, &record); err != nil {
		if client.IgnoreNotFound(err) == nil && r.fqdnWriter != nil {
			resourceKey := req.Namespace + "/" + req.Name
			if wErr := r.fqdnWriter.Delete(ctx, resourceKey); wErr != nil {
				logger.Error(wErr, "failed to delete FQDN views from read store")
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconciling DNSRecord resource", "name", record.Name, "namespace", record.Namespace,
		"portal", record.Spec.PortalRef, "sourceType", record.Spec.SourceType)

	// Skip reconciliation when DNS feature is disabled on the referenced portal.
	// Cleanup of read store entries and DNSRecord resources is handled by the
	// portal controller when the toggle changes.
	if record.Spec.PortalRef != "" {
		var portal sreportalv1alpha1.Portal
		if err := r.Get(ctx, client.ObjectKey{Name: record.Spec.PortalRef, Namespace: record.Namespace}, &portal); err == nil {
			if !portal.Spec.Features.IsDNSEnabled() {
				logger.V(1).Info("DNS feature disabled for portal, skipping", "portal", record.Spec.PortalRef)
				return ctrl.Result{}, nil
			}
		}
	}

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DNSRecord, dnsrecordchain.ChainData]{
		Resource: &record,
		Data: dnsrecordchain.ChainData{
			ResourceKey: record.Namespace + "/" + record.Name,
		},
	}
	if err := r.chain.Execute(ctx, rc); err != nil {
		metrics.ReconcileTotal.WithLabelValues("dnsrecord", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("dnsrecord", "").Observe(time.Since(start).Seconds())
		return ctrl.Result{}, err
	}

	metrics.DNSFQDNsTotal.WithLabelValues(record.Spec.PortalRef, "external-dns").Set(float64(len(record.Status.Endpoints)))
	metrics.ReconcileTotal.WithLabelValues("dnsrecord", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("dnsrecord", "").Observe(time.Since(start).Seconds())

	return rc.Result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DNSRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.DNSRecord{}).
		Watches(
			&sreportalv1alpha1.Portal{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
				portal, ok := obj.(*sreportalv1alpha1.Portal)
				if !ok {
					return nil
				}
				return portalfeatures.DnsRecordReconcileRequestsForPortal(ctx, r.Client, portal)
			}),
			builder.WithPredicates(portalfeatures.PortalDNSEnabledWakeupPredicate()),
		).
		Named("dnsrecord").
		Complete(r)
}
