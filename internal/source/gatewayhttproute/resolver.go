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

package gatewayhttproute

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

// SourceTypeGatewayHTTPRoute identifies gateway.networking.k8s.io HTTPRoute sources.
const SourceTypeGatewayHTTPRoute registry.SourceType = "gateway-httproute"

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                    { return &Resolver{} }
func (*Resolver) Type() registry.SourceType     { return SourceTypeGatewayHTTPRoute }
func (*Resolver) ObjectList() client.ObjectList { return &gwapiv1.HTTPRouteList{} }

func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	rt, ok := obj.(*gwapiv1.HTTPRoute)
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
