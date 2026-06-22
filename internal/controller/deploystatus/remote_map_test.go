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

package deploystatus

import (
	"testing"
	"time"

	"github.com/golgoth31/sreportal/internal/remoteclient"
)

func TestMapRemoteEntry_FullFields(t *testing.T) {
	deployedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	lastChecked := time.Date(2026, 6, 20, 10, 5, 0, 0, time.UTC)
	commitDate := time.Date(2026, 6, 19, 9, 0, 0, 0, time.UTC)

	re := remoteclient.RemoteDeployStatusEntry{
		Key: "k1",
		Workload: remoteclient.RemoteDeployWorkload{
			Kind: "Deployment", Namespace: "prod", Name: "widget", Container: "app",
		},
		Image:         "github.com/acme/widget@sha256:abc",
		SourceRepo:    "https://github.com/acme/widget",
		DeployedRef:   "deadbeef",
		DefaultBranch: "main",
		AheadBy:       2,
		PendingCommits: []remoteclient.RemoteDeployCommit{
			{Sha: "c1", Message: "feat: one", Author: "alice", Date: &commitDate, URL: "https://x/c1"},
		},
		PendingTruncated: true,
		DeployedAt:       &deployedAt,
		DeployRunURL:     "https://github.com/acme/widget/actions/runs/1",
		State:            "behind",
		Error:            "",
		LastCheckedAt:    &lastChecked,
	}

	got := mapRemoteEntry(re)

	if got.Key != "k1" {
		t.Errorf("Key = %q, want k1", got.Key)
	}
	if got.Workload.Kind != "Deployment" || got.Workload.Namespace != "prod" ||
		got.Workload.Name != "widget" || got.Workload.Container != "app" {
		t.Errorf("Workload mismatch: %+v", got.Workload)
	}
	if got.AheadBy != 2 {
		t.Errorf("AheadBy = %d, want 2", got.AheadBy)
	}
	if !got.PendingTruncated {
		t.Error("PendingTruncated = false, want true")
	}
	if len(got.PendingCommits) != 1 {
		t.Fatalf("PendingCommits len = %d, want 1", len(got.PendingCommits))
	}
	if got.PendingCommits[0].Sha != "c1" || !got.PendingCommits[0].Date.Equal(commitDate) {
		t.Errorf("commit mismatch: %+v", got.PendingCommits[0])
	}
	if !got.DeployedAt.Equal(deployedAt) {
		t.Errorf("DeployedAt = %v, want %v", got.DeployedAt, deployedAt)
	}
	if !got.LastCheckedAt.Equal(lastChecked) {
		t.Errorf("LastCheckedAt = %v, want %v", got.LastCheckedAt, lastChecked)
	}
	if got.State != "behind" {
		t.Errorf("State = %q, want behind", got.State)
	}
}

func TestMapRemoteEntry_NilTimestampsZeroValue(t *testing.T) {
	re := remoteclient.RemoteDeployStatusEntry{
		Key:   "k2",
		State: "ok",
	}

	got := mapRemoteEntry(re)

	if !got.DeployedAt.IsZero() {
		t.Errorf("DeployedAt = %v, want zero", got.DeployedAt)
	}
	if !got.LastCheckedAt.IsZero() {
		t.Errorf("LastCheckedAt = %v, want zero", got.LastCheckedAt)
	}
	if len(got.PendingCommits) != 0 {
		t.Errorf("PendingCommits len = %d, want 0", len(got.PendingCommits))
	}
}
