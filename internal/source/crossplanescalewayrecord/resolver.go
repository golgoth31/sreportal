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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

// SourceTypeCrossplaneScalewayRecord identifies Crossplane scaleway.crossplane.io Record sources.
const SourceTypeCrossplaneScalewayRecord registry.SourceType = "crossplane-scaleway-record"

// +kubebuilder:rbac:groups=domain.scaleway.upbound.io,resources=records,verbs=get;list;watch

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                { return &Resolver{} }
func (*Resolver) Type() registry.SourceType { return SourceTypeCrossplaneScalewayRecord }

func (*Resolver) ObjectList() client.ObjectList {
	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvr.Group,
		Version: gvr.Version,
		Kind:    "RecordList",
	})
	return u
}

func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, nil
	}
	ep, ok := recordToEndpoint(u)
	if !ok {
		return nil, nil
	}
	return []*endpoint.Endpoint{ep}, nil
}
