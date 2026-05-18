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

package crossplanescalewayrecord_test

import (
	"context"
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	cprec "github.com/golgoth31/sreportal/internal/source/crossplanescalewayrecord"
	"github.com/golgoth31/sreportal/internal/source/registry"
)

func TestCrossplaneScalewayRecordResolver_SubdomainRecord(t *testing.T) {
	r := cprec.NewResolver()
	u := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"namespace": "default", "name": "rec"},
		"spec": map[string]any{"forProvider": map[string]any{
			"type":    "A",
			"data":    "1.2.3.4",
			"dnsZone": "example.com",
			"name":    "api",
		}},
	}}
	eps, err := r.ResolveObject(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 {
		t.Fatalf("want 1 endpoint, got %d", len(eps))
	}
	if eps[0].DNSName != "api.example.com" {
		t.Errorf("DNSName = %q, want %q", eps[0].DNSName, "api.example.com")
	}
	if eps[0].RecordType != "A" {
		t.Errorf("RecordType = %q, want %q", eps[0].RecordType, "A")
	}
	if eps[0].Targets[0] != "1.2.3.4" {
		t.Errorf("Targets[0] = %q, want %q", eps[0].Targets[0], "1.2.3.4")
	}
}

func TestCrossplaneScalewayRecordResolver_ApexRecord(t *testing.T) {
	r := cprec.NewResolver()
	u := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"namespace": "default", "name": "rec"},
		"spec": map[string]any{"forProvider": map[string]any{
			"type":    "A",
			"data":    "1.2.3.4",
			"dnsZone": "example.com",
		}},
	}}
	eps, err := r.ResolveObject(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "example.com" {
		t.Fatalf("unexpected: %+v", eps)
	}
}

func TestCrossplaneScalewayRecordResolver_MissingRequiredFields(t *testing.T) {
	r := cprec.NewResolver()
	cases := []map[string]any{
		{"type": "A", "data": "1.2.3.4"},              // missing dnsZone
		{"type": "A", "dnsZone": "example.com"},       // missing data
		{"data": "1.2.3.4", "dnsZone": "example.com"}, // missing type
	}
	for i, fp := range cases {
		u := &unstructured.Unstructured{Object: map[string]any{
			"spec": map[string]any{"forProvider": fp},
		}}
		eps, err := r.ResolveObject(context.Background(), u)
		if err != nil {
			t.Fatalf("case %d: unexpected err: %v", i, err)
		}
		if eps != nil {
			t.Errorf("case %d: expected nil endpoints, got %+v", i, eps)
		}
	}
}

func TestCrossplaneScalewayRecordResolver_WrongType(t *testing.T) {
	r := cprec.NewResolver()
	eps, err := r.ResolveObject(context.Background(), nil)
	if err == nil {
		t.Fatal("expected UnexpectedObjectTypeError, got nil")
	}
	var typed *registry.UnexpectedObjectTypeError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *registry.UnexpectedObjectTypeError, got %T (%v)", err, err)
	}
	if eps != nil {
		t.Errorf("expected nil endpoints, got %+v", eps)
	}
}
