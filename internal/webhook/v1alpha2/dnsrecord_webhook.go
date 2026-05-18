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

package v1alpha2

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/log"
)

const (
	dnsOwnerAPIVersion = "sreportal.io/v1alpha2"
	dnsOwnerKind       = "DNS"
)

// nolint:unused
// dnsrecordv2log is for logging in this package.
var dnsrecordv2log = log.Default().WithName("dnsrecord-v1alpha2-resource")

// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha2-dnsrecord,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=dnsrecords,verbs=create;update,versions=v1alpha2,name=vdnsrecord-v1alpha2.kb.io,admissionReviewVersions=v1

// DNSRecordCustomValidator validates DNSRecord v1alpha2 resources.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type DNSRecordCustomValidator struct {
	client       client.Client
	controllerSA string
}

// NewDNSRecordCustomValidator constructs a DNSRecordCustomValidator. Exported for unit tests.
func NewDNSRecordCustomValidator(c client.Client, controllerSA string) *DNSRecordCustomValidator {
	return &DNSRecordCustomValidator{client: c, controllerSA: controllerSA}
}

// SetupDNSRecordWebhookWithManager registers the v1alpha2 DNSRecord validating webhook with the manager.
func SetupDNSRecordWebhookWithManager(mgr ctrl.Manager, controllerSA string) error {
	return ctrl.NewWebhookManagedBy(mgr, &sreportalv1alpha2.DNSRecord{}).
		WithValidator(&DNSRecordCustomValidator{client: mgr.GetClient(), controllerSA: controllerSA}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator.
func (v *DNSRecordCustomValidator) ValidateCreate(ctx context.Context, obj *sreportalv1alpha2.DNSRecord) (admission.Warnings, error) {
	dnsrecordv2log.Info("Validation for DNSRecord upon creation", "name", obj.GetName())
	return nil, v.validate(ctx, obj, nil)
}

// ValidateUpdate implements webhook.CustomValidator.
func (v *DNSRecordCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *sreportalv1alpha2.DNSRecord) (admission.Warnings, error) {
	dnsrecordv2log.Info("Validation for DNSRecord upon update", "name", newObj.GetName())
	return nil, v.validate(ctx, newObj, oldObj)
}

// ValidateDelete implements webhook.CustomValidator.
func (v *DNSRecordCustomValidator) ValidateDelete(_ context.Context, _ *sreportalv1alpha2.DNSRecord) (admission.Warnings, error) {
	return nil, nil
}

func (v *DNSRecordCustomValidator) validate(ctx context.Context, r *sreportalv1alpha2.DNSRecord, old *sreportalv1alpha2.DNSRecord) error {
	switch r.Spec.Origin {
	case sreportalv1alpha2.DNSRecordOriginAuto:
		if r.Spec.SourceType == "" {
			return fmt.Errorf("spec.sourceType is required when spec.origin=auto")
		}
		if len(r.Spec.Entries) > 0 {
			return fmt.Errorf("spec.entries must be empty when spec.origin=auto")
		}
		// origin=auto reserved to controller SA. Fail closed if we cannot
		// determine the caller (no admission context): refusing is safer than
		// letting an unauthenticated path through.
		if v.controllerSA != "" {
			req, err := admission.RequestFromContext(ctx)
			if err != nil {
				return fmt.Errorf("cannot determine caller identity for origin=auto: %w", err)
			}
			if req.UserInfo.Username != v.controllerSA {
				return fmt.Errorf("spec.origin=auto is reserved for the operator controller (caller: %q)", req.UserInfo.Username)
			}
		}
	case sreportalv1alpha2.DNSRecordOriginManual:
		if r.Spec.SourceType != "" {
			return fmt.Errorf("spec.sourceType must be empty when spec.origin=manual")
		}
		if len(r.Spec.Entries) == 0 {
			return fmt.Errorf("spec.entries must have at least one entry when spec.origin=manual")
		}
	}

	if old != nil && old.Spec.Origin != r.Spec.Origin {
		return fmt.Errorf("spec.origin is immutable: cannot change from %q to %q", old.Spec.Origin, r.Spec.Origin)
	}
	if old != nil && old.Spec.PortalRef != r.Spec.PortalRef {
		return fmt.Errorf("spec.portalRef is immutable: cannot change from %q to %q", old.Spec.PortalRef, r.Spec.PortalRef)
	}

	return v.validateOwnerRef(ctx, r, old)
}

// validateOwnerRef enforces the multi-DNS-per-portal ownership invariants:
//   - origin=auto MUST carry exactly one controller ownerReference to a DNS
//     in the same namespace, with blockOwnerDeletion=true, and the record's
//     portalRef must match the owner DNS portalRef.
//   - origin=manual MAY carry such an ownerReference (one-way adoption for
//     cascade-delete); when it does, the same invariants apply.
//   - On update the controller ownerRef is immutable; absent→present
//     adoption is allowed, but renaming or removing the owner is not.
func (v *DNSRecordCustomValidator) validateOwnerRef(ctx context.Context, r, old *sreportalv1alpha2.DNSRecord) error {
	refs := dnsControllerOwnerRefs(r.GetOwnerReferences())
	if len(refs) > 1 {
		return fmt.Errorf("at most one ownerReference with kind=DNS, controller=true is allowed (found %d)", len(refs))
	}

	hasRef := len(refs) == 1

	if r.Spec.Origin == sreportalv1alpha2.DNSRecordOriginAuto && !hasRef {
		return fmt.Errorf("ownerReference to a DNS (controller=true) is required when spec.origin=auto")
	}

	if old != nil {
		oldRefs := dnsControllerOwnerRefs(old.GetOwnerReferences())
		switch {
		case len(oldRefs) == 1 && !hasRef:
			return fmt.Errorf("ownerReference to DNS cannot be removed once set")
		case len(oldRefs) == 1 && hasRef:
			oldRef, newRef := oldRefs[0], refs[0]
			if oldRef.Name != newRef.Name || oldRef.UID != newRef.UID {
				return fmt.Errorf("ownerReference to DNS is immutable: cannot re-parent from %q to %q", oldRef.Name, newRef.Name)
			}
		}
	}

	if !hasRef {
		return nil
	}

	ref := refs[0]
	if ref.BlockOwnerDeletion == nil || !*ref.BlockOwnerDeletion {
		return fmt.Errorf("ownerReference to DNS must set blockOwnerDeletion=true")
	}

	var owner sreportalv1alpha2.DNS
	if err := v.client.Get(ctx, types.NamespacedName{Namespace: r.Namespace, Name: ref.Name}, &owner); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("owner DNS %q not found in namespace %q", ref.Name, r.Namespace)
		}
		return fmt.Errorf("failed to fetch owner DNS %q: %w", ref.Name, err)
	}

	if r.Spec.PortalRef != owner.Spec.PortalRef {
		return fmt.Errorf("spec.portalRef %q must match owner DNS spec.portalRef %q", r.Spec.PortalRef, owner.Spec.PortalRef)
	}

	return nil
}

// dnsControllerOwnerRefs filters ownerReferences down to those that point to a
// sreportal DNS CR as controller. The webhook tolerates non-controller refs
// of any kind; only controller refs of kind=DNS, apiVersion=sreportal.io/v1alpha2
// participate in the ownership invariants.
func dnsControllerOwnerRefs(refs []metav1.OwnerReference) []metav1.OwnerReference {
	out := make([]metav1.OwnerReference, 0, 1)
	for _, ref := range refs {
		if ref.APIVersion != dnsOwnerAPIVersion || ref.Kind != dnsOwnerKind {
			continue
		}
		if ref.Controller == nil || !*ref.Controller {
			continue
		}
		out = append(out, ref)
	}
	return out
}
