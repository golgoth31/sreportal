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

package service

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	externaldnssource "sigs.k8s.io/external-dns/source"

	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch

const SourceTypeService registry.SourceType = "service"

// Builder creates external-dns Service sources.
type Builder struct{}

// NewBuilder creates a new Service source builder.
func NewBuilder() *Builder { return &Builder{} }

func (b *Builder) Type() registry.SourceType { return SourceTypeService }

func (b *Builder) Enabled(cfg *config.OperatorConfig) bool {
	return cfg.Sources.Service != nil && cfg.Sources.Service.Enabled
}

func (b *Builder) Build(ctx context.Context, deps registry.Deps, cfg *config.OperatorConfig) (registry.Source, error) {
	sc := cfg.Sources.Service
	labelSelector, err := registry.ParseLabelSelector(sc.LabelFilter)
	if err != nil {
		return nil, err
	}

	return externaldnssource.NewServiceSource(
		ctx,
		deps.KubeClient,
		sc.Namespace,
		sc.AnnotationFilter,
		sc.FQDNTemplate,
		sc.CombineFQDNAndAnnotation,
		"",
		sc.PublishInternal,
		sc.PublishHostIP,
		false,
		sc.ServiceTypeFilter,
		sc.IgnoreHostnameAnnotation,
		labelSelector,
		sc.ResolveLoadBalancerHostname,
		false,
		false,
	)
}

func (b *Builder) GVR() (schema.GroupVersionResource, bool) {
	return schema.GroupVersionResource{Version: "v1", Resource: "services"}, true
}
