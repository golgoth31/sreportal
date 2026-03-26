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

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// SourceProvider provides access to the source lifecycle (rebuild + snapshot).
type SourceProvider interface {
	GetTypedSources() []registry.TypedSource
	RebuildSources(ctx context.Context) error
}

// EndpointEnricher enriches endpoints with K8s resource annotations.
type EndpointEnricher interface {
	EnrichEndpoints(ctx context.Context, sourceType registry.SourceType, endpoints []*endpoint.Endpoint)
}

// GVRResolver resolves a partial GVR (missing Version) to a full GVR via discovery.
type GVRResolver interface {
	ResolveGVR(gvr schema.GroupVersionResource) (schema.GroupVersionResource, error)
}

// SourceFailureTracker tracks and surfaces consecutive source failures.
type SourceFailureTracker interface {
	RecordFailure(sourceType registry.SourceType) int
	RecordRecovery(sourceType registry.SourceType) int
	MarkDegraded(ctx context.Context, portal *sreportalv1alpha1.Portal, sourceType registry.SourceType, cause error, count int)
}

// EnabledSourcesLister lists source types that are currently enabled.
type EnabledSourcesLister interface {
	EnabledSourceTypes() []registry.SourceType
}
