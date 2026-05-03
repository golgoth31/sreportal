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

package chain

import (
	"testing"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

const (
	flowNodeAPI   = "service:core:api"
	flowNodeOther = "service:core:other"
	flowKindSvc   = "service"
)

func TestIsEvaluable(t *testing.T) {
	h := &ObserveFlowsHandler{}

	tests := []struct {
		name      string
		edge      *sreportalv1alpha1.FlowEdge
		evalTypes map[string]bool
		want      bool
	}{
		{
			name:      "service-to-service with service in eval types",
			edge:      &sreportalv1alpha1.FlowEdge{From: flowNodeAPI, To: "service:core:db-proxy"},
			evalTypes: map[string]bool{flowKindSvc: true},
			want:      true,
		},
		{
			name:      "service-to-database with only service in eval types",
			edge:      &sreportalv1alpha1.FlowEdge{From: flowNodeAPI, To: "database:core:postgres"},
			evalTypes: map[string]bool{flowKindSvc: true},
			want:      false,
		},
		{
			name:      "cron-to-database with only service in eval types",
			edge:      &sreportalv1alpha1.FlowEdge{From: "cron:jobs:cleanup", To: "database:data:postgres"},
			evalTypes: map[string]bool{flowKindSvc: true},
			want:      false,
		},
		{
			name:      "service-to-external with only service in eval types",
			edge:      &sreportalv1alpha1.FlowEdge{From: flowNodeAPI, To: "external:core:vault.example.com"},
			evalTypes: map[string]bool{flowKindSvc: true},
			want:      false,
		},
		{
			name:      "service-to-database with both in eval types",
			edge:      &sreportalv1alpha1.FlowEdge{From: flowNodeAPI, To: "database:data:postgres"},
			evalTypes: map[string]bool{flowKindSvc: true, "database": true},
			want:      true,
		},
		{
			name:      "empty eval types disables evaluation",
			edge:      &sreportalv1alpha1.FlowEdge{From: flowNodeAPI, To: flowNodeOther},
			evalTypes: map[string]bool{},
			want:      false,
		},
		{
			name:      "nil eval types disables evaluation",
			edge:      &sreportalv1alpha1.FlowEdge{From: flowNodeAPI, To: flowNodeOther},
			evalTypes: nil,
			want:      false,
		},
		{
			name:      "malformed node ID returns false",
			edge:      &sreportalv1alpha1.FlowEdge{From: "bad-id", To: flowNodeAPI},
			evalTypes: map[string]bool{flowKindSvc: true},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.isEvaluable(tt.edge, tt.evalTypes)
			if got != tt.want {
				t.Errorf("isEvaluable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluatedDerivedFromSpec(t *testing.T) {
	// Verify that if an edge type is removed from evaluatedEdgeTypes,
	// the Evaluated flag is correctly set to false (not carried forward).
	h := &ObserveFlowsHandler{}

	edge := &sreportalv1alpha1.FlowEdge{From: flowNodeAPI, To: flowNodeOther}

	// Initially evaluable.
	evalWithService := map[string]bool{flowKindSvc: true}
	if !h.isEvaluable(edge, evalWithService) {
		t.Fatal("expected evaluable with service in eval types")
	}

	// After removing service from eval types, not evaluable anymore.
	evalEmpty := map[string]bool{}
	if h.isEvaluable(edge, evalEmpty) {
		t.Fatal("expected not evaluable after removing service from eval types")
	}
}
