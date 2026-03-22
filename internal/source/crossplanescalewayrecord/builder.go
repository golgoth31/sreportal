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

package crossplanescalewayrecord

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// +kubebuilder:rbac:groups=domain.scaleway.upbound.io,resources=records,verbs=get;list;watch

const SourceTypeCrossplaneScalewayRecord registry.SourceType = "crossplane-scaleway-record"

// Builder creates Crossplane Scaleway Record sources.
type Builder struct{}

// NewBuilder creates a new Crossplane Scaleway Record source builder.
func NewBuilder() *Builder { return &Builder{} }

func (b *Builder) Type() registry.SourceType { return SourceTypeCrossplaneScalewayRecord }

func (b *Builder) Enabled(cfg *config.OperatorConfig) bool {
	return cfg.Sources.CrossplaneScalewayRecord != nil && cfg.Sources.CrossplaneScalewayRecord.Enabled
}

func (b *Builder) Build(_ context.Context, deps registry.Deps, cfg *config.OperatorConfig) (registry.Source, error) {
	dynClient, err := dynamic.NewForConfig(deps.RestConfig)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}

	sc := cfg.Sources.CrossplaneScalewayRecord

	return &Source{
		dynamicClient: dynClient,
		namespace:     sc.Namespace,
		labelSelector: sc.LabelFilter,
		clusterScoped: sc.ClusterScoped,
	}, nil
}

func (b *Builder) GVR() (schema.GroupVersionResource, bool) {
	return gvr, true
}
