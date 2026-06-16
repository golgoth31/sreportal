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

package service

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/source/registry"
)

// SourceTypeService identifies Kubernetes Service sources.
const SourceTypeService registry.SourceType = "service"

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch

// Resolver converts Service objects from the controller-runtime cache into
// external-dns Endpoints.
type Resolver struct{}

var _ registry.Resolver = (*Resolver)(nil)

// NewResolver creates a new Service Resolver.
func NewResolver() *Resolver { return &Resolver{} }

// Type returns the source kind handled by this Resolver.
func (*Resolver) Type() registry.SourceType { return SourceTypeService }

// ObjectList returns a fresh empty ServiceList suitable for cache.List.
func (*Resolver) ObjectList() client.ObjectList { return &corev1.ServiceList{} }

// ResolveObject converts a single Service object into zero or more Endpoints.
// For each Service carrying the external-dns hostname annotation, it emits an
// A-record from LoadBalancer ingress IPs and/or a CNAME from ingress Hostnames.
func (*Resolver) ResolveObject(_ context.Context, obj client.Object) ([]*endpoint.Endpoint, error) {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		return nil, registry.UnexpectedObjectType(SourceTypeService, obj)
	}
	host := svc.Annotations["external-dns.alpha.kubernetes.io/hostname"]
	if host == "" {
		return nil, nil
	}
	ips, hostnames := loadBalancerTargets(svc)
	if len(ips) == 0 && len(hostnames) == 0 {
		return nil, nil
	}
	dnsName := strings.TrimSuffix(host, ".")
	var eps []*endpoint.Endpoint
	if len(ips) > 0 {
		eps = append(eps, endpoint.NewEndpoint(dnsName, endpoint.RecordTypeA, ips...))
	}
	if len(hostnames) > 0 {
		eps = append(eps, endpoint.NewEndpoint(dnsName, endpoint.RecordTypeCNAME, hostnames...))
	}
	return eps, nil
}

func loadBalancerTargets(svc *corev1.Service) (ips, hostnames []string) {
	for _, lb := range svc.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			ips = append(ips, lb.IP)
		}
		if lb.Hostname != "" {
			hostnames = append(hostnames, lb.Hostname)
		}
	}
	return ips, hostnames
}
