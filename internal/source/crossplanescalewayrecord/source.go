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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/external-dns/endpoint"
)

var gvr = schema.GroupVersionResource{
	Group:    "domain.scaleway.upbound.io",
	Version:  "v1alpha1",
	Resource: "records",
}

// Source implements the external-dns Source interface for Crossplane Scaleway DNS Records.
type Source struct {
	dynamicClient dynamic.Interface
	namespace     string
	labelSelector string
	clusterScoped bool
}

// AddEventHandler is a no-op; the source controller polls periodically.
func (s *Source) AddEventHandler(_ context.Context, _ func()) {}

// Endpoints returns external-dns endpoints derived from Crossplane Scaleway Record resources.
func (s *Source) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error) {
	var list *unstructured.UnstructuredList
	var err error

	listOpts := metav1.ListOptions{LabelSelector: s.labelSelector}

	if s.clusterScoped {
		list, err = s.dynamicClient.Resource(gvr).List(ctx, listOpts)
	} else {
		list, err = s.dynamicClient.Resource(gvr).Namespace(s.namespace).List(ctx, listOpts)
	}
	if err != nil {
		return nil, fmt.Errorf("list crossplane scaleway records: %w", err)
	}

	var endpoints []*endpoint.Endpoint
	for i := range list.Items {
		ep, ok := recordToEndpoint(&list.Items[i])
		if ok {
			endpoints = append(endpoints, ep)
		}
	}
	return endpoints, nil
}

// recordToEndpoint converts a Crossplane Scaleway Record to an external-dns Endpoint.
// Returns false if the record is missing required fields (type, data, dnsZone).
func recordToEndpoint(obj *unstructured.Unstructured) (*endpoint.Endpoint, bool) {
	forProvider, ok := nestedMap(obj.Object, "spec", "forProvider")
	if !ok {
		return nil, false
	}

	recordType, _ := forProvider["type"].(string)
	data, _ := forProvider["data"].(string)
	dnsZone, _ := forProvider["dnsZone"].(string)

	if recordType == "" || data == "" || dnsZone == "" {
		return nil, false
	}

	name, _ := forProvider["name"].(string)

	var dnsName string
	if name == "" {
		dnsName = dnsZone
	} else {
		dnsName = name + "." + dnsZone
	}

	ep := &endpoint.Endpoint{
		DNSName:    dnsName,
		Targets:    endpoint.Targets{data},
		RecordType: recordType,
		Labels:     make(map[string]string),
	}

	if ttl, ok := nestedInt64(forProvider, "ttl"); ok && ttl > 0 {
		ep.RecordTTL = endpoint.TTL(ttl)
	}

	// Set resource label for annotation enrichment in the source controller.
	ns := obj.GetNamespace()
	ep.Labels[endpoint.ResourceLabelKey] = fmt.Sprintf("record/%s/%s", ns, obj.GetName())

	return ep, true
}

// nestedMap safely extracts a nested map from an unstructured object.
func nestedMap(obj map[string]any, fields ...string) (map[string]any, bool) {
	current := obj
	for _, f := range fields {
		next, ok := current[f].(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

// nestedInt64 extracts an int64 from a map, handling JSON number types.
func nestedInt64(m map[string]any, key string) (int64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}
