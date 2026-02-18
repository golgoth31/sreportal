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

package v1alpha1

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var dnslog = logf.Log.WithName("dns-resource")

// SetupDNSWebhookWithManager registers the webhook for DNS in the manager.
func SetupDNSWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &sreportalv1alpha1.DNS{}).
		WithValidator(&DNSCustomValidator{client: mgr.GetClient()}).
		WithDefaulter(&DNSCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-sreportal-my-domain-v1alpha1-dns,mutating=true,failurePolicy=fail,sideEffects=None,groups=sreportal.my.domain,resources=dns,verbs=create;update,versions=v1alpha1,name=mdns-v1alpha1.kb.io,admissionReviewVersions=v1

// DNSCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind DNS when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type DNSCustomDefaulter struct{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind DNS.
func (d *DNSCustomDefaulter) Default(_ context.Context, obj *sreportalv1alpha1.DNS) error {
	dnslog.Info("Defaulting for DNS", "name", obj.GetName())

	return nil
}

// +kubebuilder:webhook:path=/validate-sreportal-my-domain-v1alpha1-dns,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.my.domain,resources=dns,verbs=create;update,versions=v1alpha1,name=vdns-v1alpha1.kb.io,admissionReviewVersions=v1

// DNSCustomValidator struct is responsible for validating the DNS resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type DNSCustomValidator struct {
	client client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type DNS.
func (v *DNSCustomValidator) ValidateCreate(ctx context.Context, obj *sreportalv1alpha1.DNS) (admission.Warnings, error) {
	dnslog.Info("Validation for DNS upon creation", "name", obj.GetName())

	return v.validatePortalRef(ctx, obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type DNS.
func (v *DNSCustomValidator) ValidateUpdate(ctx context.Context, _, newObj *sreportalv1alpha1.DNS) (admission.Warnings, error) {
	dnslog.Info("Validation for DNS upon update", "name", newObj.GetName())

	return v.validatePortalRef(ctx, newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type DNS.
func (v *DNSCustomValidator) ValidateDelete(_ context.Context, obj *sreportalv1alpha1.DNS) (admission.Warnings, error) {
	dnslog.Info("Validation for DNS upon deletion", "name", obj.GetName())

	return nil, nil
}

// validatePortalRef checks that the referenced portal exists.
func (v *DNSCustomValidator) validatePortalRef(ctx context.Context, obj *sreportalv1alpha1.DNS) (admission.Warnings, error) {
	if obj.Spec.PortalRef == "" {
		return nil, fmt.Errorf("spec.portalRef is required")
	}

	var portal sreportalv1alpha1.Portal
	key := types.NamespacedName{
		Name:      obj.Spec.PortalRef,
		Namespace: obj.Namespace,
	}

	if err := v.client.Get(ctx, key, &portal); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("referenced portal %q not found in namespace %q", obj.Spec.PortalRef, obj.Namespace)
		}
		return nil, fmt.Errorf("failed to check portal reference: %w", err)
	}

	return nil, nil
}
