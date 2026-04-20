package image

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/log"
)

var _ manager.Runnable = (*Scanner)(nil)

// Scanner periodically computes image inventory projections from workloads.
type Scanner struct {
	client      client.Client
	writer      domainimage.ImageWriter
	interval    time.Duration
	lastPortals map[string]struct{}
}

func NewScanner(c client.Client, writer domainimage.ImageWriter, interval time.Duration) *Scanner {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &Scanner{
		client:      c,
		writer:      writer,
		interval:    interval,
		lastPortals: map[string]struct{}{},
	}
}

func (s *Scanner) Start(ctx context.Context) error {
	logger := log.Default().WithName("image-scanner")
	if err := s.scan(ctx); err != nil {
		logger.Error(err, "initial scan failed")
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.scan(ctx); err != nil {
				logger.Error(err, "periodic scan failed")
			}
		}
	}
}

func (s *Scanner) scan(ctx context.Context) error {
	var list sreportalv1alpha1.ImageInventoryList
	if err := s.client.List(ctx, &list); err != nil {
		return fmt.Errorf("list image inventories: %w", err)
	}

	perPortal := make(map[string][]domainimage.ImageView)
	seen := map[string]struct{}{}

	for i := range list.Items {
		inv := &list.Items[i]
		views, err := s.scanInventory(ctx, inv)
		if err != nil {
			log.FromContext(ctx).Error(err, "scan inventory failed", "namespace", inv.Namespace, "name", inv.Name)
			continue
		}
		key := inv.Spec.PortalRef
		seen[key] = struct{}{}
		perPortal[key] = append(perPortal[key], views...)
	}

	for portalRef, images := range perPortal {
		if err := s.writer.Replace(ctx, portalRef, images); err != nil {
			return fmt.Errorf("replace portal %s images: %w", portalRef, err)
		}
	}
	for portalRef := range s.lastPortals {
		if _, ok := seen[portalRef]; ok {
			continue
		}
		if err := s.writer.Delete(ctx, portalRef); err != nil {
			return fmt.Errorf("delete portal %s images: %w", portalRef, err)
		}
	}
	s.lastPortals = seen
	return nil
}

func (s *Scanner) scanInventory(ctx context.Context, inv *sreportalv1alpha1.ImageInventory) ([]domainimage.ImageView, error) {
	selector := labels.Everything()
	if inv.Spec.LabelSelector != "" {
		parsed, err := labels.Parse(inv.Spec.LabelSelector)
		if err != nil {
			return nil, fmt.Errorf("parse labelSelector: %w", err)
		}
		selector = parsed
	}

	opts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: selector},
	}
	if inv.Spec.NamespaceFilter != "" {
		opts = append(opts, client.InNamespace(inv.Spec.NamespaceFilter))
	}

	var out []domainimage.ImageView
	for _, kind := range inv.Spec.EffectiveWatchedKinds() {
		views, err := s.scanKind(ctx, inv.Spec.PortalRef, kind, opts...)
		if err != nil {
			return nil, err
		}
		out = append(out, views...)
	}
	return out, nil
}

func (s *Scanner) scanKind(ctx context.Context, portalRef string, kind sreportalv1alpha1.ImageInventoryKind, opts ...client.ListOption) ([]domainimage.ImageView, error) {
	kindStr := string(kind)
	switch kind {
	case sreportalv1alpha1.ImageInventoryKindDeployment:
		var list appsv1.DeploymentList
		if err := s.client.List(ctx, &list, opts...); err != nil {
			return nil, err
		}
		return fromPodSpecList(portalRef, kindStr, list.Items, func(d appsv1.Deployment) (string, string, corev1.PodSpec) {
			return d.Namespace, d.Name, d.Spec.Template.Spec
		}), nil
	case sreportalv1alpha1.ImageInventoryKindStatefulSet:
		var list appsv1.StatefulSetList
		if err := s.client.List(ctx, &list, opts...); err != nil {
			return nil, err
		}
		return fromPodSpecList(portalRef, kindStr, list.Items, func(d appsv1.StatefulSet) (string, string, corev1.PodSpec) {
			return d.Namespace, d.Name, d.Spec.Template.Spec
		}), nil
	case sreportalv1alpha1.ImageInventoryKindDaemonSet:
		var list appsv1.DaemonSetList
		if err := s.client.List(ctx, &list, opts...); err != nil {
			return nil, err
		}
		return fromPodSpecList(portalRef, kindStr, list.Items, func(d appsv1.DaemonSet) (string, string, corev1.PodSpec) {
			return d.Namespace, d.Name, d.Spec.Template.Spec
		}), nil
	case sreportalv1alpha1.ImageInventoryKindCronJob:
		var list batchv1.CronJobList
		if err := s.client.List(ctx, &list, opts...); err != nil {
			return nil, err
		}
		return fromPodSpecList(portalRef, kindStr, list.Items, func(d batchv1.CronJob) (string, string, corev1.PodSpec) {
			return d.Namespace, d.Name, d.Spec.JobTemplate.Spec.Template.Spec
		}), nil
	case sreportalv1alpha1.ImageInventoryKindJob:
		var list batchv1.JobList
		if err := s.client.List(ctx, &list, opts...); err != nil {
			return nil, err
		}
		return fromPodSpecList(portalRef, kindStr, list.Items, func(d batchv1.Job) (string, string, corev1.PodSpec) {
			return d.Namespace, d.Name, d.Spec.Template.Spec
		}), nil
	default:
		return nil, fmt.Errorf("unsupported kind %q", kind)
	}
}

func fromPodSpecList[T any](portalRef, kind string, items []T, get func(T) (string, string, corev1.PodSpec)) []domainimage.ImageView {
	out := make([]domainimage.ImageView, 0, len(items)*2)
	for _, it := range items {
		ns, name, spec := get(it)
		out = append(out, imageViewsFromPodSpec(portalRef, kind, ns, name, spec)...)
	}
	return out
}

func imageViewsFromPodSpec(portalRef, kind, namespace, name string, spec corev1.PodSpec) []domainimage.ImageView {
	out := make([]domainimage.ImageView, 0, len(spec.Containers)+len(spec.InitContainers))
	appendContainer := func(c corev1.Container) {
		ref, err := domainimage.ParseReference(c.Image)
		if err != nil {
			return
		}
		out = append(out, domainimage.ImageView{
			PortalRef:  portalRef,
			Registry:   ref.Registry,
			Repository: ref.Repository,
			Tag:        ref.Tag,
			TagType:    ref.TagType,
			Workloads: []domainimage.WorkloadRef{{
				Kind:      kind,
				Namespace: namespace,
				Name:      name,
				Container: c.Name,
			}},
		})
	}
	for _, c := range spec.Containers {
		appendContainer(c)
	}
	for _, c := range spec.InitContainers {
		appendContainer(c)
	}
	return out
}
