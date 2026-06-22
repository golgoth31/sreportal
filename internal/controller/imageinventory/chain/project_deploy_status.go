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
	byNamespace := h.buildEntries(ctx, inv.Spec.PortalRef, rc.Data.Observations)

	for ns, entries := range byNamespace {
		if err := h.upsertCR(ctx, inv, ns, entries); err != nil {
			return fmt.Errorf("upsert DeployStatus for namespace %s: %w", ns, err)
		}
	}
	return nil
}

// buildEntries resolves each observation's OCI labels and returns the
// source-labeled DeployStatusEntries grouped by namespace. Entries with no
// source label are dropped.
func (h *ProjectDeployStatusHandler) buildEntries(
	ctx context.Context,
	portalRef string,
	obs []domainimageregistry.ContainerObservation,
) map[string][]sreportalv1alpha1.DeployStatusEntry {
	logger := log.FromContext(ctx)
	out := map[string][]sreportalv1alpha1.DeployStatusEntry{}

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
			// Best-effort: a registry hiccup must not fail the inventory chain.
			logger.V(1).Info("skipping image with unreadable OCI labels", "image", image, "error", err.Error())
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
	return out
}

// upsertCR creates or patches the per-(portal, namespace) DeployStatus CR,
// owned by the parent ImageInventory so GC works. Only Spec is written.
func (h *ProjectDeployStatusHandler) upsertCR(
	ctx context.Context,
	inv *sreportalv1alpha1.ImageInventory,
	namespace string,
	entries []sreportalv1alpha1.DeployStatusEntry,
) error {
	logger := log.FromContext(ctx)

	// Deterministic ordering so repeated reconciles produce identical specs.
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })

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
		cr.Spec.PortalRef = inv.Spec.PortalRef
		cr.Spec.Namespace = namespace
		cr.Spec.Services = entries
		return nil
	})
	if err != nil {
		return err
	}
	logger.V(1).Info("DeployStatus reconciled", "operation", op, "name", name, "serviceCount", len(entries))
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
