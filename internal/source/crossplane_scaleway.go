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
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/golgoth31/sreportal/internal/adapter"
)

// CrossplaneScalewayRecordGVR is the GroupVersionResource for the Crossplane
// Scaleway Record CRD (domain.scaleway.m.upbound.io/v1alpha1).
// Exported so that source_controller.gvrForSourceType can reuse it.
var CrossplaneScalewayRecordGVR = schema.GroupVersionResource{
	Group:    "domain.scaleway.m.upbound.io",
	Version:  "v1alpha1",
	Resource: "records",
}

// CrossplaneScalewayRecordSource lists Crossplane Scaleway Record CRDs and
// converts them into external-dns Endpoints. It implements the external-dns
// source.Source interface so it can be used transparently alongside native
// external-dns sources.
type CrossplaneScalewayRecordSource struct {
	dynamicClient dynamic.Interface
	labelSelector labels.Selector
}

// NewCrossplaneScalewayRecordSource creates a source that watches Crossplane
// Scaleway Record CRDs. The labelSelector filters which Records are considered.
func NewCrossplaneScalewayRecordSource(
	dynamicClient dynamic.Interface,
	labelSelector labels.Selector,
) *CrossplaneScalewayRecordSource {
	return &CrossplaneScalewayRecordSource{
		dynamicClient: dynamicClient,
		labelSelector: labelSelector,
	}
}

// Endpoints returns the list of endpoints derived from Crossplane Scaleway Records.
func (s *CrossplaneScalewayRecordSource) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error) {
	log := ctrl.Log.WithName("crossplane-scaleway-source")

	// Crossplane Records are cluster-scoped; list without namespace.
	listOpts := metav1.ListOptions{}
	if s.labelSelector != nil && !s.labelSelector.Empty() {
		listOpts.LabelSelector = s.labelSelector.String()
	}

	records, err := s.dynamicClient.Resource(CrossplaneScalewayRecordGVR).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("list crossplane scaleway records: %w", err)
	}

	var endpoints []*endpoint.Endpoint

	for i := range records.Items {
		eps, err := s.recordToEndpoints(&records.Items[i])
		if err != nil {
			log.V(2).Info("skipping record", "name", records.Items[i].GetName(), "error", err)
			continue
		}
		endpoints = append(endpoints, eps...)
	}

	log.V(1).Info("collected endpoints from crossplane scaleway records", "count", len(endpoints))

	return endpoints, nil
}

// AddEventHandler is a no-op; this source uses polling via the SourceReconciler ticker.
func (s *CrossplaneScalewayRecordSource) AddEventHandler(_ context.Context, _ func()) {}

// recordToEndpoints converts a single unstructured Crossplane Scaleway Record
// into one or more external-dns Endpoints.
//
// The Record CRD structure (observed state in status.atProvider):
//   - fqdn:    the fully qualified domain name (computed by the provider)
//   - name:    the record name (subdomain part)
//   - dnsZone: the DNS zone
//   - type:    the record type (A, AAAA, CNAME, MX, TXT, etc.)
//   - data:    the record value / target
//   - ttl:     the TTL in seconds
//
// When status.atProvider.fqdn is available, it is used directly. Otherwise,
// the FQDN is constructed from name + "." + dnsZone.
func (s *CrossplaneScalewayRecordSource) recordToEndpoints(obj *unstructured.Unstructured) ([]*endpoint.Endpoint, error) {
	atProvider, found, err := unstructured.NestedMap(obj.Object, "status", "atProvider")
	if err != nil {
		return nil, fmt.Errorf("read status.atProvider: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("missing status.atProvider")
	}

	fqdn, err := resolveFQDN(atProvider)
	if err != nil {
		return nil, err
	}

	recordType := nestedString(atProvider, "type")
	if recordType == "" {
		return nil, fmt.Errorf("missing type in status.atProvider")
	}

	data := nestedString(atProvider, "data")
	if data == "" {
		return nil, fmt.Errorf("missing data in status.atProvider")
	}

	ep := &endpoint.Endpoint{
		DNSName:    fqdn,
		RecordType: recordType,
		Targets:    endpoint.NewTargets(data),
		RecordTTL:  parseTTL(atProvider),
		Labels: map[string]string{
			// Crossplane Records are cluster-scoped, so we use a two-segment
			// resource label (kind/name). enrichEndpoints in source_controller
			// expects three segments (kind/namespace/name) and silently skips
			// two-segment labels, which is correct here because annotation
			// enrichment is handled directly by this source.
			endpoint.ResourceLabelKey: fmt.Sprintf("Record/%s", obj.GetName()),
		},
	}

	// Copy sreportal annotations from the Record's annotations to endpoint labels.
	adapter.EnrichEndpointLabels(ep, obj.GetAnnotations())

	return []*endpoint.Endpoint{ep}, nil
}

// resolveFQDN extracts the fully qualified domain name from a Crossplane Scaleway
// Record's atProvider map. It prefers the computed "fqdn" field, falling back to
// constructing it from "name" + "." + "dnsZone".
func resolveFQDN(atProvider map[string]any) (string, error) {
	fqdn := nestedString(atProvider, "fqdn")
	if fqdn == "" {
		name := nestedString(atProvider, "name")
		dnsZone := nestedString(atProvider, "dnsZone")
		if dnsZone == "" {
			return "", fmt.Errorf("no fqdn and no dnsZone in status.atProvider")
		}
		if name == "" {
			fqdn = dnsZone
		} else {
			fqdn = name + "." + dnsZone
		}
	}

	// Ensure trailing dot is removed for consistency with external-dns.
	return strings.TrimSuffix(fqdn, "."), nil
}

// parseTTL extracts the TTL value from an atProvider map. Returns 0 if absent
// or not a numeric type.
func parseTTL(atProvider map[string]any) endpoint.TTL {
	ttlVal, ok := atProvider["ttl"]
	if !ok {
		return 0
	}
	switch v := ttlVal.(type) {
	case float64:
		return endpoint.TTL(v)
	case int64:
		return endpoint.TTL(v)
	default:
		return 0
	}
}

// nestedString extracts a string value from a map, returning "" if the key
// is absent or not a string.
func nestedString(obj map[string]any, key string) string {
	val, ok := obj[key]
	if !ok {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return ""
	}
	return s
}
