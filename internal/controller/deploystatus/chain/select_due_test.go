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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

func TestSelectDue_SkipsRecentlyChecked(t *testing.T) {
	now := time.Now()
	refresh := 5 * time.Minute

	// Spec.Services carries the tracked input (identity only).
	specSvcs := []sreportalv1alpha1.DeployStatusEntry{
		{Key: "fresh"},
		{Key: "stale"},
		{Key: "never"},
	}
	// Status.Services carries the observed last-check timestamps.
	statusSvcs := []sreportalv1alpha1.DeployStatusEntry{
		// checked 1 minute ago — within the 5m interval, should be skipped
		{Key: "fresh", LastCheckedAt: metav1.NewTime(now.Add(-time.Minute))},
		// checked 1 hour ago — outside the interval, should be due
		{Key: "stale", LastCheckedAt: metav1.NewTime(now.Add(-time.Hour))},
		// "never" has no status entry — always due
	}

	due := selectDue(specSvcs, statusSvcs, refresh, now)

	gotKeys := map[string]bool{}
	for _, d := range due {
		gotKeys[d.Key] = true
	}

	if gotKeys["fresh"] {
		t.Error("fresh entry should not be due (checked within interval)")
	}
	if !gotKeys["stale"] {
		t.Error("stale entry should be due (last checked > 5m ago)")
	}
	if !gotKeys["never"] {
		t.Error("never-checked entry should always be due")
	}
}

func TestSelectDue_MapsWorkloadFields(t *testing.T) {
	now := time.Now()
	refresh := 5 * time.Minute

	svcs := []sreportalv1alpha1.DeployStatusEntry{
		{
			Key:         "x",
			Image:       "registry.io/app:sha256-abc",
			SourceRepo:  "https://github.com/org/repo",
			DeployedRef: "abc123",
			Workload: sreportalv1alpha1.DeployStatusWorkloadRef{
				Kind:      testKindDeployment,
				Namespace: "prod",
				Name:      testWorkloadApp,
				Container: testDefaultBranch,
			},
		},
	}

	due := selectDue(svcs, nil, refresh, now)
	if len(due) != 1 {
		t.Fatalf("expected 1 due item, got %d", len(due))
	}
	wi := due[0]
	if wi.Key != "x" {
		t.Errorf("key = %q, want x", wi.Key)
	}
	if wi.Image != "registry.io/app:sha256-abc" {
		t.Errorf("image = %q", wi.Image)
	}
	if wi.SourceURL != "https://github.com/org/repo" {
		t.Errorf("sourceURL = %q", wi.SourceURL)
	}
	if wi.DeployedRef != "abc123" {
		t.Errorf("deployedRef = %q", wi.DeployedRef)
	}
	if wi.WorkloadKind != testKindDeployment || wi.WorkloadNamespace != "prod" ||
		wi.WorkloadName != testWorkloadApp || wi.WorkloadContainer != testDefaultBranch {
		t.Errorf("workload fields not mapped correctly: %+v", wi)
	}
}

func TestSelectDue_EmptyServicesProducesNoDue(t *testing.T) {
	due := selectDue(nil, nil, 5*time.Minute, time.Now())
	if len(due) != 0 {
		t.Errorf("expected empty due list, got %d", len(due))
	}
}
