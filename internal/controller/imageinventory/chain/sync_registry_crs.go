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
	"errors"
	"fmt"
	"sort"

	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/metrics"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// syncRegistryCRsConcurrency caps in-flight upsert/delete operations during
// SyncRegistryCRs. Each ImageInventory typically owns dozens of child
// ImageRegistry CRs (one per host×namespace); running them sequentially
// serialized API round-trips and could push the handler beyond 5s.
const syncRegistryCRsConcurrency = 8

// SyncRegistryCRsHandler aggregates ChainData.Observations by (host, namespace)
// and reconciles the corresponding child ImageRegistry CRs (create/patch/delete)
// owned by the parent ImageInventory.
//
// No-op for remote inventories — those write directly to the readstore via
// FetchRemoteImagesHandler and don't own ImageRegistry CRs (those belong to the
// source portal's controller).
type SyncRegistryCRsHandler struct {
	client client.Client
}

// NewSyncRegistryCRsHandler constructs a SyncRegistryCRsHandler.
func NewSyncRegistryCRsHandler(c client.Client) *SyncRegistryCRsHandler {
	return &SyncRegistryCRsHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *SyncRegistryCRsHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource
	if inv.Spec.IsRemote {
		return nil
	}

	groups := domainimageregistry.AggregateForCRs(inv.Spec.PortalRef, rc.Data.Observations)

	// Stable iteration: sort group keys. desired is computed up front from
	// the deterministic (host, namespace) tuples so the parallel upsert loop
	// below has nothing to write back into shared state.
	gKeys := make([]domainimageregistry.Group, 0, len(groups))
	for g := range groups {
		gKeys = append(gKeys, g)
	}
	sort.SliceStable(gKeys, func(i, j int) bool {
		if gKeys[i].Host != gKeys[j].Host {
			return gKeys[i].Host < gKeys[j].Host
		}
		return gKeys[i].Namespace < gKeys[j].Namespace
	})
	desired := make(map[string]sreportalv1alpha1.ImageRegistryRef, len(gKeys))
	for _, g := range gKeys {
		hash := domainimageregistry.CRName(inv.Spec.PortalRef, g.Host, g.Namespace)
		desired[hash] = sreportalv1alpha1.ImageRegistryRef{
			Hash:      hash,
			Host:      g.Host,
			Namespace: g.Namespace,
		}
	}

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(syncRegistryCRsConcurrency)
	for _, g := range gKeys {
		entries := groups[g]
		ref := desired[domainimageregistry.CRName(inv.Spec.PortalRef, g.Host, g.Namespace)]
		eg.Go(func() error {
			if err := h.upsertCR(egCtx, inv, ref.Hash, g, entries); err != nil {
				// A peer goroutine already failed and canceled egCtx; let
				// the original error propagate instead of masking it with
				// our own context.Canceled.
				if errors.Is(err, context.Canceled) && ctx.Err() == nil {
					return nil
				}
				return fmt.Errorf("upsert ImageRegistry %s: %w", ref.Hash, err)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		metrics.ImageInventorySyncTotal.WithLabelValues(inv.Name, "error").Inc()
		_ = statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionFalse, ReasonProjectionFailed, err.Error())
		return err
	}

	// Garbage-collect child ImageRegistry CRs that were previously owned by
	// this ImageInventory but no longer match any group in the latest scan.
	if err := h.deleteOrphans(ctx, inv, desired); err != nil {
		metrics.ImageInventorySyncTotal.WithLabelValues(inv.Name, "error").Inc()
		wrapped := fmt.Errorf("delete orphan ImageRegistry CRs: %w", err)
		_ = statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionFalse, ReasonProjectionFailed, wrapped.Error())
		return wrapped
	}

	// Project desired refs into the parent's Status.Registries (deterministic order).
	refs := make([]sreportalv1alpha1.ImageRegistryRef, 0, len(desired))
	for _, ref := range desired {
		refs = append(refs, ref)
	}
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].Hash < refs[j].Hash })
	base := inv.DeepCopy()
	inv.Status.Registries = refs
	if err := h.client.Status().Patch(ctx, inv, client.MergeFrom(base)); err != nil {
		metrics.ImageInventorySyncTotal.WithLabelValues(inv.Name, "error").Inc()
		wrapped := fmt.Errorf("patch Status.Registries: %w", err)
		_ = statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionFalse, ReasonProjectionFailed, wrapped.Error())
		return wrapped
	}

	metrics.ImageInventorySyncTotal.WithLabelValues(inv.Name, "success").Inc()
	return nil
}

