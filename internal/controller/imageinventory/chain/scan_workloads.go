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

package chain

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	imagectrl "github.com/golgoth31/sreportal/internal/controller/image"
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ScanWorkloadsHandler performs a full scan of the cluster (filtered by the
// ImageInventory spec) and populates ChainData.ByWorkload. The actual store
// projection happens in ProjectImagesHandler so the local and remote paths
// share a single write path.
type ScanWorkloadsHandler struct {
	client client.Client
}

// NewScanWorkloadsHandler constructs a ScanWorkloadsHandler.
func NewScanWorkloadsHandler(c client.Client) *ScanWorkloadsHandler {
	return &ScanWorkloadsHandler{client: c}
}

// Handle implements reconciler.Handler.
// Scans the cluster for the inventory's watched workload kinds and stores the
// projection in ChainData. No-op for remote inventories — those are populated
// by FetchRemoteImagesHandler.
func (h *ScanWorkloadsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource
	if inv.Spec.IsRemote {
		return nil
	}

	byWorkload, err := h.scanAll(ctx, inv)
	if err != nil {
		metrics.ImageInventoryScanTotal.WithLabelValues(inv.Name, "error").Inc()
		wrapped := fmt.Errorf("full scan: %w", err)
		_ = statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionFalse, ReasonScanFailed, wrapped.Error())
		return wrapped
	}

	rc.Data.ByWorkload = byWorkload
	return nil
}

func (h *ScanWorkloadsHandler) scanAll(ctx context.Context, inv *sreportalv1alpha1.ImageInventory) (map[domainimage.WorkloadKey][]domainimage.ImageView, error) {
	selector := labels.Everything()
	if inv.Spec.LabelSelector != "" {
		parsed, err := labels.Parse(inv.Spec.LabelSelector)
		if err != nil {
			return nil, fmt.Errorf("parse labelSelector: %w", err)
		}
		selector = parsed
	}
	opts := []client.ListOption{client.MatchingLabelsSelector{Selector: selector}}
	if inv.Spec.NamespaceFilter != "" {
		opts = append(opts, client.InNamespace(inv.Spec.NamespaceFilter))
	}

	out := make(map[domainimage.WorkloadKey][]domainimage.ImageView)
	portalRef := inv.Spec.PortalRef
	for _, kind := range inv.Spec.EffectiveWatchedKinds() {
		if err := h.scanKind(ctx, portalRef, kind, out, opts...); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h *ScanWorkloadsHandler) scanKind(ctx context.Context, portalRef string, kind sreportalv1alpha1.ImageInventoryKind, out map[domainimage.WorkloadKey][]domainimage.ImageView, opts ...client.ListOption) error {
	kindStr := string(kind)
	logger := log.FromContext(ctx).WithValues("kind", kindStr, "portalRef", portalRef)

	collect := func(ns, name string, spec corev1.PodSpec, podSelector *metav1.LabelSelector) {
		wk := domainimage.WorkloadKey{Kind: kindStr, Namespace: ns, Name: name}
		views := imagectrl.ImageViewsFromPodSpec(portalRef, kindStr, ns, name, spec, domainimage.ContainerSourceSpec)

		if sel := selectorFromLabelSelector(podSelector); sel != nil {
			pod, err := imagectrl.FindRunningPodForWorkload(ctx, h.client, ns, sel)
			if err != nil {
				logger.Error(err, "find running pod failed; falling back to spec", "workload", wk)
			} else if pod != nil {
				views = append(views, imagectrl.ImageViewsFromPodDiff(portalRef, kindStr, ns, name, spec, pod.Spec)...)
			}
		}
		out[wk] = views
	}

	switch kind {
	case sreportalv1alpha1.ImageInventoryKindDeployment:
		var list appsv1.DeploymentList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.Template.Spec, it.Spec.Selector)
		}
	case sreportalv1alpha1.ImageInventoryKindStatefulSet:
		var list appsv1.StatefulSetList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.Template.Spec, it.Spec.Selector)
		}
	case sreportalv1alpha1.ImageInventoryKindDaemonSet:
		var list appsv1.DaemonSetList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.Template.Spec, it.Spec.Selector)
		}
	case sreportalv1alpha1.ImageInventoryKindCronJob:
		var list batchv1.CronJobList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return err
		}
		for i := range list.Items {
			it := &list.Items[i]
			// CronJobs: pod lookup skipped — pods belong to Jobs (transient workloads).
			collect(it.Namespace, it.Name, it.Spec.JobTemplate.Spec.Template.Spec, nil)
		}
	case sreportalv1alpha1.ImageInventoryKindJob:
		var list batchv1.JobList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.Template.Spec, it.Spec.Selector)
		}
	default:
		return fmt.Errorf("unsupported kind %q", kind)
	}
	return nil
}

// selectorFromLabelSelector converts a *metav1.LabelSelector to a labels.Selector,
// returning nil for empty/invalid selectors so the caller skips pod lookup
// rather than matching every pod in the namespace.
func selectorFromLabelSelector(s *metav1.LabelSelector) labels.Selector {
	if s == nil {
		return nil
	}
	if len(s.MatchLabels) == 0 && len(s.MatchExpressions) == 0 {
		return nil
	}
	sel, err := metav1.LabelSelectorAsSelector(s)
	if err != nil {
		return nil
	}
	return sel
}
