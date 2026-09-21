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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	domainimageregistry "github.com/golgoth31/sreportal/internal/domain/imageregistry"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// OCI label keys used to resolve an image to its git source.
const (
	ociLabelSource   = "org.opencontainers.image.source"
	ociLabelRevision = "org.opencontainers.image.revision"
)

// ImageLabelReader reads an image's OCI config labels. Implemented by
// *registry.CraneClient; faked in tests.
type ImageLabelReader interface {
	ImageConfigLabels(ctx context.Context, host, repository, reference string) (map[string]string, error)
}

// ProjectDeployStatusHandler projects the local-path image observations into
// per-(portal, namespace) DeployStatus CRs. For each observed workload-image
// that carries an org.opencontainers.image.source label (first-party services
// only), it emits one DeployStatusEntry into Spec.Services. Base images (no
// source label) are skipped so the CR carries no noise.
//
// Only Spec is written — State and all Status fields are observed and owned by
// the DeployStatus controller.
//
// No-op for:
//   - remote inventories (Observations is empty there);
//   - portals with the deployStatus feature disabled.
type ProjectDeployStatusHandler struct {
	client client.Client
	labels ImageLabelReader
}

// NewProjectDeployStatusHandler constructs a ProjectDeployStatusHandler.
func NewProjectDeployStatusHandler(c client.Client, labels ImageLabelReader) *ProjectDeployStatusHandler {
	return &ProjectDeployStatusHandler{client: c, labels: labels}
}

// Handle implements reconciler.Handler.
func (h *ProjectDeployStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource
	if inv.Spec.IsRemote {
		return nil
	}

	// Feature gate: skip projection when the portal disables deployStatus.
	var portal sreportalv1alpha1.Portal
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: inv.Namespace, Name: inv.Spec.PortalRef}, &portal); err != nil {
		if apierrors.IsNotFound(err) {
			// ValidatePortalRefHandler already short-circuits this case; defensive no-op.
			return nil
		}
		return fmt.Errorf("get portal for deploy-status gate: %w", err)
	}
	if !portal.Spec.Features.IsDeployStatusEnabled() {
		return nil
	}

	// Group source-labeled entries by observed namespace — one DeployStatus CR
	// per (portalRef, namespace), mirroring sync_registry_crs.go's per-group upsert.
	byNamespace, carryForwardKeys := h.buildEntries(ctx, rc.Data.Observations)

	for ns, entries := range byNamespace {
		if err := h.upsertCR(ctx, inv, ns, entries, carryForwardKeys[ns]); err != nil {
			return fmt.Errorf("upsert DeployStatus for namespace %s: %w", ns, err)
		}
	}

	// Garbage-collect DeployStatus CRs previously owned by this ImageInventory
	// whose namespace no longer carries any first-party image this cycle. Without
	// this, a vanished workload's stale lag would be shown indefinitely.
	if err := h.deleteOrphans(ctx, inv, byNamespace); err != nil {
		return fmt.Errorf("delete orphan DeployStatus CRs: %w", err)
	}
	return nil
}

// buildEntries resolves each observation's OCI labels and returns the
// source-labeled DeployStatusEntries grouped by namespace. Entries with no
// source label are dropped (legit base images).
//
// It also returns, per namespace, the set of entry keys whose OCI-label read
// FAILED. A transient registry error must not silently drop a workload from the
// CR (which would be indistinguishable from "not first-party"): the caller
// carries forward the prior entry for these keys. Namespaces that have only
// errored keys are still registered in the returned map (with no fresh entries)
// so they are neither pruned nor lose their carried-forward entries.
func (h *ProjectDeployStatusHandler) buildEntries(
	ctx context.Context,
	obs []domainimageregistry.ContainerObservation,
) (map[string][]sreportalv1alpha1.DeployStatusEntry, map[string]map[string]struct{}) {
	logger := log.FromContext(ctx)
	out := map[string][]sreportalv1alpha1.DeployStatusEntry{}
	carryForward := map[string]map[string]struct{}{}

	for _, o := range obs {
		image := o.PodImage
		if image == "" {
			continue
		}
		parsed, err := domainimage.ParseReference(image)
		if err != nil {
			logger.V(1).Info("skipping unparseable image for deploy-status", "image", image, "error", err.Error())
			continue
		}

		labels, err := h.labels.ImageConfigLabels(ctx, parsed.Registry, parsed.Repository, parsed.Tag)
		if err != nil {
			// A transient read error must NOT change CR membership. Log at Error
			// (default-visible) and mark the key for carry-forward so an existing
			// entry survives this cycle instead of vanishing.
			key := deployStatusEntryKey(image, o.WorkloadKind, o.WorkloadNamespace, o.WorkloadName, o.ContainerName)
			logger.Error(err, "failed to read OCI labels for deploy-status; carrying forward any prior entry",
				"image", image, "namespace", o.WorkloadNamespace, "workload", o.WorkloadName, "key", key)
			if carryForward[o.WorkloadNamespace] == nil {
				carryForward[o.WorkloadNamespace] = map[string]struct{}{}
			}
			carryForward[o.WorkloadNamespace][key] = struct{}{}
			// Register the namespace so it is not pruned and the carry-forward
			// merge runs even when no fresh entry exists for it this cycle.
			if _, ok := out[o.WorkloadNamespace]; !ok {
				out[o.WorkloadNamespace] = nil
			}
			continue
		}
		source := labels[ociLabelSource]
		if source == "" {
			// Base image / no first-party source — skip (no noise entries).
			continue
		}

		entry := sreportalv1alpha1.DeployStatusEntry{
			Key:   deployStatusEntryKey(image, o.WorkloadKind, o.WorkloadNamespace, o.WorkloadName, o.ContainerName),
			Image: image,
			Workload: sreportalv1alpha1.DeployStatusWorkloadRef{
				Kind:      o.WorkloadKind,
				Namespace: o.WorkloadNamespace,
				Name:      o.WorkloadName,
				Container: o.ContainerName,
			},
			SourceRepo:  source,
			DeployedRef: deployedRef(labels, parsed),
		}
		out[o.WorkloadNamespace] = append(out[o.WorkloadNamespace], entry)
	}
	return out, carryForward
}

