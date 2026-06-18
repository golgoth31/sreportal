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

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

const annotationV1Alpha2DNSRecordSpec = "sreportal.io/v1alpha2-dnsrecord-spec"

// SetupDNSRecordWebhookWithManager registers the v1alpha1 DNSRecord webhook.
func SetupDNSRecordWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &sreportalv1alpha1.DNSRecord{}).
		WithValidator(&DNSRecordValidator{}).
		Complete()
}

// matchPolicy=Exact so this webhook only fires for genuine v1alpha1 requests.
// With the default (Equivalent), the apiserver converts v1alpha2 writes to
// v1alpha1 and routes them here too — which wrongly blocks the DNS controller's
// own v1alpha2 record updates (conversion stamps every record with the
// v1alpha2-spec annotation this validator keys on).
// +kubebuilder:webhook:path=/validate-sreportal-io-v1alpha1-dnsrecord,mutating=false,failurePolicy=fail,matchPolicy=Exact,sideEffects=None,groups=sreportal.io,resources=dnsrecords,verbs=update,versions=v1alpha1,name=vdnsrecord-v1alpha1.kb.io,admissionReviewVersions=v1

// DNSRecordValidator rejects updates that would silently clobber the v1alpha2-only
// data (origin, entries) stored in the spec annotation when a client edits a
// v1alpha2-backed record through the v1alpha1 surface.
type DNSRecordValidator struct{}

func (v *DNSRecordValidator) ValidateCreate(_ context.Context, _ *sreportalv1alpha1.DNSRecord) (admission.Warnings, error) {
	return nil, nil
}

func (v *DNSRecordValidator) ValidateUpdate(_ context.Context, oldObj, _ *sreportalv1alpha1.DNSRecord) (admission.Warnings, error) {
	if oldObj == nil {
		return nil, nil
	}
	if _, backedByV2 := oldObj.Annotations[annotationV1Alpha2DNSRecordSpec]; !backedByV2 {
		return nil, nil
	}
	return nil, fmt.Errorf("DNSRecord backed by v1alpha2 cannot be modified via v1alpha1; use sreportal.io/v1alpha2")
}

func (v *DNSRecordValidator) ValidateDelete(_ context.Context, _ *sreportalv1alpha1.DNSRecord) (admission.Warnings, error) {
	return nil, nil
}
