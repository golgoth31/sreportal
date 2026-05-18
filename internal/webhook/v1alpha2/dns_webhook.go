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

	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/log"
)

// nolint:unused
// dnsv2log is for logging in this package.
var dnsv2log = log.Default().WithName("dns-v1alpha2-resource")

// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha2-dns,mutating=false,failurePolicy=fail,sideEffects=None,groups=sreportal.io,resources=dns,verbs=create;update,versions=v1alpha2,name=vdns-v1alpha2.kb.io,admissionReviewVersions=v1

// DNSCustomValidator validates DNS v1alpha2 resources.
//
// NOTE: Portal existence is validated by the v1alpha1 DNS webhook which holds the client.
// This validator only enforces structural invariants (portalRef immutability).
// Multiple DNS CRs may reference the same Portal (N:1 allowed).
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type DNSCustomValidator struct{}

// NewDNSCustomValidator constructs a DNSCustomValidator. Exported for unit tests.
func NewDNSCustomValidator() *DNSCustomValidator {
	return &DNSCustomValidator{}
}

// SetupDNSWebhookWithManager registers the v1alpha2 DNS validating webhook with the manager.
func SetupDNSWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &sreportalv1alpha2.DNS{}).
		WithValidator(&DNSCustomValidator{}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator.
func (v *DNSCustomValidator) ValidateCreate(_ context.Context, obj *sreportalv1alpha2.DNS) (admission.Warnings, error) {
	dnsv2log.Info("Validation for DNS upon creation", "name", obj.GetName())
	return nil, v.validate(obj)
}

// ValidateUpdate implements webhook.CustomValidator.
func (v *DNSCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *sreportalv1alpha2.DNS) (admission.Warnings, error) {
	dnsv2log.Info("Validation for DNS upon update", "name", newObj.GetName())
	if oldObj.Spec.PortalRef != newObj.Spec.PortalRef {
		return nil, fmt.Errorf("portalRef is immutable: cannot change from %q to %q", oldObj.Spec.PortalRef, newObj.Spec.PortalRef)
	}
	return nil, v.validate(newObj)
}

// ValidateDelete implements webhook.CustomValidator.
func (v *DNSCustomValidator) ValidateDelete(_ context.Context, _ *sreportalv1alpha2.DNS) (admission.Warnings, error) {
	return nil, nil
}

// validate runs structural invariant checks shared by both create and update paths.
func (v *DNSCustomValidator) validate(obj *sreportalv1alpha2.DNS) error {
	if obj.Spec.Defaults.LabelFilter != "" {
		if _, err := labels.Parse(obj.Spec.Defaults.LabelFilter); err != nil {
			return fmt.Errorf("spec.defaults.labelFilter: %w", err)
		}
	}
	for kind, lf := range collectLabelFilters(&obj.Spec.Sources) {
		if lf == "" {
			continue
		}
		if _, err := labels.Parse(lf); err != nil {
			return fmt.Errorf("spec.sources.%s.labelFilter: %w", kind, err)
		}
	}
	enabled := enabledSourceTypes(&obj.Spec.Sources)
	for _, p := range obj.Spec.Sources.Priority {
		if _, ok := enabled[p]; !ok {
			return fmt.Errorf("spec.sources.priority entry %q is not an enabled source in this DNS", p)
		}
	}
	return nil
}

// collectLabelFilters returns a map of source JSON key → LabelFilter for every
// non-nil source pointer in SourcesSpec. No reflection — explicit field access.
//
// Keep in sync with enabledSourceTypes when adding new source kinds.
func collectLabelFilters(s *sreportalv1alpha2.SourcesSpec) map[string]string {
	m := make(map[string]string)
	if s.Service != nil {
		m["service"] = s.Service.LabelFilter
	}
	if s.Ingress != nil {
		m["ingress"] = s.Ingress.LabelFilter
	}
	if s.DNSEndpoint != nil {
		m["dnsEndpoint"] = s.DNSEndpoint.LabelFilter
	}
	if s.IstioGateway != nil {
		m["istioGateway"] = s.IstioGateway.LabelFilter
	}
	if s.IstioVirtualService != nil {
		m["istioVirtualService"] = s.IstioVirtualService.LabelFilter
	}
	if s.GatewayHTTPRoute != nil {
		m["gatewayHTTPRoute"] = s.GatewayHTTPRoute.LabelFilter
	}
	if s.GatewayGRPCRoute != nil {
		m["gatewayGRPCRoute"] = s.GatewayGRPCRoute.LabelFilter
	}
	if s.GatewayTLSRoute != nil {
		m["gatewayTLSRoute"] = s.GatewayTLSRoute.LabelFilter
	}
	if s.GatewayTCPRoute != nil {
		m["gatewayTCPRoute"] = s.GatewayTCPRoute.LabelFilter
	}
	if s.GatewayUDPRoute != nil {
		m["gatewayUDPRoute"] = s.GatewayUDPRoute.LabelFilter
	}
	if s.CrossplaneScalewayRecord != nil {
		m["crossplaneScalewayRecord"] = s.CrossplaneScalewayRecord.LabelFilter
	}
	return m
}

// enabledSourceTypes returns the set of SourceType values whose corresponding
// source pointer is non-nil AND whose Enabled field is true. No reflection.
//
// Keep in sync with collectLabelFilters when adding new source kinds.
func enabledSourceTypes(s *sreportalv1alpha2.SourcesSpec) map[sreportalv1alpha2.SourceType]struct{} {
	m := make(map[sreportalv1alpha2.SourceType]struct{})
	if s.Service != nil && s.Service.Enabled {
		m[sreportalv1alpha2.SourceTypeService] = struct{}{}
	}
	if s.Ingress != nil && s.Ingress.Enabled {
		m[sreportalv1alpha2.SourceTypeIngress] = struct{}{}
	}
	if s.DNSEndpoint != nil && s.DNSEndpoint.Enabled {
		m[sreportalv1alpha2.SourceTypeDNSEndpoint] = struct{}{}
	}
	if s.IstioGateway != nil && s.IstioGateway.Enabled {
		m[sreportalv1alpha2.SourceTypeIstioGateway] = struct{}{}
	}
	if s.IstioVirtualService != nil && s.IstioVirtualService.Enabled {
		m[sreportalv1alpha2.SourceTypeIstioVirtualService] = struct{}{}
	}
	if s.GatewayHTTPRoute != nil && s.GatewayHTTPRoute.Enabled {
		m[sreportalv1alpha2.SourceTypeGatewayHTTPRoute] = struct{}{}
	}
	if s.GatewayGRPCRoute != nil && s.GatewayGRPCRoute.Enabled {
		m[sreportalv1alpha2.SourceTypeGatewayGRPCRoute] = struct{}{}
	}
	if s.GatewayTLSRoute != nil && s.GatewayTLSRoute.Enabled {
		m[sreportalv1alpha2.SourceTypeGatewayTLSRoute] = struct{}{}
	}
	if s.GatewayTCPRoute != nil && s.GatewayTCPRoute.Enabled {
		m[sreportalv1alpha2.SourceTypeGatewayTCPRoute] = struct{}{}
	}
	if s.GatewayUDPRoute != nil && s.GatewayUDPRoute.Enabled {
		m[sreportalv1alpha2.SourceTypeGatewayUDPRoute] = struct{}{}
	}
	if s.CrossplaneScalewayRecord != nil && s.CrossplaneScalewayRecord.Enabled {
		m[sreportalv1alpha2.SourceTypeCrossplaneScalewayRecord] = struct{}{}
	}
	return m
}
