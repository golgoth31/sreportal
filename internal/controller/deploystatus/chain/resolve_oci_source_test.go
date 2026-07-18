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
	"context"
	"testing"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

func TestResolveOCISource_UnmatchedHostMarksUnresolved(t *testing.T) {
	h := NewResolveOCISourceHandler([]config.ForgeConfig{
		{Host: testForgeHost, Auth: config.ForgeAuthConfig{TokenEnv: "X"}},
	})
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: &sreportalv1alpha1.DeployStatus{},
		Data: ChainData{Due: []WorkItem{
			{Key: "a", SourceURL: "https://gitlab.com/o/r"}, // unmatched host
			{Key: "b", SourceURL: "https://github.com/o/r"}, // matched
			{Key: "c", SourceURL: ""},                       // no label -> unresolved
		}},
	}
	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatal(err)
	}

	// a and c become unresolved ComputedEntry; b remains Due.
	gotUnresolved := 0
	for _, e := range rc.Data.Computed {
		if e.State == stateUnresolved {
			gotUnresolved++
		}
	}
	if gotUnresolved != 2 {
		t.Fatalf("unresolved = %d, want 2", gotUnresolved)
	}
	if len(rc.Data.Due) != 1 || rc.Data.Due[0].Key != "b" {
		t.Fatalf("Due should retain only matched item b, got %+v", rc.Data.Due)
	}
}

func TestResolveOCISource_MatchedItemKeepsWorkload(t *testing.T) {
	h := NewResolveOCISourceHandler([]config.ForgeConfig{
		{Host: testForgeHost, Auth: config.ForgeAuthConfig{TokenEnv: "X"}},
	})
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: &sreportalv1alpha1.DeployStatus{},
		Data: ChainData{Due: []WorkItem{
			{Key: "x", SourceURL: "https://github.com/myorg/myrepo"},
		}},
	}
	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatal(err)
	}
	if len(rc.Data.Due) != 1 {
		t.Fatalf("expected 1 due item, got %d", len(rc.Data.Due))
	}
	wi := rc.Data.Due[0]
	if wi.Workload.Host != "github.com" || wi.Workload.Owner != "myorg" || wi.Workload.Repo != "myrepo" {
		t.Errorf("unexpected workload ref: %+v", wi.Workload)
	}
	if len(rc.Data.Computed) != 0 {
		t.Errorf("matched item should not produce a computed entry, got %+v", rc.Data.Computed)
	}
}

func TestResolveOCISource_UnparseableURLMarksUnresolved(t *testing.T) {
	h := NewResolveOCISourceHandler([]config.ForgeConfig{
		{Host: testForgeHost, Auth: config.ForgeAuthConfig{TokenEnv: "X"}},
	})
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: &sreportalv1alpha1.DeployStatus{},
		Data: ChainData{Due: []WorkItem{
			{Key: "bad", SourceURL: "not-a-valid-url-with-no-path"},
		}},
	}
	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatal(err)
	}
	if len(rc.Data.Computed) != 1 || rc.Data.Computed[0].State != stateUnresolved {
		t.Errorf("expected one unresolved entry, got %+v", rc.Data.Computed)
	}
	if len(rc.Data.Due) != 0 {
		t.Errorf("unparseable item should be removed from Due")
	}
}
