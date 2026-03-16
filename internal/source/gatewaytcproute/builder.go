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

package gatewaytcproute

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	externaldnssource "sigs.k8s.io/external-dns/source"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=tcproutes,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch

const SourceTypeGatewayTCPRoute registry.SourceType = "gateway-tcproute"

// Builder creates external-dns Gateway TCPRoute sources.
type Builder struct {
	gatewayClient gateway.Interface
}

// NewBuilder creates a new Gateway TCPRoute source builder.
func NewBuilder() *Builder { return &Builder{} }

func (b *Builder) Type() registry.SourceType { return SourceTypeGatewayTCPRoute }

func (b *Builder) Enabled(cfg *config.OperatorConfig) bool {
	return cfg.Sources.GatewayTCPRoute != nil && cfg.Sources.GatewayTCPRoute.Enabled
}

func (b *Builder) Build(_ context.Context, deps registry.Deps, cfg *config.OperatorConfig) (registry.Source, error) {
	if err := b.ensureGatewayClient(deps); err != nil {
		return nil, err
	}
	gc := cfg.Sources.GatewayTCPRoute
	srcCfg, err := registry.NewGatewaySourceConfig(gc)
	if err != nil {
		return nil, err
	}
	clients := registry.NewGatewayClientGenerator(deps, b.gatewayClient)
	return externaldnssource.NewGatewayTCPRouteSource(clients, srcCfg)
}

func (b *Builder) GVR() (schema.GroupVersionResource, bool) {
	return schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "", // resolved at runtime via discovery (cluster-dependent)
		Resource: "tcproutes",
	}, true
}

func (b *Builder) ensureGatewayClient(deps registry.Deps) error {
	if b.gatewayClient != nil {
		return nil
	}
	var err error
	b.gatewayClient, err = registry.NewGatewayClient(deps.RestConfig)
	return err
}