// upsertCR creates or patches the per-(portal, namespace) DeployStatus CR,
// owned by the parent ImageInventory so GC works. Only Spec is written.
//
// carryForwardKeys are entry keys whose OCI-label read failed this cycle: for
// each, if the existing CR already carries an entry, it is preserved so a
// transient registry error does not drop the workload from the CR.
func (h *ProjectDeployStatusHandler) upsertCR(
	ctx context.Context,
	inv *sreportalv1alpha1.ImageInventory,
	namespace string,
	entries []sreportalv1alpha1.DeployStatusEntry,
	carryForwardKeys map[string]struct{},
) error {
	logger := log.FromContext(ctx)

	name := domainimageregistry.DeployStatusCRName(inv.Spec.PortalRef, namespace)
	cr := &sreportalv1alpha1.DeployStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: inv.Namespace,
		},
	}
	op, err := controllerutil.CreateOrUpdate(ctx, h.client, cr, func() error {
		if err := controllerutil.SetControllerReference(inv, cr, h.client.Scheme()); err != nil {
			return fmt.Errorf("set owner reference: %w", err)
		}

		// Carry forward prior entries for keys whose label read failed this cycle.
		// cr.Spec.Services here holds the previously persisted spec (CreateOrUpdate
		// fetched the object before invoking this mutate fn).
		merged := append([]sreportalv1alpha1.DeployStatusEntry(nil), entries...)
		if len(carryForwardKeys) > 0 {
			fresh := make(map[string]struct{}, len(entries))
			for _, e := range entries {
				fresh[e.Key] = struct{}{}
			}
			for _, prev := range cr.Spec.Services {
				if _, carry := carryForwardKeys[prev.Key]; !carry {
					continue
				}
				if _, alreadyFresh := fresh[prev.Key]; alreadyFresh {
					continue
				}
				merged = append(merged, prev)
			}
		}

		// Deterministic ordering so repeated reconciles produce identical specs.
		sort.SliceStable(merged, func(i, j int) bool { return merged[i].Key < merged[j].Key })

		cr.Spec.PortalRef = inv.Spec.PortalRef
		cr.Spec.Namespace = namespace
		cr.Spec.Services = merged
		return nil
	})
	if err != nil {
		return err
	}
	logger.V(1).Info("DeployStatus reconciled", "operation", op, "name", name, "serviceCount", len(cr.Spec.Services))
	return nil
}

// deleteOrphans deletes DeployStatus CRs owned by this ImageInventory whose
// namespace no longer carries any first-party image this cycle. Ownership is
// checked via the controller owner reference, mirroring the delete-vs-empty
// choice in sync_registry_crs.go (delete the whole CR).
func (h *ProjectDeployStatusHandler) deleteOrphans(
	ctx context.Context,
	inv *sreportalv1alpha1.ImageInventory,
	byNamespace map[string][]sreportalv1alpha1.DeployStatusEntry,
) error {
	logger := log.FromContext(ctx)

	var list sreportalv1alpha1.DeployStatusList
	if err := h.client.List(ctx, &list, client.InNamespace(inv.Namespace)); err != nil {
		return fmt.Errorf("list DeployStatus CRs: %w", err)
	}

	for i := range list.Items {
		cr := &list.Items[i]
		if !metav1.IsControlledBy(cr, inv) {
			continue
		}
		if _, keep := byNamespace[cr.Spec.Namespace]; keep {
			continue
		}
		if err := h.client.Delete(ctx, cr); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("delete orphan DeployStatus %s: %w", cr.Name, err)
		}
		logger.V(1).Info("deleted orphan DeployStatus", "name", cr.Name, "namespace", cr.Spec.Namespace)
	}
	return nil
}

// deployedRef returns the deployed git ref: the OCI revision label if present,
// else the image tag when it is a semver tag (fallback), else empty.
func deployedRef(labels map[string]string, parsed domainimage.ParsedReference) string {
	if rev := labels[ociLabelRevision]; rev != "" {
		return rev
	}
	if parsed.TagType == domainimage.TagTypeSemver {
		return parsed.Tag
	}
	return ""
}

// deployStatusEntryKey is sha256(image|workloadKind|workloadNamespace|workloadName|container)[:16].
func deployStatusEntryKey(image, kind, namespace, name, container string) string {
	sum := sha256.Sum256([]byte(image + "|" + kind + "|" + namespace + "|" + name + "|" + container))
	return hex.EncodeToString(sum[:])[:16]
}
