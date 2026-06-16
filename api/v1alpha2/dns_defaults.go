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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DefaultSourcesSpec returns the built-in default source configuration used
// when the operator boots without a legacy sources ConfigMap. It enables the
// most common discovery sources — Service, Ingress, and the non-Istio Gateway
// API HTTP/GRPC routes — and leaves everything else off.
func DefaultSourcesSpec() SourcesSpec {
	enabled := func() CommonSourceSpec { return CommonSourceSpec{Enabled: true} }
	return SourcesSpec{
		Service:          &ServiceSourceSpec{CommonSourceSpec: enabled()},
		Ingress:          &IngressSourceSpec{CommonSourceSpec: enabled()},
		GatewayHTTPRoute: &GatewayRouteSourceSpec{CommonSourceSpec: enabled()},
		GatewayGRPCRoute: &GatewayRouteSourceSpec{CommonSourceSpec: enabled()},
		Priority: []SourceType{
			SourceTypeIngress,
			SourceTypeService,
			SourceTypeGatewayHTTPRoute,
			SourceTypeGatewayGRPCRoute,
		},
	}
}

// DefaultGroupMappingSpec returns the built-in default group mapping.
func DefaultGroupMappingSpec() GroupMappingSpec {
	return GroupMappingSpec{DefaultGroup: "Services"}
}

// DefaultReconciliationSpec returns the built-in default reconciliation timing,
// matching the CRD field defaults (interval 5m, retryOnError 30s).
func DefaultReconciliationSpec() ReconciliationSpec {
	return ReconciliationSpec{
		Interval:     metav1.Duration{Duration: 5 * time.Minute},
		RetryOnError: metav1.Duration{Duration: 30 * time.Second},
	}
}
