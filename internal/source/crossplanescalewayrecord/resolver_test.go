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

const (
	tNSDefault    = "default"
	tForProvider  = "forProvider"
	tKeyType      = "type"
	tKeyDNSZone   = "dnsZone"
	tKeyData      = "data"
	tKeyMetadata  = "metadata"
	tKeySpec      = "spec"
	tZoneExample  = "example.com"
	tTargetIP1234 = "1.2.3.4"
	tKeyName      = "name"
	tKeyNamespace = "namespace"
	tNameRec      = "rec"
)

func TestCrossplaneScalewayRecordResolver_SubdomainRecord(t *testing.T) {
	r := cprec.NewResolver()
	u := &unstructured.Unstructured{Object: map[string]any{
		tKeyMetadata: map[string]any{tKeyNamespace: tNSDefault, tKeyName: tNameRec},
		tKeySpec: map[string]any{tForProvider: map[string]any{
			tKeyType:    "A",
			tKeyData:    tTargetIP1234,
			tKeyDNSZone: tZoneExample,
			tKeyName:    "api",
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
	if eps[0].Targets[0] != tTargetIP1234 {
		t.Errorf("Targets[0] = %q, want %q", eps[0].Targets[0], tTargetIP1234)
	}
}

func TestCrossplaneScalewayRecordResolver_ApexRecord(t *testing.T) {
	r := cprec.NewResolver()
	u := &unstructured.Unstructured{Object: map[string]any{
		tKeyMetadata: map[string]any{tKeyNamespace: tNSDefault, tKeyName: tNameRec},
		tKeySpec: map[string]any{tForProvider: map[string]any{
			tKeyType:    "A",
			tKeyData:    tTargetIP1234,
			tKeyDNSZone: tZoneExample,
		}},
	}}
	eps, err := r.ResolveObject(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != tZoneExample {
		t.Fatalf("unexpected: %+v", eps)
	}
}

func TestCrossplaneScalewayRecordResolver_MissingRequiredFields(t *testing.T) {
	r := cprec.NewResolver()
	cases := []map[string]any{
		{tKeyType: "A", tKeyData: tTargetIP1234},             // missing dnsZone
		{tKeyType: "A", tKeyDNSZone: tZoneExample},           // missing data
		{tKeyData: tTargetIP1234, tKeyDNSZone: tZoneExample}, // missing type
	}
	for i, fp := range cases {
		u := &unstructured.Unstructured{Object: map[string]any{
			tKeySpec: map[string]any{tForProvider: fp},
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

// TestCrossplaneScalewayRecordResolver_TrailingDot verifies that the resolver
// strips trailing dots from dnsZone (and name) before concatenating, so a
// dnsZone "example.com." resolves to DNSName "api.example.com" (scenario 3).
func TestCrossplaneScalewayRecordResolver_TrailingDot(t *testing.T) {
	r := cprec.NewResolver()
	u := &unstructured.Unstructured{Object: map[string]any{
		tKeyMetadata: map[string]any{tKeyNamespace: tNSDefault, tKeyName: tNameRec},
		tKeySpec: map[string]any{tForProvider: map[string]any{
			tKeyType:    "A",
			tKeyData:    tTargetIP1234,
			tKeyDNSZone: "example.com.",
			tKeyName:    "api",
		}},
	}}
	eps, err := r.ResolveObject(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].DNSName != "api.example.com" {
		t.Errorf("expected trailing dot stripped from dnsZone, got %q", eps[0].DNSName)
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
