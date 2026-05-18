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

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

// SourceTypeDNSEndpoint identifies external-dns DNSEndpoint sources.
const SourceTypeDNSEndpoint registry.SourceType = "dnsendpoint"

// +kubebuilder:rbac:groups=externaldns.k8s.io,resources=dnsendpoints,verbs=get;list;watch

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                    { return &Resolver{} }
func (*Resolver) Type() registry.SourceType     { return SourceTypeDNSEndpoint }
func (*Resolver) ObjectList() client.ObjectList { return &v1alpha1.DNSEndpointList{} }

func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	de, ok := obj.(*v1alpha1.DNSEndpoint)
	if !ok || de == nil {
		return nil, nil
	}
	if len(de.Spec.Endpoints) == 0 {
		return nil, nil
	}
	out := make([]*endpoint.Endpoint, 0, len(de.Spec.Endpoints))
	for _, e := range de.Spec.Endpoints {
		if e == nil || e.DNSName == "" {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
