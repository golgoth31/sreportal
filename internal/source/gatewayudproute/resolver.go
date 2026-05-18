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

package gatewayudproute

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

// SourceTypeGatewayUDPRoute identifies gateway.networking.k8s.io UDPRoute sources.
const SourceTypeGatewayUDPRoute registry.SourceType = "gateway-udproute"

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=udproutes,verbs=get;list;watch

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                    { return &Resolver{} }
func (*Resolver) Type() registry.SourceType     { return SourceTypeGatewayUDPRoute }
func (*Resolver) ObjectList() client.ObjectList { return &gwapiv1alpha2.UDPRouteList{} }

func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	rt, ok := obj.(*gwapiv1alpha2.UDPRoute)
	if !ok {
		return nil, nil
	}
	target := rt.Annotations["external-dns.alpha.kubernetes.io/target"]
	host := rt.Annotations["external-dns.alpha.kubernetes.io/hostname"]
	if target == "" || host == "" {
		return nil, nil
	}
	s := strings.TrimSuffix(host, ".")
	if s == "" {
		return nil, nil
	}
	return []*endpoint.Endpoint{endpoint.NewEndpoint(s, endpoint.RecordTypeA, target)}, nil
}
