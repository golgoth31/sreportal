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
	"sigs.k8s.io/external-dns/endpoint"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

// FieldIndexPortalRef is the field index name used to look up resources by spec.portalRef.
// Duplicated here to avoid a circular import with the parent controller package.
const FieldIndexPortalRef = "spec.portalRef"

// ChainData holds typed shared state between source reconciliation handlers.
type ChainData struct {
	// TypedSources is the snapshot of built sources for this tick.
	TypedSources []registry.TypedSource

	// Index is the portal lookup structure built by BuildPortalIndexHandler.
	Index *PortalIndex

	// EndpointsByPortalSource holds collected endpoints keyed by (portal, sourceType).
	EndpointsByPortalSource map[PortalSourceKey][]*endpoint.Endpoint
}

// PortalSourceKey identifies a unique (portal, sourceType) pair.
type PortalSourceKey struct {
	PortalName string
	SourceType registry.SourceType
}

// PortalIndex is a pre-computed lookup structure built from the portal list.
type PortalIndex struct {
	Main   *sreportalv1alpha1.Portal
	ByName map[string]*sreportalv1alpha1.Portal
	Local  []*sreportalv1alpha1.Portal
}
