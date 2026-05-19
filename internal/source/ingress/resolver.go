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

package ingress

import (
	"context"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

// SourceTypeIngress identifies networking.k8s.io/v1 Ingress sources.
const SourceTypeIngress registry.SourceType = "ingress"

// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch

type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

func NewResolver() *Resolver                    { return &Resolver{} }
func (*Resolver) Type() registry.SourceType     { return SourceTypeIngress }
func (*Resolver) ObjectList() client.ObjectList { return &networkingv1.IngressList{} }

func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	ing, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return nil, registry.UnexpectedObjectType(SourceTypeIngress, obj)
	}
	host := ing.Annotations["external-dns.alpha.kubernetes.io/hostname"]
	if host == "" {
		return nil, nil
	}
	var targets []string
	for _, lb := range ing.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			targets = append(targets, lb.IP)
		}
	}
	if len(targets) == 0 {
		return nil, nil
	}
	return []*endpoint.Endpoint{
		endpoint.NewEndpoint(strings.TrimSuffix(host, "."), endpoint.RecordTypeA, targets...),
	}, nil
}
