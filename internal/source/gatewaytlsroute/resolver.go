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

package gatewaytlsroute

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

// SourceTypeGatewayTLSRoute identifies gateway.networking.k8s.io TLSRoute sources.
const SourceTypeGatewayTLSRoute registry.SourceType = "gateway-tlsroute"

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=tlsroutes,verbs=get;list;watch

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                    { return &Resolver{} }
func (*Resolver) Type() registry.SourceType     { return SourceTypeGatewayTLSRoute }
func (*Resolver) ObjectList() client.ObjectList { return &gwapiv1alpha2.TLSRouteList{} }

func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	rt, ok := obj.(*gwapiv1alpha2.TLSRoute)
	if !ok {
		return nil, nil
	}
	target := rt.Annotations["external-dns.alpha.kubernetes.io/target"]
	if target == "" || len(rt.Spec.Hostnames) == 0 {
		return nil, nil
	}
	out := make([]*endpoint.Endpoint, 0, len(rt.Spec.Hostnames))
	for _, h := range rt.Spec.Hostnames {
		s := strings.TrimSuffix(string(h), ".")
		if s == "" {
			continue
		}
		out = append(out, endpoint.NewEndpoint(s, endpoint.RecordTypeA, target))
	}
	return out, nil
}
