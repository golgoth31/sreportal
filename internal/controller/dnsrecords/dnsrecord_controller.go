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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnsrecordchain "github.com/golgoth31/sreportal/internal/controller/dnsrecords/chain"
	portalfeatures "github.com/golgoth31/sreportal/internal/controller/portal/features"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// DNSRecordResolveInterval is the RequeueAfter applied at the end of every
// reconcile to re-run the DNS resolution drift check. The DNSRecord chain
// is otherwise purely event-driven on spec changes (generation bumps).
const DNSRecordResolveInterval = 1 * time.Hour

// DNSRecordReconciler reconciles a v1alpha2 DNSRecord object and projects its
// endpoints directly into the FQDN read store via a Chain-of-Responsibility
// pipeline. Group mapping and DisableDNSCheck are loaded from the referenced
// DNS CR by the first chain step (LoadDNSConfigHandler).
type DNSRecordReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	fqdnWriter domaindns.FQDNWriter
	resolver   domaindns.Resolver
	chain      *reconciler.Chain[*v1alpha2.DNSRecord, dnsrecordchain.ChainData]
}

// NewDNSRecordReconciler creates a new DNSRecordReconciler.
func NewDNSRecordReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	resolver domaindns.Resolver,
) *DNSRecordReconciler {
	r := &DNSRecordReconciler{
		Client:   c,
		Scheme:   scheme,
		resolver: resolver,
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
		dnsrecordchain.NewLoadDNSConfigHandler(r.Client),
		dnsrecordchain.NewMaterialiseEntriesHandler(),
		dnsrecordchain.NewResolveDNSHandler(r.Client, r.resolver),
		dnsrecordchain.NewProjectStoreHandler(r.fqdnWriter),
	)
}

// +kubebuilder:rbac:groups=sreportal.io,resources=dnsrecords,verbs=get;list;watch
// +kubebuilder:rbac:groups=sreportal.io,resources=dnsrecords/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=portals,verbs=get;list;watch
// +kubebuilder:rbac:groups=sreportal.io,resources=dns,verbs=get;list;watch

