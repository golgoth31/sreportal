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
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	domaindns "github.com/golgoth31/sreportal/internal/domain/dns"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
)

// DNSRecordReconciler reconciles a DNSRecord object and projects its endpoints
// directly into the FQDN read store.
type DNSRecordReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	fqdnWriter      domaindns.FQDNWriter
	groupMapping    *config.GroupMappingConfig
	resolver        domaindns.Resolver
	disableDNSCheck bool
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
	return &DNSRecordReconciler{
		Client:          c,
		Scheme:          scheme,
		groupMapping:    groupMapping,
		resolver:        resolver,
		disableDNSCheck: disableDNSCheck,
	}
}

// SetFQDNWriter sets the FQDN read-store writer.
func (r *DNSRecordReconciler) SetFQDNWriter(w domaindns.FQDNWriter) {
	r.fqdnWriter = w
}

// Reconcile resolves DNS for each endpoint, persists SyncStatus in the CR,
// and projects endpoints into the FQDN read store.
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

	// Resolve DNS and persist SyncStatus in the CR status
	if !r.disableDNSCheck && r.resolver != nil && len(record.Status.Endpoints) > 0 {
		base := record.DeepCopy()
		r.resolveEndpoints(ctx, record.Status.Endpoints)
		if err := r.Status().Patch(ctx, &record, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, fmt.Errorf("patch DNSRecord status: %w", err)
		}
	}

	// Project into read store
	if r.fqdnWriter != nil {
		resourceKey := record.Namespace + "/" + record.Name
		views := dnsRecordToFQDNViews(&record, r.groupMapping)
		if err := r.fqdnWriter.Replace(ctx, resourceKey, views); err != nil {
			logger.Error(err, "failed to update FQDN read store")
		}
	}

	// Update metrics
	portal := record.Spec.PortalRef
	metrics.DNSFQDNsTotal.WithLabelValues(portal, "external-dns").Set(float64(len(record.Status.Endpoints)))
	metrics.ReconcileTotal.WithLabelValues("dnsrecord", "success").Inc()
	metrics.ReconcileDuration.WithLabelValues("dnsrecord").Observe(time.Since(start).Seconds())

	return ctrl.Result{}, nil
}

const (
	maxDNSRecordLookups    = 10
	dnsRecordLookupTimeout = 5 * time.Second
)

// resolveEndpoints resolves DNS for each endpoint in-place, setting SyncStatus.
func (r *DNSRecordReconciler) resolveEndpoints(ctx context.Context, endpoints []sreportalv1alpha1.EndpointStatus) {
	workers := min(maxDNSRecordLookups, len(endpoints))
	ch := make(chan int, len(endpoints))
	for i := range endpoints {
		ch <- i
	}
	close(ch)

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for idx := range ch {
				ep := &endpoints[idx]
				lookupCtx, cancel := context.WithTimeout(ctx, dnsRecordLookupTimeout)
				result := domaindns.CheckFQDN(lookupCtx, r.resolver, ep.DNSName, ep.RecordType, ep.Targets)
				ep.SyncStatus = string(result.Status)
				cancel()
			}
		})
	}

	wg.Wait()
}

// SetupWithManager sets up the controller with the Manager.
func (r *DNSRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.DNSRecord{}).
		Named("dnsrecord").
		Complete(r)
}
