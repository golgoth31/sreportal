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

	"github.com/golgoth31/sreportal/internal/domain/forge"
)

func TestComputeLag_FiltersMergesAndCaps(t *testing.T) {
	commits := make([]forge.Commit, 0, 60)
	// one merge commit that must be filtered out
	commits = append(commits, forge.Commit{SHA: "m", Merge: true})
	// 55 regular commits — after filtering 55 remain, capped to 50
	for range 55 {
		commits = append(commits, forge.Commit{SHA: "c"})
	}
	cr := forge.CompareResult{AheadBy: 55, Commits: commits}

	pending, trunc := ComputeLag(cr)
	if len(pending) != 50 {
		t.Fatalf("pending = %d, want 50 (cap)", len(pending))
	}
	if !trunc {
		t.Error("expected truncated = true")
	}
	for _, c := range pending {
		if c.SHA == "m" {
			t.Error("merge commit should be filtered out")
		}
	}
}

func TestComputeLag_NoTruncationWhenUnder50(t *testing.T) {
	commits := []forge.Commit{
		{SHA: "a"},
		{SHA: "b", Merge: true}, // filtered
		{SHA: "c"},
	}
	cr := forge.CompareResult{AheadBy: 2, Commits: commits}

	pending, trunc := ComputeLag(cr)
	if len(pending) != 2 {
		t.Fatalf("pending = %d, want 2", len(pending))
	}
	if trunc {
		t.Error("expected truncated = false")
	}
}

func TestStateFor(t *testing.T) {
	if StateFor(0) != "ok" {
		t.Errorf("aheadBy 0 -> ok, got %q", StateFor(0))
	}
	if StateFor(3) != stateBehind {
		t.Errorf("aheadBy 3 -> behind, got %q", StateFor(3))
	}
	if StateFor(1) != stateBehind {
		t.Errorf("aheadBy 1 -> behind, got %q", StateFor(1))
	}
}
