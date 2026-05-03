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

package dnsendpoint

import (
	"context"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	externaldnssource "sigs.k8s.io/external-dns/source"

	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

const (
	dnsEndpointAPIVersion = "externaldns.k8s.io/v1alpha1"
	dnsEndpointKind       = "DNSEndpoint"
)

// +kubebuilder:rbac:groups=externaldns.k8s.io,resources=dnsendpoints,verbs=get;list;watch

const SourceTypeDNSEndpoint registry.SourceType = "dnsendpoint"

// Builder creates external-dns CRD (DNSEndpoint) sources.
type Builder struct{}

// NewBuilder creates a new DNSEndpoint source builder.
func NewBuilder() *Builder { return &Builder{} }

func (b *Builder) Type() registry.SourceType { return SourceTypeDNSEndpoint }

func (b *Builder) Enabled(cfg *config.OperatorConfig) bool {
	return cfg.Sources.DNSEndpoint != nil && cfg.Sources.DNSEndpoint.Enabled
}

func (b *Builder) Build(ctx context.Context, deps registry.Deps, cfg *config.OperatorConfig) (registry.Source, error) {
	return externaldnssource.NewCRDSource(ctx, deps.RestConfig, &externaldnssource.Config{
		Namespace:           cfg.Sources.DNSEndpoint.Namespace,
		LabelFilter:         labels.Everything(),
		CRDSourceAPIVersion: dnsEndpointAPIVersion,
		CRDSourceKind:       dnsEndpointKind,
	})
}

func (b *Builder) GVR() (schema.GroupVersionResource, bool) {
	return schema.GroupVersionResource{
		Group:    "externaldns.k8s.io",
		Version:  "v1alpha1",
		Resource: "dnsendpoints",
	}, true
}