// Reconcile loads the DNSRecord, handles the not-found / feature-disabled
// short-circuits inline, then runs the chain to load DNS config, materialise
// manual entries, sync the hash, resolve DNS, and project entries into the
// FQDN read store.
func (r *DNSRecordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	var record v1alpha2.DNSRecord
	if err := r.Get(ctx, req.NamespacedName, &record); err != nil {
		if client.IgnoreNotFound(err) == nil && r.fqdnWriter != nil {
			resourceKey := req.Namespace + "/" + req.Name
			if wErr := r.fqdnWriter.Delete(ctx, resourceKey); wErr != nil {
				logger.Error(wErr, "failed to delete FQDN views from read store")
				return ctrl.Result{}, wErr
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconciling DNSRecord resource", "name", record.Name, "namespace", record.Namespace,
		"portal", record.Spec.PortalRef, "origin", record.Spec.Origin, "sourceType", record.Spec.SourceType)

	// Fast-out when the referenced Portal does not exist: drop any read store
	// contribution from this DNSRecord and return without requeue. This avoids
	// running the full reconcile chain (and emitting errors) when the Portal
	// has been deleted but the DNS/DNSRecord cleanup hasn't propagated yet.
	if record.Spec.PortalRef != "" {
		var portal sreportalv1alpha1.Portal
		if err := r.Get(ctx, types.NamespacedName{Namespace: record.Namespace, Name: record.Spec.PortalRef}, &portal); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("portal not found, dropping DNSRecord from read store", "portal", record.Spec.PortalRef)
				if r.fqdnWriter != nil {
					resourceKey := record.Namespace + "/" + record.Name
					if wErr := r.fqdnWriter.Delete(ctx, resourceKey); wErr != nil {
						return ctrl.Result{}, wErr
					}
				}
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
	}

	// Skip reconciliation when DNS feature is disabled on the referenced portal.
	// Cleanup of read store entries and DNSRecord resources is handled by the
	// portal controller when the toggle changes.
	if record.Spec.PortalRef != "" {
		enabled, err := portalfeatures.LookupPortalFeature(ctx, r.Client, record.Namespace, record.Spec.PortalRef,
			func(f *sreportalv1alpha1.PortalFeatures) bool { return f.IsDNSEnabled() })
		if err != nil {
			return ctrl.Result{}, err
		}
		if !enabled {
			logger.V(1).Info("DNS feature disabled for portal, skipping", "portal", record.Spec.PortalRef)
			return ctrl.Result{}, nil
		}
	}

	ownerDNSName := ""
	for _, or := range record.OwnerReferences {
		if or.Controller != nil && *or.Controller && or.Kind == "DNS" {
			ownerDNSName = or.Name
			break
		}
	}

	rc := &reconciler.ReconcileContext[*v1alpha2.DNSRecord, dnsrecordchain.ChainData]{
		Resource: &record,
		Data: dnsrecordchain.ChainData{
			ResourceKey:  record.Namespace + "/" + record.Name,
			OwnerDNSName: ownerDNSName,
		},
	}
	if err := r.chain.Execute(ctx, rc); err != nil {
		metrics.ReconcileTotal.WithLabelValues("dnsrecord", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("dnsrecord", "").Observe(time.Since(start).Seconds())
		return ctrl.Result{}, err
	}

	if rc.Result.RequeueAfter == 0 {
		rc.Result.RequeueAfter = DNSRecordResolveInterval
	}

	originLabel := string(record.Spec.Origin)
	if originLabel == "" {
		originLabel = "external-dns"
	}
	metrics.DNSFQDNsTotal.WithLabelValues(record.Spec.PortalRef, originLabel).Set(float64(len(record.Status.Endpoints)))
	metrics.ReconcileTotal.WithLabelValues("dnsrecord", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("dnsrecord", "").Observe(time.Since(start).Seconds())

	return rc.Result, nil
}

// SetupWithManager sets up the controller with the Manager.
//
// It registers a field index on v1alpha2.DNSRecord spec.portalRef and watches
// both v1alpha1.Portal (DNS feature toggle) and v1alpha2.DNS (config changes)
// to enqueue affected DNSRecord resources.
func (r *DNSRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1alpha2.DNSRecord{},
		portalfeatures.FieldIndexPortalRef,
		func(obj client.Object) []string {
			rec, ok := obj.(*v1alpha2.DNSRecord)
			if !ok || rec.Spec.PortalRef == "" {
				return nil
			}
			return []string{rec.Spec.PortalRef}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha2.DNSRecord{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&sreportalv1alpha1.Portal{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
				portal, ok := obj.(*sreportalv1alpha1.Portal)
				if !ok || portal == nil {
					return nil
				}
				if !portal.Spec.Features.IsDNSEnabled() {
					return nil
				}
				var list v1alpha2.DNSRecordList
				if err := r.Client.List(ctx, &list,
					client.InNamespace(portal.Namespace),
					client.MatchingFields{portalfeatures.FieldIndexPortalRef: portal.Name},
				); err != nil {
					// Enqueueing the Portal key here would target a DNSRecord with
					// that name (the reconciler is registered For DNSRecord). Skip
					// instead — the next watch tick or periodic resync will retry.
					log.FromContext(ctx).Error(err, "list DNSRecord for Portal watch", "portal", portal.Name)
					return nil
				}
				reqs := make([]ctrl.Request, 0, len(list.Items))
				for i := range list.Items {
					reqs = append(reqs, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&list.Items[i])})
				}
				return reqs
			}),
			builder.WithPredicates(portalfeatures.PortalDNSEnabledWakeupPredicate()),
		).
		Watches(
			&v1alpha2.DNS{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
				dns, ok := obj.(*v1alpha2.DNS)
				if !ok || dns == nil {
					return nil
				}
				if dns.Spec.PortalRef == "" {
					return nil
				}
				var list v1alpha2.DNSRecordList
				if err := r.Client.List(ctx, &list,
					client.InNamespace(dns.Namespace),
					client.MatchingFields{portalfeatures.FieldIndexPortalRef: dns.Spec.PortalRef},
				); err != nil {
					// Same as above: DNS is not a DNSRecord — enqueueing its key
					// would resolve to a phantom DNSRecord. Skip on error.
					log.FromContext(ctx).Error(err, "list DNSRecord for DNS watch", "dns", dns.Name)
					return nil
				}
				reqs := make([]ctrl.Request, 0, len(list.Items))
				for i := range list.Items {
					reqs = append(reqs, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&list.Items[i])})
				}
				return reqs
			}),
		).
		Named("dnsrecord").
		Complete(r)
}
