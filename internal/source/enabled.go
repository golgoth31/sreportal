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

package source

import (
	sreportalv1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
	"github.com/golgoth31/sreportal/internal/source/crossplanescalewayrecord"
	"github.com/golgoth31/sreportal/internal/source/dnsendpoint"
	"github.com/golgoth31/sreportal/internal/source/gatewaygrpcroute"
	"github.com/golgoth31/sreportal/internal/source/gatewayhttproute"
	"github.com/golgoth31/sreportal/internal/source/gatewaytcproute"
	"github.com/golgoth31/sreportal/internal/source/gatewaytlsroute"
	"github.com/golgoth31/sreportal/internal/source/gatewayudproute"
	"github.com/golgoth31/sreportal/internal/source/ingress"
	"github.com/golgoth31/sreportal/internal/source/istiogateway"
	"github.com/golgoth31/sreportal/internal/source/istiovirtualservice"
	"github.com/golgoth31/sreportal/internal/source/registry"
	"github.com/golgoth31/sreportal/internal/source/service"
)

// EnabledKindsFromSpec maps DNS.spec.sources to (SourceType -> enabled).
// Only true entries are emitted (callers may freely check map[k] for absence).
func EnabledKindsFromSpec(s *sreportalv1alpha2.SourcesSpec) map[registry.SourceType]bool {
	out := map[registry.SourceType]bool{}
	if s == nil {
		return out
	}
	if s.Service != nil && s.Service.Enabled {
		out[service.SourceTypeService] = true
	}
	if s.Ingress != nil && s.Ingress.Enabled {
		out[ingress.SourceTypeIngress] = true
	}
	if s.DNSEndpoint != nil && s.DNSEndpoint.Enabled {
		out[dnsendpoint.SourceTypeDNSEndpoint] = true
	}
	if s.IstioGateway != nil && s.IstioGateway.Enabled {
		out[istiogateway.SourceTypeIstioGateway] = true
	}
	if s.IstioVirtualService != nil && s.IstioVirtualService.Enabled {
		out[istiovirtualservice.SourceTypeIstioVirtualService] = true
	}
	if s.GatewayHTTPRoute != nil && s.GatewayHTTPRoute.Enabled {
		out[gatewayhttproute.SourceTypeGatewayHTTPRoute] = true
	}
	if s.GatewayGRPCRoute != nil && s.GatewayGRPCRoute.Enabled {
		out[gatewaygrpcroute.SourceTypeGatewayGRPCRoute] = true
	}
	if s.GatewayTLSRoute != nil && s.GatewayTLSRoute.Enabled {
		out[gatewaytlsroute.SourceTypeGatewayTLSRoute] = true
	}
	if s.GatewayTCPRoute != nil && s.GatewayTCPRoute.Enabled {
		out[gatewaytcproute.SourceTypeGatewayTCPRoute] = true
	}
	if s.GatewayUDPRoute != nil && s.GatewayUDPRoute.Enabled {
		out[gatewayudproute.SourceTypeGatewayUDPRoute] = true
	}
	if s.CrossplaneScalewayRecord != nil && s.CrossplaneScalewayRecord.Enabled {
		out[crossplanescalewayrecord.SourceTypeCrossplaneScalewayRecord] = true
	}
	return out
}
