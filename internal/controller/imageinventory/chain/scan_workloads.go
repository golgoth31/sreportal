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
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ScanWorkloadsHandler performs a full scan of the cluster (filtered by the
// ImageInventory spec) and populates ChainData.Observations with one entry
// per (workload, container). The aggregation into ImageRegistry CRs happens
// in SyncRegistryCRsHandler.
//
// No-op for remote inventories — those are populated by FetchRemoteImagesHandler
// directly into the readstore.
type ScanWorkloadsHandler struct {
	client client.Client
}

// NewScanWorkloadsHandler constructs a ScanWorkloadsHandler.
func NewScanWorkloadsHandler(c client.Client) *ScanWorkloadsHandler {
	return &ScanWorkloadsHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *ScanWorkloadsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource
	if inv.Spec.IsRemote {
		return nil
	}

	obs, err := h.scanAll(ctx, inv)
	if err != nil {
		metrics.ImageInventorySyncTotal.WithLabelValues(inv.Name, "error").Inc()
		wrapped := fmt.Errorf("full scan: %w", err)
		_ = statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionFalse, ReasonScanFailed, wrapped.Error())
		return wrapped
	}

	rc.Data.Observations = obs
	return nil
}

func (h *ScanWorkloadsHandler) scanAll(ctx context.Context, inv *sreportalv1alpha1.ImageInventory) ([]domainimageregistry.ContainerObservation, error) {
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

	// One pod index per scan: workloads in the same namespace share a single
	// List call. Without this, a 500-workload scan triggers 500 pod LISTs.
	idx := newPodIndex(h.client)

	var out []domainimageregistry.ContainerObservation
	for _, kind := range inv.Spec.EffectiveWatchedKinds() {
		obs, err := h.scanKind(ctx, idx, kind, opts...)
		if err != nil {
			return nil, err
		}
		out = append(out, obs...)
	}
	return out, nil
}

func (h *ScanWorkloadsHandler) scanKind(ctx context.Context, idx *podIndex, kind sreportalv1alpha1.ImageInventoryKind, opts ...client.ListOption) ([]domainimageregistry.ContainerObservation, error) {
	kindStr := string(kind)
	logger := log.FromContext(ctx).WithValues("kind", kindStr)

	var out []domainimageregistry.ContainerObservation
	collect := func(ns, name string, spec corev1.PodSpec, podSelector *metav1.LabelSelector) {
		// Map container-name -> running-pod image (if any). When no running
		// pod is found, podImageByName is nil and we fall back to using the
		// template image as the runtime image (ChangeType=none).
		var podImageByName map[string]string
		if sel := selectorFromLabelSelector(podSelector); sel != nil {
			pod, err := idx.findNewestRunning(ctx, ns, sel)
			if err != nil {
				logger.Error(err, "find running pod failed; falling back to spec", "namespace", ns, "name", name)
			} else if pod != nil {
				podImageByName = buildContainerImageMap(pod.Spec)
			}
		}

		out = append(out, observationsFromWorkload(kindStr, ns, name, spec, podImageByName)...)
	}

	switch kind {
	case sreportalv1alpha1.ImageInventoryKindDeployment:
		var list appsv1.DeploymentList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return nil, err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.Template.Spec, it.Spec.Selector)
		}
	case sreportalv1alpha1.ImageInventoryKindStatefulSet:
		var list appsv1.StatefulSetList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return nil, err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.Template.Spec, it.Spec.Selector)
		}
	case sreportalv1alpha1.ImageInventoryKindDaemonSet:
		var list appsv1.DaemonSetList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return nil, err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.Template.Spec, it.Spec.Selector)
		}
	case sreportalv1alpha1.ImageInventoryKindCronJob:
		var list batchv1.CronJobList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return nil, err
		}
		for i := range list.Items {
			it := &list.Items[i]
			// CronJobs: pod lookup skipped — pods belong to Jobs (transient workloads).
			collect(it.Namespace, it.Name, it.Spec.JobTemplate.Spec.Template.Spec, nil)
		}
	case sreportalv1alpha1.ImageInventoryKindJob:
		var list batchv1.JobList
		if err := h.client.List(ctx, &list, opts...); err != nil {
			return nil, err
		}
		for i := range list.Items {
			it := &list.Items[i]
			collect(it.Namespace, it.Name, it.Spec.Template.Spec, it.Spec.Selector)
		}
	default:
		return nil, fmt.Errorf("unsupported kind %q", kind)
	}
	return out, nil
}

// observationsFromWorkload yields one ContainerObservation per
// (container in workload spec) plus one per injected container observed
// only in the running pod.
//
// Mapping rules:
//   - For each container declared in spec, TemplateImage = spec image and
//     PodImage = pod image (if found in podImageByName), else equal to
//     TemplateImage (degraded fallback when no running pod is available).
//   - For each container present in the pod but absent from spec
//     (injected by webhook), TemplateImage="" and PodImage=pod image.
func observationsFromWorkload(
	kind, namespace, name string,
	spec corev1.PodSpec,
	podImageByName map[string]string,
) []domainimageregistry.ContainerObservation {
	out := make([]domainimageregistry.ContainerObservation, 0, len(spec.Containers)+len(spec.InitContainers))
	declared := map[string]struct{}{}

	emitSpec := func(c corev1.Container) {
		declared[c.Name] = struct{}{}
		podImg := c.Image
		if podImageByName != nil {
			if got, ok := podImageByName[c.Name]; ok {
				podImg = got
			}
		}
		out = append(out, domainimageregistry.ContainerObservation{
			WorkloadKind:      kind,
			WorkloadName:      name,
			WorkloadNamespace: namespace,
			ContainerName:     c.Name,
			TemplateImage:     c.Image,
			PodImage:          podImg,
		})
	}
	for _, c := range spec.Containers {
		emitSpec(c)
	}
	for _, c := range spec.InitContainers {
		emitSpec(c)
	}

	// Injected containers — present in pod but not in spec.
	// Stable order: iterate over a sorted list for determinism in tests.
	for cname, img := range podImageByName {
		if _, ok := declared[cname]; ok {
			continue
		}
		out = append(out, domainimageregistry.ContainerObservation{
			WorkloadKind:      kind,
			WorkloadName:      name,
			WorkloadNamespace: namespace,
			ContainerName:     cname,
			TemplateImage:     "",
			PodImage:          img,
		})
	}
	return out
}

// buildContainerImageMap returns containerName -> image for every container
// (regular and init) declared in spec.
func buildContainerImageMap(spec corev1.PodSpec) map[string]string {
	m := make(map[string]string, len(spec.Containers)+len(spec.InitContainers))
	for _, c := range spec.Containers {
		m[c.Name] = c.Image
	}
	for _, c := range spec.InitContainers {
		m[c.Name] = c.Image
	}
	return m
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
