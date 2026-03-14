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

// DefaultBuilders returns all registered source builders in default order.
// Used by the Source reconciler and passed to the DNS reconciler for priority filtering.
func DefaultBuilders() []registry.Builder {
	return []registry.Builder{
		service.NewBuilder(),
		ingress.NewBuilder(),
		dnsendpoint.NewBuilder(),
		istiogateway.NewBuilder(),
		istiovirtualservice.NewBuilder(),
		gatewayhttproute.NewBuilder(),
		gatewaygrpcroute.NewBuilder(),
		gatewaytlsroute.NewBuilder(),
		gatewaytcproute.NewBuilder(),
		gatewayudproute.NewBuilder(),
	}
}