// upsertCR creates or patches a single ImageRegistry CR owned by `inv`.
//
// The CR is created in the same namespace as the parent ImageInventory so
// ownerRef-driven garbage collection works (Kubernetes requires owner and
// dependent to be in the same namespace for namespaced owner refs).
func (h *SyncRegistryCRsHandler) upsertCR(
	ctx context.Context,
	inv *sreportalv1alpha1.ImageInventory,
	hash string,
	group domainimageregistry.Group,
	entries []domainimageregistry.Entry,
) error {
	logger := log.FromContext(ctx).WithValues("imageregistry", hash, "host", group.Host, "namespace", group.Namespace)

	cr := &sreportalv1alpha1.ImageRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hash,
			Namespace: inv.Namespace,
		},
	}
	op, err := controllerutil.CreateOrPatch(ctx, h.client, cr, func() error {
		// Owner reference for cascading delete.
		if err := controllerutil.SetControllerReference(inv, cr, h.client.Scheme()); err != nil {
			return fmt.Errorf("set owner reference: %w", err)
		}
		cr.Spec.Host = group.Host
		cr.Spec.PortalRef = inv.Spec.PortalRef
		cr.Spec.Namespace = group.Namespace
		cr.Spec.Images = entriesToSpecImages(entries)
		return nil
	})
	if err != nil {
		return err
	}
	logger.V(1).Info("ImageRegistry reconciled", "operation", op, "imageCount", len(entries))
	return nil
}

// deleteOrphans deletes every ImageRegistry CR previously listed in
// inv.Status.Registries that is no longer in `desired`. We rely on the parent
// status (rather than listing children by ownerRef) so the controller works
// without a field/label index — at the cost of needing the status to be in
// sync, which it is, since we always update it on success.
//
// Deletes run concurrently and the pre-flight Get is skipped: Delete already
// returns IsNotFound, which we treat as a no-op.
func (h *SyncRegistryCRsHandler) deleteOrphans(
	ctx context.Context,
	inv *sreportalv1alpha1.ImageInventory,
	desired map[string]sreportalv1alpha1.ImageRegistryRef,
) error {
	logger := log.FromContext(ctx)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(syncRegistryCRsConcurrency)
	for _, prev := range inv.Status.Registries {
		if _, keep := desired[prev.Hash]; keep {
			continue
		}
		eg.Go(func() error {
			cr := &sreportalv1alpha1.ImageRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prev.Hash,
					Namespace: inv.Namespace,
				},
			}
			if err := h.client.Delete(egCtx, cr); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				if errors.Is(err, context.Canceled) && ctx.Err() == nil {
					return nil
				}
				return fmt.Errorf("delete orphan ImageRegistry %s: %w", prev.Hash, err)
			}
			logger.V(1).Info("deleted orphan ImageRegistry", "name", prev.Hash)
			return nil
		})
	}
	return eg.Wait()
}

// entriesToSpecImages converts the domain Entry slice into the API
// ImageRegistrySpecEntry slice consumed by the CRD.
func entriesToSpecImages(entries []domainimageregistry.Entry) []sreportalv1alpha1.ImageRegistrySpecEntry {
	out := make([]sreportalv1alpha1.ImageRegistrySpecEntry, 0, len(entries))
	for _, e := range entries {
		workloads := make([]sreportalv1alpha1.ImageRegistryWorkloadRef, 0, len(e.Workloads))
		for _, w := range e.Workloads {
			workloads = append(workloads, sreportalv1alpha1.ImageRegistryWorkloadRef{
				Kind:      w.Kind,
				Namespace: w.Namespace,
				Name:      w.Name,
				Container: w.Container,
			})
		}
		out = append(out, sreportalv1alpha1.ImageRegistrySpecEntry{
			Key:           e.Key,
			OriginalImage: e.OriginalImage,
			MutatedImage:  e.MutatedImage,
			ChangeType:    string(e.ChangeType),
			Repository:    e.Repository,
			OriginalTag:   e.OriginalTag,
			TagType:       string(e.TagType),
			Workloads:     workloads,
		})
	}
	return out
}
