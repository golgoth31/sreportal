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

package istiogateway

import (
	"context"

	istioclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/apimachinery/pkg/runtime/schema"
	externaldnssource "sigs.k8s.io/external-dns/source"

	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// +kubebuilder:rbac:groups=networking.istio.io,resources=gateways,verbs=get;list;watch

const SourceTypeIstioGateway registry.SourceType = "istio-gateway"

// Builder creates external-dns Istio Gateway sources.
type Builder struct {
	istioClient istioclient.Interface
}

// NewBuilder creates a new Istio Gateway source builder.
func NewBuilder() *Builder { return &Builder{} }

func (b *Builder) Type() registry.SourceType { return SourceTypeIstioGateway }

func (b *Builder) Enabled(cfg *config.OperatorConfig) bool {
	return cfg.Sources.IstioGateway != nil && cfg.Sources.IstioGateway.Enabled
}

func (b *Builder) Build(ctx context.Context, deps registry.Deps, cfg *config.OperatorConfig) (registry.Source, error) {
	if err := b.ensureIstioClient(deps); err != nil {
		return nil, err
	}
	gc := cfg.Sources.IstioGateway
	return externaldnssource.NewIstioGatewaySource(
		ctx,
		deps.KubeClient,
		b.istioClient,
		gc.Namespace,
		gc.AnnotationFilter,
		gc.FQDNTemplate,
		gc.CombineFQDNAndAnnotation,
		gc.IgnoreHostnameAnnotation,
	)
}

func (b *Builder) GVR() (schema.GroupVersionResource, bool) {
	return schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1", Resource: "gateways"}, true
}

func (b *Builder) ensureIstioClient(deps registry.Deps) error {
	if b.istioClient != nil {
		return nil
	}
	var err error
	b.istioClient, err = registry.NewIstioClient(deps.RestConfig)
	return err
}
