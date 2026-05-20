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

package dns

import (
	"context"
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	dnschain "github.com/golgoth31/sreportal/internal/controller/dns/chain"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	domainsource "github.com/golgoth31/sreportal/internal/domain/source"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	// DefaultRequeueAfter is the fallback requeue interval when the DNS CR
	// has no explicit reconciliation.interval set.
	DefaultRequeueAfter = 5 * time.Minute
	// MinRequeueAfter caps the lower bound of spec.reconciliation.interval to
	// avoid hot-looping the controller.
	MinRequeueAfter = 30 * time.Second
)

// DNSReconciler reconciles a DNS object
type DNSReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	SourceReader domainsource.SourceEndpointReader
	Conflicts    domaindns.FQDNConflictReader
	chain        *reconciler.Chain[*v1alpha2.DNS, dnschain.ChainData]
}

// NewDNSReconciler creates a new DNSReconciler with the handler chain.
func NewDNSReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	sourceReader domainsource.SourceEndpointReader,
	conflicts domaindns.FQDNConflictReader,
) *DNSReconciler {
	r := &DNSReconciler{
		Client:       c,
		Scheme:       scheme,
		SourceReader: sourceReader,
		Conflicts:    conflicts,
	}
	r.chain = reconciler.NewChain[*v1alpha2.DNS, dnschain.ChainData](
		"dns",
		&dnschain.LookupSourcesHandler{Source: sourceReader},
		&dnschain.IntraDNSDedupHandler{},
		&dnschain.UpsertDNSRecordsHandler{Client: c},
		&dnschain.SourcesStatusHandler{Conflicts: conflicts},
	)
	return r
}

// +kubebuilder:rbac:groups=sreportal.io,resources=dns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=dns/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=dns/finalizers,verbs=update
// +kubebuilder:rbac:groups=sreportal.io,resources=dnsrecords,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=dnsrecords/finalizers,verbs=update
// +kubebuilder:rbac:groups=sreportal.io,resources=portals,verbs=get;list;watch
// +kubebuilder:rbac:groups=sreportal.io,resources=components,verbs=get;list;watch;create;update;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DNSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := log.FromContext(ctx)

	var resource v1alpha2.DNS
	if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Remote DNS CRs are managed by the portal controller — skip reconciliation.
	if resource.Spec.IsRemote {
		logger.V(1).Info("skipping remote DNS resource (managed by portal controller)", "name", resource.Name)
		return ctrl.Result{}, nil
	}

	logger.Info("reconciling DNS resource", "name", resource.Name, "namespace", resource.Namespace)

	rc := &reconciler.ReconcileContext[*v1alpha2.DNS, dnschain.ChainData]{
		Resource: &resource,
	}

	if err := r.chain.Execute(ctx, rc); err != nil {
		logger.Error(err, "reconciliation failed")
		// Surface the chain failure on SourcesReady so the DNS CR no longer
		// advertises a stale True condition while the controller is broken.
		// Best-effort: ignore the patch error (we'll already return the chain
		// error and re-run).
		dnschain.SetCondition(&resource, metav1.Condition{
			Type:    "SourcesReady",
			Status:  metav1.ConditionFalse,
			Reason:  "ReconcileFailed",
			Message: err.Error(),
		})
		if patchErr := r.Status().Update(ctx, &resource); patchErr != nil {
			logger.V(1).Info("failed to persist SourcesReady=False after chain error", "patchError", patchErr)
		}
		metrics.ReconcileTotal.WithLabelValues("dns", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("dns", "").Observe(time.Since(start).Seconds())
		return ctrl.Result{}, err
	}

	now := metav1.Now()
	resource.Status.LastReconcileTime = &now
	resource.Status.ObservedGeneration = resource.Generation
	next := metav1.NewTime(now.Add(requeueInterval(&resource)))
	resource.Status.NextReconcileTime = &next

	// Persist any status updates accumulated by SourcesStatusHandler + above.
	if err := r.Status().Update(ctx, &resource); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// Context was canceled or timed out (shutdown / re-queue race): skip silently.
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to update DNS status")
		metrics.ReconcileTotal.WithLabelValues("dns", "error").Inc()
		metrics.ReconcileDuration.WithLabelValues("dns", "").Observe(time.Since(start).Seconds())
		return ctrl.Result{}, err
	}

	if rc.Result.RequeueAfter == 0 {
		rc.Result.RequeueAfter = requeueInterval(&resource)
	}

	metrics.ReconcileTotal.WithLabelValues("dns", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("dns", "").Observe(time.Since(start).Seconds())

	logger.Info("reconciliation completed", "requeueAfter", rc.Result.RequeueAfter)
	return rc.Result, nil
}

// requeueInterval returns the per-DNS requeue duration, falling back to
// DefaultRequeueAfter when unset and clamping anything below MinRequeueAfter.
func requeueInterval(dns *v1alpha2.DNS) time.Duration {
	d := dns.Spec.Reconciliation.Interval.Duration
	if d <= 0 {
		return DefaultRequeueAfter
	}
	if d < MinRequeueAfter {
		return MinRequeueAfter
	}
	return d
}

// SetupWithManager sets up the controller with the Manager.
func (r *DNSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha2.DNS{}).
		Watches(
			&v1alpha2.DNSRecord{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueDNSForRecord),
			// Ignore status-only patches (e.g. syncStatus, endpointsHash) — they
			// don't affect DNS reconciliation. Spec changes (generation bump) from
			// UpsertDNSRecordsHandler will still trigger re-reconciliation correctly.
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&sreportalv1alpha1.Portal{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueDNSForPortal),
		).
		Named("dns").
		Complete(r)
}

// enqueueDNSForRecord enqueues the owning DNS for a DNSRecord change. The
// DNSRecord webhook enforces a controller ownerRef to a DNS CR, so reading
// ownerRefs is sufficient and avoids a name-based fallback.
func (r *DNSReconciler) enqueueDNSForRecord(_ context.Context, obj client.Object) []ctrl.Request {
	dr, ok := obj.(*v1alpha2.DNSRecord)
	if !ok {
		return nil
	}
	for _, ref := range dr.GetOwnerReferences() {
		if ref.Controller != nil && *ref.Controller && ref.Kind == "DNS" {
			return []ctrl.Request{{NamespacedName: types.NamespacedName{Namespace: dr.Namespace, Name: ref.Name}}}
		}
	}
	return nil
}

// enqueueDNSForPortal enqueues every DNS in the Portal's namespace that
// references it via spec.portalRef.
func (r *DNSReconciler) enqueueDNSForPortal(ctx context.Context, obj client.Object) []ctrl.Request {
	portal, ok := obj.(*sreportalv1alpha1.Portal)
	if !ok {
		return nil
	}
	var list v1alpha2.DNSList
	if err := r.List(ctx, &list, client.InNamespace(portal.Namespace)); err != nil {
		log.FromContext(ctx).Error(err, "list DNS for Portal watch", "portal", portal.Name)
		return nil
	}
	reqs := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		if list.Items[i].Spec.PortalRef == portal.Name {
			reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: list.Items[i].Namespace, Name: list.Items[i].Name,
			}})
		}
	}
	return reqs
}
