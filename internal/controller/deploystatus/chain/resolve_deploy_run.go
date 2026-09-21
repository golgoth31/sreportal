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

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ResolveDeployRunHandler enriches computed entries with a best-effort deploy
// workflow run URL. Errors are swallowed — the link is optional.
type ResolveDeployRunHandler struct {
	clientFor      func(host string) forge.Client
	workflowByHost map[string]string // host -> deployWorkflow filename
}

// NewResolveDeployRunHandler constructs a ResolveDeployRunHandler.
func NewResolveDeployRunHandler(clientFor func(host string) forge.Client, forges []config.ForgeConfig) *ResolveDeployRunHandler {
	wf := make(map[string]string, len(forges))
	for _, f := range forges {
		wf[f.Host] = f.DeployWorkflow
	}
	return &ResolveDeployRunHandler{clientFor: clientFor, workflowByHost: wf}
}

// Handle implements reconciler.Handler.
func (h *ResolveDeployRunHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]) error {
	logger := log.FromContext(ctx).WithName("resolve-deploy-run")

	// Build a key→RepoRef index from the Due set (only resolved items have a non-zero ref).
	repoByKey := make(map[string]forge.RepoRef, len(rc.Data.Due))
	for _, wi := range rc.Data.Due {
		repoByKey[wi.Key] = wi.Workload
	}

	for i := range rc.Data.Computed {
		e := &rc.Data.Computed[i]
		// Only enrich successfully compared entries.
		if e.State == stateError || e.State == stateUnresolved {
			continue
		}
		ref, ok := repoByKey[e.Key]
		if !ok {
			continue
		}
		wf := h.workflowByHost[ref.Host]
		// Best-effort: the link is optional, so a failed resolution must not fail
		// the chain. Surface it at V(1) so a systematically broken workflow
		// resolution is visible without flooding default logs.
		url, err := h.clientFor(ref.Host).LatestWorkflowRun(ctx, ref, wf, e.DefaultBranch)
		if err != nil {
			logger.V(1).Info("resolve deploy run failed (best-effort)",
				"key", e.Key, "host", ref.Host, "workflow", wf, "error", err.Error())
		}
		e.DeployRunURL = url
	}
	return nil
}
