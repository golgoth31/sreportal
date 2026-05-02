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

// Package chain contains Chain-of-Responsibility handlers for the ImageInventory controller.
package chain

import (
	"context"
	"errors"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/controller/statusutil"
	domainimage "github.com/golgoth31/sreportal/internal/domain/image"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ReadyConditionType is the condition type used to expose reconciliation readiness.
const ReadyConditionType = "Ready"

// Reason codes used on the Ready condition.
const (
	ReasonInvalidSpec      = "InvalidSpec"
	ReasonPortalNotFound   = "PortalNotFound"
	ReasonScanFailed       = "ScanFailed"
	ReasonProjectionFailed = "ProjectionFailed"
	ReasonReconciled       = "Reconciled"
	ReconciledMessage      = "image inventory ready"
)

// ErrInvalidSpec is returned by the spec validation handler when the spec is invalid.
var ErrInvalidSpec = errors.New("invalid ImageInventory spec")

// ErrPortalNotFound is returned by the portalRef validation handler when the portal does not exist.
var ErrPortalNotFound = errors.New("portal not found")

// ChainData holds shared state between handlers.
type ChainData struct {
	// ByWorkload is the per-workload image projection for the inventory's
	// portal, populated either by ScanWorkloadsHandler (local) or by
	// FetchRemoteImagesHandler (remote), and consumed by ProjectImagesHandler.
	ByWorkload map[domainimage.WorkloadKey][]domainimage.ImageView
}

// --- Handler 1: ValidateSpec ---

// ValidateSpecHandler performs structural validation on the spec.
type ValidateSpecHandler struct {
	client client.Client
}

// NewValidateSpecHandler creates a new ValidateSpecHandler.
func NewValidateSpecHandler(c client.Client) *ValidateSpecHandler {
	return &ValidateSpecHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *ValidateSpecHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource
	if err := validateSpec(inv.Spec); err != nil {
		msg := err.Error()
		_ = statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionFalse, ReasonInvalidSpec, msg)
		return fmt.Errorf("%w: %s", ErrInvalidSpec, msg)
	}
	return nil
}

// --- Handler 2: ValidatePortalRef ---

// ValidatePortalRefHandler verifies the referenced Portal exists.
type ValidatePortalRefHandler struct {
	client client.Client
}

// NewValidatePortalRefHandler creates a new ValidatePortalRefHandler.
func NewValidatePortalRefHandler(c client.Client) *ValidatePortalRefHandler {
	return &ValidatePortalRefHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *ValidatePortalRefHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource
	var portal sreportalv1alpha1.Portal
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: inv.Namespace, Name: inv.Spec.PortalRef}, &portal); err != nil {
		if apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("portal %q not found in namespace %q", inv.Spec.PortalRef, inv.Namespace)
			_ = statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionFalse, ReasonPortalNotFound, msg)
			return fmt.Errorf("%w: %s", ErrPortalNotFound, msg)
		}
		return fmt.Errorf("get portal: %w", err)
	}
	return nil
}

// --- Handler 3: UpdateStatus ---

// UpdateStatusHandler patches the Ready condition and observedGeneration.
type UpdateStatusHandler struct {
	client client.Client
}

// NewUpdateStatusHandler creates a new UpdateStatusHandler.
func NewUpdateStatusHandler(c client.Client) *UpdateStatusHandler {
	return &UpdateStatusHandler{client: c}
}

// Handle implements reconciler.Handler.
func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.ImageInventory, ChainData]) error {
	inv := rc.Resource

	if inv.Status.ObservedGeneration != inv.GetGeneration() {
		inv.Status.ObservedGeneration = inv.GetGeneration()
		if err := h.client.Status().Update(ctx, inv); err != nil {
			return fmt.Errorf("update observedGeneration: %w", err)
		}
		if err := h.client.Get(ctx, types.NamespacedName{Namespace: inv.Namespace, Name: inv.Name}, inv); err != nil {
			return fmt.Errorf("re-fetch image inventory: %w", err)
		}
	}

	if err := statusutil.SetConditionAndPatch(ctx, h.client, inv, ReadyConditionType, metav1.ConditionTrue, ReasonReconciled, ReconciledMessage); err != nil {
		return err
	}

	rc.Result.RequeueAfter = inv.Spec.EffectiveInterval()
	return nil
}

// --- Shared validation helpers ---

func validateSpec(spec sreportalv1alpha1.ImageInventorySpec) error {
	if spec.PortalRef == "" {
		return field.Required(field.NewPath("spec", "portalRef"), "portalRef is required")
	}
	if spec.LabelSelector != "" {
		if _, err := labels.Parse(spec.LabelSelector); err != nil {
			return field.Invalid(field.NewPath("spec", "labelSelector"), spec.LabelSelector, err.Error())
		}
	}
	// Defense-in-depth: WatchedKinds is also constrained at the CRD admission layer
	// via the ImageInventoryKind enum, but we validate defaults and any slipped-through
	// values here as well so the controller never scans unsupported kinds.
	for _, kind := range spec.EffectiveWatchedKinds() {
		if !isSupportedWorkloadKind(kind) {
			return field.NotSupported(
				field.NewPath("spec", "watchedKinds"),
				kind,
				[]string{
					string(sreportalv1alpha1.ImageInventoryKindDeployment),
					string(sreportalv1alpha1.ImageInventoryKindStatefulSet),
					string(sreportalv1alpha1.ImageInventoryKindDaemonSet),
					string(sreportalv1alpha1.ImageInventoryKindCronJob),
					string(sreportalv1alpha1.ImageInventoryKindJob),
				},
			)
		}
	}
	return nil
}

func isSupportedWorkloadKind(kind sreportalv1alpha1.ImageInventoryKind) bool {
	return slices.Contains([]sreportalv1alpha1.ImageInventoryKind{
		sreportalv1alpha1.ImageInventoryKindDeployment,
		sreportalv1alpha1.ImageInventoryKindStatefulSet,
		sreportalv1alpha1.ImageInventoryKindDaemonSet,
		sreportalv1alpha1.ImageInventoryKindCronJob,
		sreportalv1alpha1.ImageInventoryKindJob,
	}, kind)
}
