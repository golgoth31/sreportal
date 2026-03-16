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
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// Factory assembles external-dns sources from registered builders.
type Factory struct {
	deps     registry.Deps
	builders []registry.Builder
}

// NewFactory creates a Factory with the given builders.
func NewFactory(kubeClient kubernetes.Interface, restConfig *rest.Config, builders []registry.Builder) *Factory {
	return &Factory{
		deps: registry.Deps{
			KubeClient: kubeClient,
			RestConfig: restConfig,
		},
		builders: builders,
	}
}

// BuildTypedSources creates external-dns sources for all enabled types.
// Sources are built in the order of registered builders.
func (f *Factory) BuildTypedSources(ctx context.Context, cfg *config.OperatorConfig) ([]registry.TypedSource, error) {
	log := ctrl.Log.WithName("source-factory")
	typedSources := make([]registry.TypedSource, 0, len(f.builders))

	for _, b := range f.builders {
		if !b.Enabled(cfg) {
			log.V(1).Info("source not enabled", "type", b.Type())
			continue
		}
		log.Info("building source", "type", b.Type())
		src, err := b.Build(ctx, f.deps, cfg)
		if err != nil {
			return nil, fmt.Errorf("build %s source: %w", b.Type(), err)
		}
		typedSources = append(typedSources, registry.TypedSource{Type: b.Type(), Source: src})
		log.Info("source built successfully", "type", b.Type())
	}

	log.Info("sources built", "count", len(typedSources))
	return typedSources, nil
}

// EnabledSourceTypes returns the list of enabled source types from configuration.
func (f *Factory) EnabledSourceTypes(cfg *config.OperatorConfig) []registry.SourceType {
	var types []registry.SourceType
	for _, b := range f.builders {
		if b.Enabled(cfg) {
			types = append(types, b.Type())
		}
	}
	return types
}

// GVRForSourceType resolves the GroupVersionResource for a given source type
// by looking up the registered builder. Returns false if no builder matches
// or the source type does not support annotation enrichment.
func (f *Factory) GVRForSourceType(sourceType registry.SourceType) (schema.GroupVersionResource, bool) {
	for _, b := range f.builders {
		if b.Type() == sourceType {
			return b.GVR()
		}
	}
	return schema.GroupVersionResource{}, false
}
