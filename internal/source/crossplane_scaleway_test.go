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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"sigs.k8s.io/external-dns/endpoint"
)

func newFakeRecord(name string, atProvider map[string]any, annotations map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "domain.scaleway.m.upbound.io/v1alpha1",
			"kind":       "Record",
			"metadata": map[string]any{
				"name": name,
			},
			"status": map[string]any{
				"atProvider": atProvider,
			},
		},
	}
	if len(annotations) > 0 {
		obj.SetAnnotations(annotations)
	}
	return obj
}

func TestCrossplaneScalewayRecordSource_recordToEndpoints(t *testing.T) {
	cases := []struct {
		name        string
		atProvider  map[string]any
		annotations map[string]string
		wantDNS     string
		wantType    string
		wantTargets []string
		wantTTL     endpoint.TTL
		wantLabels  map[string]string
		wantErr     bool
	}{
		{
			name: "full record with fqdn",
			atProvider: map[string]any{
				"fqdn":    "app.example.com.",
				"type":    "A",
				"data":    "1.2.3.4",
				"ttl":     float64(300),
				"dnsZone": "example.com",
				"name":    "app",
			},
			wantDNS:     "app.example.com",
			wantType:    "A",
			wantTargets: []string{"1.2.3.4"},
			wantTTL:     300,
		},
		{
			name: "fallback to name + dnsZone when fqdn absent",
			atProvider: map[string]any{
				"type":    "CNAME",
				"data":    "other.example.com",
				"ttl":     float64(600),
				"dnsZone": "example.com",
				"name":    "www",
			},
			wantDNS:     "www.example.com",
			wantType:    "CNAME",
			wantTargets: []string{"other.example.com"},
			wantTTL:     600,
		},
		{
			name: "root record (empty name)",
			atProvider: map[string]any{
				"type":    "A",
				"data":    "5.6.7.8",
				"dnsZone": "example.com",
				"name":    "",
			},
			wantDNS:     "example.com",
			wantType:    "A",
			wantTargets: []string{"5.6.7.8"},
		},
		{
			name: "sreportal annotations propagated",
			atProvider: map[string]any{
				"fqdn": "app.example.com",
				"type": "A",
				"data": "1.2.3.4",
			},
			annotations: map[string]string{
				"sreportal.io/portal": "prod",
				"sreportal.io/groups": "infra,dns",
				"sreportal.io/ignore": "false",
			},
			wantDNS:     "app.example.com",
			wantType:    "A",
			wantTargets: []string{"1.2.3.4"},
			wantLabels: map[string]string{
				"sreportal.io/portal": "prod",
				"sreportal.io/groups": "infra,dns",
				"sreportal.io/ignore": "false",
			},
		},
		{
			name:       "missing status.atProvider",
			atProvider: nil,
			wantErr:    true,
		},
		{
			name: "missing type",
			atProvider: map[string]any{
				"fqdn": "app.example.com",
				"data": "1.2.3.4",
			},
			wantErr: true,
		},
		{
			name: "missing data",
			atProvider: map[string]any{
				"fqdn": "app.example.com",
				"type": "A",
			},
			wantErr: true,
		},
		{
			name: "missing both fqdn and dnsZone",
			atProvider: map[string]any{
				"type": "A",
				"data": "1.2.3.4",
				"name": "app",
			},
			wantErr: true,
		},
	}

	src := &CrossplaneScalewayRecordSource{}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var obj *unstructured.Unstructured
			if tc.atProvider == nil {
				// Build an object without status.atProvider.
				obj = &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "domain.scaleway.m.upbound.io/v1alpha1",
						"kind":       "Record",
						"metadata":   map[string]any{"name": "test"},
					},
				}
			} else {
				obj = newFakeRecord("test", tc.atProvider, tc.annotations)
			}

			eps, err := src.recordToEndpoints(obj)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, eps, 1)

			ep := eps[0]
			assert.Equal(t, tc.wantDNS, ep.DNSName)
			assert.Equal(t, tc.wantType, ep.RecordType)
			assert.Equal(t, tc.wantTargets, []string(ep.Targets))
			assert.Equal(t, tc.wantTTL, ep.RecordTTL)

			if tc.wantLabels != nil {
				for k, v := range tc.wantLabels {
					assert.Equal(t, v, ep.Labels[k], "label %s", k)
				}
			}
		})
	}
}

func TestCrossplaneScalewayRecordSource_Endpoints(t *testing.T) {
	scheme := runtime.NewScheme()

	rec1 := newFakeRecord("rec-a", map[string]any{
		"fqdn": "a.example.com",
		"type": "A",
		"data": "1.1.1.1",
		"ttl":  float64(300),
	}, nil)

	rec2 := newFakeRecord("rec-b", map[string]any{
		"type":    "CNAME",
		"data":    "b-target.example.com",
		"dnsZone": "example.com",
		"name":    "b",
	}, nil)

	// Record with missing data — should be skipped without error.
	recBad := newFakeRecord("rec-bad", map[string]any{
		"fqdn": "bad.example.com",
		"type": "A",
		// data is missing
	}, nil)

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			CrossplaneScalewayRecordGVR: "RecordList",
		},
		rec1, rec2, recBad,
	)

	src := NewCrossplaneScalewayRecordSource(client, labels.Everything())

	eps, err := src.Endpoints(context.Background())
	require.NoError(t, err)
	require.Len(t, eps, 2, "bad record should be skipped")

	// Endpoints may be in any order; sort by DNSName for assertion stability.
	if eps[0].DNSName > eps[1].DNSName {
		eps[0], eps[1] = eps[1], eps[0]
	}

	assert.Equal(t, "a.example.com", eps[0].DNSName)
	assert.Equal(t, "A", eps[0].RecordType)
	assert.Equal(t, endpoint.Targets{"1.1.1.1"}, eps[0].Targets)
	assert.Equal(t, endpoint.TTL(300), eps[0].RecordTTL)

	assert.Equal(t, "b.example.com", eps[1].DNSName)
	assert.Equal(t, "CNAME", eps[1].RecordType)
	assert.Equal(t, endpoint.Targets{"b-target.example.com"}, eps[1].Targets)
}
