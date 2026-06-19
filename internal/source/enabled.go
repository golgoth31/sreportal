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
	"github.com/golgoth31/sreportal/internal/source/externaldns"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// EnabledKindsFromSpec maps DNS.spec.sources to (SourceType -> enabled).
// Only true entries are emitted (callers may freely check map[k] for absence).
func EnabledKindsFromSpec(s *sreportalv1alpha2.SourcesSpec) map[registry.SourceType]bool {
	out := map[registry.SourceType]bool{}
	if s == nil {
		return out
	}
	if s.Service != nil && s.Service.Enabled {
		out[externaldns.KindService] = true
	}
	if s.Ingress != nil && s.Ingress.Enabled {
		out[externaldns.KindIngress] = true
	}
	if s.DNSEndpoint != nil && s.DNSEndpoint.Enabled {
		out[externaldns.KindDNSEndpoint] = true
	}
	if s.IstioGateway != nil && s.IstioGateway.Enabled {
		out[externaldns.KindIstioGateway] = true
	}
	if s.IstioVirtualService != nil && s.IstioVirtualService.Enabled {
		out[externaldns.KindIstioVirtualService] = true
	}
	if s.GatewayHTTPRoute != nil && s.GatewayHTTPRoute.Enabled {
		out[externaldns.KindGatewayHTTPRoute] = true
	}
	if s.GatewayGRPCRoute != nil && s.GatewayGRPCRoute.Enabled {
		out[externaldns.KindGatewayGRPCRoute] = true
	}
	if s.GatewayTLSRoute != nil && s.GatewayTLSRoute.Enabled {
		out[externaldns.KindGatewayTLSRoute] = true
	}
	if s.GatewayTCPRoute != nil && s.GatewayTCPRoute.Enabled {
		out[externaldns.KindGatewayTCPRoute] = true
	}
	if s.GatewayUDPRoute != nil && s.GatewayUDPRoute.Enabled {
		out[externaldns.KindGatewayUDPRoute] = true
	}
	if s.CrossplaneScalewayRecord != nil && s.CrossplaneScalewayRecord.Enabled {
		out[crossplanescalewayrecord.SourceTypeCrossplaneScalewayRecord] = true
	}
	return out
}
