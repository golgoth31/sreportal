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

// Package chain contains Chain-of-Responsibility handlers for the DeployStatus controller.
package chain

import (
	"github.com/golgoth31/sreportal/internal/domain/forge"
)

// ChainData is the typed shared state passed between DeployStatus handlers.
type ChainData struct {
	// Due are the entries selected for a forge check this cycle.
	Due []WorkItem
	// Computed holds the resulting projection entries to write to the readstore + CR.
	Computed []ComputedEntry
}

// WorkItem is one service entry to evaluate this cycle.
type WorkItem struct {
	Key         string
	Image       string
	// Workload holds the resolved forge repo ref (zero until ResolveOCISourceHandler).
	Workload    forge.RepoRef
	// workload identity fields from the CRD spec:
	WorkloadKind      string
	WorkloadNamespace string
	WorkloadName      string
	WorkloadContainer string
	// SourceURL is the OCI org.opencontainers.image.source label value.
	SourceURL   string
	// DeployedRef is the OCI revision label value (commit SHA or tag).
	DeployedRef string
}

// ComputedEntry is the per-service result produced by the forge-compare step.
type ComputedEntry struct {
	Key           string
	Image         string
	SourceRepo    string
	DeployedRef   string
	DefaultBranch string
	AheadBy       int
	PendingCommits []forge.Commit
	PendingTrunc  bool
	DeployRunURL  string
	State         string // ok | behind | unresolved | error
	Error         string
}
