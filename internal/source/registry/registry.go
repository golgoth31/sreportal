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

// Package registry provides the central list of all source builders.
// To add a new source: implement source.Builder in internal/source/<name>,
// add your config to config.SourcesConfig, then append your builder here.
package registry

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	externaldnssource "sigs.k8s.io/external-dns/source"

	"github.com/golgoth31/sreportal/internal/config"
)

// Source is an alias for the external-dns source.Source interface.
type Source = externaldnssource.Source

// SourceType represents the type of external-dns source.
type SourceType string

// TypedSource pairs a Source with its type.
type TypedSource struct {
	Type   SourceType
	Source Source
}

// Deps holds the infrastructure dependencies that source builders need.
type Deps struct {
	KubeClient kubernetes.Interface
	RestConfig *rest.Config
}

// Builder creates an external-dns Source for a specific source type.
// Each source type implements this interface in its own package.
type Builder interface {
	// Type returns the source type identifier.
	Type() SourceType
	// Enabled reports whether this source is configured and active.
	Enabled(cfg *config.OperatorConfig) bool
	// Build constructs the Source. Called only when Enabled returns true.
	Build(ctx context.Context, deps Deps, cfg *config.OperatorConfig) (Source, error)
	// GVR returns the GroupVersionResource for annotation enrichment.
	// Returns false if enrichment is not supported for this source type.
	GVR() (schema.GroupVersionResource, bool)
}

// ParseLabelSelector parses a label selector string, returning Everything for empty input.
func ParseLabelSelector(selector string) (labels.Selector, error) {
	if selector == "" {
		return labels.Everything(), nil
	}
	sel, err := labels.Parse(selector)
	if err != nil {
		return nil, fmt.Errorf("parse label selector %q: %w", selector, err)
	}
	return sel, nil
}
