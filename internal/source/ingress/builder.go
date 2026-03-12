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

package ingress

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	externaldnssource "sigs.k8s.io/external-dns/source"

	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch

const SourceTypeIngress registry.SourceType = "ingress"

// Builder creates external-dns Ingress sources.
type Builder struct{}

// NewBuilder creates a new Ingress source builder.
func NewBuilder() *Builder { return &Builder{} }

func (b *Builder) Type() registry.SourceType { return SourceTypeIngress }

func (b *Builder) Enabled(cfg *config.OperatorConfig) bool {
	return cfg.Sources.Ingress != nil && cfg.Sources.Ingress.Enabled
}

func (b *Builder) Build(ctx context.Context, deps registry.Deps, cfg *config.OperatorConfig) (registry.Source, error) {
	ic := cfg.Sources.Ingress
	labelSelector, err := registry.ParseLabelSelector(ic.LabelFilter)
	if err != nil {
		return nil, err
	}

	return externaldnssource.NewIngressSource(
		ctx,
		deps.KubeClient,
		ic.Namespace,
		ic.AnnotationFilter,
		ic.FQDNTemplate,
		ic.CombineFQDNAndAnnotation,
		ic.IgnoreHostnameAnnotation,
		ic.IgnoreIngressTLSSpec,
		ic.IgnoreIngressRulesSpec,
		labelSelector,
		ic.IngressClassNames,
	)
}

// GVR returns false: ingress annotation enrichment was not enabled in the
// original implementation and changing that is out of scope for this refactor.
func (b *Builder) GVR() (schema.GroupVersionResource, bool) {
	return schema.GroupVersionResource{}, false
}
