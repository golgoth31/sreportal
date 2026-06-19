# Deploy Status — per-service deployment lag from cluster truth + GitHub source

- **Date**: 2026-06-18
- **Status**: Design — pending implementation plan
- **Scope**: New `deployStatus` portal feature: proto, `DeployStatus` CRD (controller-managed), controller, domain + git-forge port, git-forge client, readstore, Connect service, MCP server, validating webhook, React page, remote-portal federation handler, operator config for the forge token/host/deploy-workflow.
- **Out of scope**: The existing `release` feature (push-based release log — unchanged); any write path to the git forge (read-only); deploying anything (we only link to the deploy workflow run); registry credential handling (reuses imageInventory's existing registry access).
- **Genericity constraint**: SRE Portal is a generic, vendor-agnostic operator — **nothing in this feature may hardcode Agicap-specific conventions** (org names, repo lists, workflow file names, secret backends). Everything organisation-specific is configuration with sensible generic defaults.
- **Related**: imageInventory / `ImageRegistry` (`api/v1alpha1/imageregistry_types.go`, `internal/controller/imageinventory/`) — the upstream source of observed deployed images this feature builds on.

## 1. Motivation

The team needs an at-a-glance view of **what is pending deployment**: for each running service, which commits on the default branch are newer than what is actually live, and a one-click link to the workflow run that gates the prod deploy.

A standalone reference implementation exists (`AgicapTech/di-business-meta-app/scripts/deploy-dashboard`): a zero-dependency Node generator that, per repo, takes the **latest git tag as a proxy for prod** and computes `ahead_by` of `tag...defaultBranch`. Its core limitation is that it has **no cluster access** — the latest tag is only a proxy, and the tracked repo list is a hardcoded `REPOS` constant.

SRE Portal is a cluster operator and already knows the **truth**: `imageInventory` observes the image tag actually running on every workload (`ImageRegistry` CR, with `repository`, `originalTag`, `tagType ∈ {semver,commit,digest,latest,other}`). So instead of a tag proxy and a hardcoded list, this feature:

- derives the **deployed version from cluster truth** (reuse `ImageRegistry` observations),
- resolves each deployed image to its **git source via OCI labels** (`org.opencontainers.image.source` / `.revision`),
- computes the lag with a **GitHub compare `deployedRef...defaultBranch`**,
- discovers the **service list from the cluster**, not a static constant.

This is distinct from the existing `release` feature, which is a **push-based historical log** (`AddRelease`, day-grouped, TTL). Deploy Status is **pull-based current state**.

## 2. Architecture & data flow

Follows the established `imageregistry` conventions: controller-managed CR + readstore, chain-of-handlers controller, `isDue`-paced polling, proto → Connect → MCP → React, gated by a `Portal` feature flag.

```
ImageRegistry CR (existing) ── observed images per (portalRef, host, namespace)
   │  per image: mutatedImage/originalImage, originalTag, tagType, workloads
   ▼
[controller: deploystatus]  (chain-of-handlers, isDue-paced like imageregistry)
   1. select_due        — pick entries whose lastCheckedAt is due (rate-limit pacing)
   2. resolve_oci_source — read image manifest labels (reuse imageInventory registry client):
                            org.opencontainers.image.source   → sourceRepo (repo URL)
                            org.opencontainers.image.revision → deployedRef (commit SHA)
                            fallback: if no .revision and originalTag is semver → deployedRef = originalTag (git tag)
   3. github_compare    — read default branch dynamically, then compare deployedRef...defaultBranch:
                            aheadBy, pendingCommits (merge commits filtered, cap 50), deployedAt (date of deployed commit)
   4. resolve_deploy_run — latest run of the *configured* deploy workflow on the default branch (best-effort);
                            no workflow configured / not resolvable → fall back to the repo Actions page filtered on the default branch
   5. update_readstore  — write computed entries into the readstore
   6. update_status     — patch DeployStatus CR spec/status + per-entry lastCheckedAt
   ▼
DeployStatus CR (controller-managed)  +  readstore (serves the API)
   ▼
DeployStatusService (Connect/gRPC) ──► MCP server (/mcp/deploystatus) ──► React "Deploy Status" page
   gated by Portal.spec.features.deployStatus (IsDeployStatusEnabled, default true)
```

### Entry state machine

`state` per service entry:

- `ok` — `aheadBy == 0` (live with the default branch tip).
- `behind` — `aheadBy > 0` (pending commits exist).
- `unresolved` — no OCI source label and no usable semver tag → repo cannot be determined. **No repo is invented** (per the "never fabricate facts" rule).
- `error` — manifest read / compare / network failure for this entry. Does **not** fail the reconcile; surfaced as an `error` card.

## 3. The `DeployStatus` CRD (controller-managed)

Mirrors `ImageRegistry`: **not user-edited** in v1, one instance per `(portalRef, namespace)`, populated by the controller.

```go
// DeployStatusSpec is controller-managed — derived from ImageRegistry observations.
type DeployStatusSpec struct {
    // portalRef is the Portal name this deploy status is derived from.
    PortalRef string `json:"portalRef"`
    // namespace is the Kubernetes namespace observed (may differ from the CR's own namespace).
    Namespace string `json:"namespace"`
    // isRemote marks a shadow CR (`remote-<portal>`) whose entries are fetched from a
    // remote portal's DeployStatusService rather than computed locally (federation, §6).
    // +optional
    IsRemote bool `json:"isRemote,omitempty"`
    // services is the list of per-workload deploy status entries.
    // +listType=map
    // +listMapKey=key
    Services []DeployStatusEntry `json:"services,omitempty"`
}

type DeployStatusEntry struct {
    // key is sha256(image|workloadKind|workloadNamespace|workloadName|container)[:16] — stable patch key.
    Key string `json:"key"`
    // workload identifies the workload+container running the image
    // (kind/namespace/name/container, mirroring ImageRegistryWorkloadRef).
    Workload DeployStatusWorkloadRef `json:"workload"`
    // image is the deployed image reference observed on the running Pod.
    Image string `json:"image"`
    // sourceRepo is the git repo URL from the OCI source label. Empty when unresolved.
    SourceRepo string `json:"sourceRepo,omitempty"`
    // deployedRef is the deployed commit SHA (OCI revision label) or the git tag (semver fallback).
    DeployedRef string `json:"deployedRef,omitempty"`
    // defaultBranch is the repo's default branch, read dynamically.
    DefaultBranch string `json:"defaultBranch,omitempty"`
    // aheadBy is the number of commits the default branch is ahead of deployedRef.
    AheadBy int `json:"aheadBy,omitempty"`
    // pendingCommits lists the commits not yet deployed (merge commits filtered, capped at 50).
    PendingCommits []DeployStatusCommit `json:"pendingCommits,omitempty"`
    // pendingTruncated is true when more than 50 commits are pending (UI shows a "full diff" link).
    PendingTruncated bool `json:"pendingTruncated,omitempty"`
    // deployedAt is the commit date of the deployed ref (proxy — not the real deploy time).
    DeployedAt metav1.Time `json:"deployedAt,omitempty"`
    // deployRunURL links to the deploy workflow run gating prod (best-effort; falls back to filtered Actions page).
    DeployRunURL string `json:"deployRunUrl,omitempty"`
    // state is ok | behind | unresolved | error.
    // +kubebuilder:validation:Enum=ok;behind;unresolved;error
    State string `json:"state"`
    // error carries the last per-entry error message (set when state=error).
    Error string `json:"error,omitempty"`
    // lastCheckedAt paces re-checks (isDue); set on every attempt, success or error.
    LastCheckedAt metav1.Time `json:"lastCheckedAt,omitempty"`
}

type DeployStatusCommit struct {
    // sha is the commit SHA.
    Sha string `json:"sha"`
    // message is the commit message first line.
    Message string `json:"message"`
    // author is the commit author.
    Author string `json:"author,omitempty"`
    // date is the commit date.
    Date metav1.Time `json:"date,omitempty"`
    // url links to the commit on GitHub.
    URL string `json:"url,omitempty"`
}
```

`DeployStatusStatus` carries `observedGeneration`, `lastError`, `serviceCount`, and `conditions` (same shape as `ImageRegistryStatus`).

Printer columns: `Portal` (`.spec.portalRef`), `Namespace`, `Services` (`.status.serviceCount`), `Age`.

## 4. Portal feature flag

Add to `PortalFeatures` (`api/v1alpha1/portal_types.go`), mirroring the five existing flags:

```go
// deployStatus enables the deploy status page (per-service deployment lag) for this portal.
// +optional
// +kubebuilder:default=true
DeployStatus *bool `json:"deployStatus,omitempty"`
```

```go
// IsDeployStatusEnabled returns true if the deploy status feature is enabled (nil-safe, defaults to true).
func (f *PortalFeatures) IsDeployStatusEnabled() bool {
    return f == nil || f.DeployStatus == nil || *f.DeployStatus
}
```

The Connect service and MCP server gate on this flag via the existing `internal/grpc/feature_gate.go` pattern. The Helm chart template (`templates/portal.yaml`) and `values.yaml` `portals.features` block gain a `deployStatus` entry.

## 5. Components to create

Layout mirrors `imageregistry` / `imageinventory`.

| Layer | Files |
|---|---|
| proto | `proto/sreportal/v1/deploystatus.proto` — `DeployStatusService.ListDeployStatus(ListDeployStatusRequest) returns (ListDeployStatusResponse)`; messages `DeployStatusEntry`, `DeployStatusCommit`. Request filters by `portal` (defaults to `main`). |
| CRD | `api/v1alpha1/deploystatus_types.go`; `DeployStatus` flag + `IsDeployStatusEnabled()` in `portal_types.go` |
| controller | `internal/controller/deploystatus/deploystatus_controller.go` + `chain/{select_due,resolve_oci_source,github_compare,resolve_deploy_run,update_readstore,update_status,handlers}.go` |
| domain | `internal/domain/deploystatus/{read_model,reader,writer}.go`; **forge-agnostic port** `internal/domain/forge/port.go` (interface: `DefaultBranch`, `Compare`, `LatestWorkflowRun`) |
| client | `internal/forgeclient/github/client.go` — GitHub implementation of the forge port; retry/backoff on 429/5xx, **no retry on 4xx** (mirrors `internal/alertmanagerclient`) |
| readstore | `internal/readstore/deploystatus/store.go` |
| grpc | `internal/grpc/deploystatus_service.go` (+ feature_gate wiring) |
| mcp | `internal/mcp/deploystatus_server.go` (mount `/mcp/deploystatus`) |
| webhook | `internal/webhook/v1alpha1/deploystatus_webhook.go` (validate controller-managed invariants) |
| federation | `internal/controller/portal/chain/sync_remote_deploy_status.go` (shadow `remote-<portal>` CR, see §6) |
| UI | React "Deploy Status" page + sidebar entry, gated by the feature flag (follows the existing image-inventory page) |
| config | `DeployStatusConfig`/`ForgeConfig` in `internal/config/types.go` (+ loader/validation); token values read via `os.Getenv(TokenEnv)` in `main.go`; `deployStatus.forges` surfaced in Helm `values.yaml` with per-`TokenEnv` `secretKeyRef` (see §7) |
| RBAC / CRD manifests | `config/crd`, `config/rbac` regenerated via `make manifests generate` |

**OCI source resolution** reuses the registry manifest access already implemented for imageInventory's remote lookups (`internal/controller/imageinventory/chain/fetch_remote_images.go`) — no new registry client. Only the **label extraction** (`org.opencontainers.image.source` / `.revision`) is new.

## 6. Remote federation

Deploy Status federates across clusters exactly like the other federatable features. The portal controller chain already has one `sync_remote_<feature>.go` handler per feature (`sync_remote_image_inventory.go`, `sync_remote_dns.go`, `sync_remote_alertmanager.go`, `sync_remote_network_flows.go`); each creates a shadow `remote-<portal>` CR with `IsRemote=true` so the feature's own controller fetches the remote portal's data and projects it into the local readstore.

- **New handler** `internal/controller/portal/chain/sync_remote_deploy_status.go`: for each remote portal, create/update a shadow `DeployStatus` CR named `remote-<portal>` with `IsRemote=true`. No-op for local portals or when `IsDeployStatusEnabled()` is false (mirrors `SyncRemoteImageInventoryHandler`).
- **Controller branch on `IsRemote`**: a remote CR skips the local compute chain (no OCI/forge calls). Instead it uses the existing remote-client plumbing (`build_remote_client.go` / `fetch_remote_data.go` / `health_check_remote.go`) to call the remote portal's `DeployStatusService.ListDeployStatus` and project the returned entries into the local readstore unchanged.
- **No extra config**: federation reuses the `Portal.spec.remote` connection already defined for the other features; the deploy-status data simply rides the same channel.

## 7. Git-forge access & credentials (operator-level)

Credentials follow the operator's established secret convention (`SlackEmojiConfig`, `internal/config/types.go:115`): **the config file carries non-secret structure/toggles; the secret value is read from a named environment variable** via `os.Getenv` in `main.go`. No secret ever lives in the config file or in a CR — only the *name* of the env var. The domain dependency is a **forge-agnostic port**; v1 ships a single **GitHub implementation** (github.com + GitHub Enterprise via configurable host) — a GitLab/Gitea client can be added later behind the same port.

New section in `OperatorConfig` (`internal/config/types.go`):

```go
type DeployStatusConfig struct {
    // Enabled toggles the feature at operator level.
    Enabled bool `json:"enabled" yaml:"enabled"`
    // RefreshInterval paces per-entry re-checks (isDue). Default e.g. 5m.
    RefreshInterval Duration `json:"refreshInterval,omitempty" yaml:"refreshInterval,omitempty"`
    // Forges lists the configured git forges. The OCI source label host is matched
    // against Forge.Host; no match -> entry state "unresolved".
    Forges []ForgeConfig `json:"forges,omitempty" yaml:"forges,omitempty"`
}

type ForgeConfig struct {
    // Host matched against the OCI source label, e.g. "github.com", "ghe.example.com".
    Host string `json:"host" yaml:"host"`
    // Kind selects the forge client implementation. v1: "github".
    Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`
    // Auth configures how the forge token is obtained (exactly one mode).
    Auth ForgeAuthConfig `json:"auth" yaml:"auth"`
    // DeployWorkflow is the workflow file to resolve for the "deploy to prod" link
    // (empty -> fall back to the CI/Actions page filtered on the default branch).
    DeployWorkflow string `json:"deployWorkflow,omitempty" yaml:"deployWorkflow,omitempty"`
}

// ForgeAuthConfig selects exactly one authentication mode. All secret VALUES are
// read from named env vars via os.Getenv — never stored in config or a CR.
type ForgeAuthConfig struct {
    // TokenEnv names the env var holding a (fine-grained) PAT. Set this XOR App.
    TokenEnv string `json:"tokenEnv,omitempty" yaml:"tokenEnv,omitempty"`
    // App configures GitHub App authentication. Set this XOR TokenEnv.
    App *GitHubAppConfig `json:"app,omitempty" yaml:"app,omitempty"`
}

// GitHubAppConfig authenticates as a GitHub App installation: the client signs a
// short-lived App JWT (RS256) and exchanges it for an installation access token
// (~1h TTL), cached and refreshed before expiry.
type GitHubAppConfig struct {
    // AppID is the GitHub App's numeric ID.
    AppID int64 `json:"appID" yaml:"appID"`
    // InstallationID is the installation whose token is minted.
    InstallationID int64 `json:"installationID" yaml:"installationID"`
    // PrivateKeyEnv names the env var holding the App private key (PEM).
    PrivateKeyEnv string `json:"privateKeyEnv" yaml:"privateKeyEnv"`
}
```

- **Two auth modes** (exactly one per forge): a **fine-grained PAT** (`auth.tokenEnv` → static bearer) or a **GitHub App installation** (`auth.app` → the client builds an App JWT signed RS256 with the PEM from `privateKeyEnv`, POSTs to `/app/installations/{installationID}/access_tokens`, and caches the resulting short-lived installation token, refreshing it before the `expires_at` returned by GitHub). Both paths use only the Go stdlib (`crypto/rsa`, `crypto/x509`, `encoding/pem`, `encoding/json`, `encoding/base64`) — the client stays zero-dependency, consistent with `internal/alertmanagerclient`.
- **Endpoint matching**: for each resolved OCI source URL, the controller picks the `ForgeConfig` whose `Host` matches the URL host; the client's token source provides the bearer (static PAT or freshly-minted installation token). **Mono-forge** = one entry; **multi-org / GHES** = several entries — zero hardcoding.
- **Required forge scopes**: **Contents: Read** (tags, compare, default branch) + **Actions/CI: Read** (resolve the deploy-gate run; without it the deploy link falls back to the filtered CI page).
- **No hardcoded repo list**: the service set is derived entirely from cluster observations — the key improvement over the reference's hardcoded `REPOS` constant, and what keeps the feature organisation-agnostic.
- **Unmatched source host** → entry `unresolved` (no call, nothing fabricated). **Token env empty / installation-token mint fails / forge unreachable** → entry `error`. **No forge configured at all** → the feature disables cleanly (no reconcile crash).
- **Helm**: `values.yaml` exposes `deployStatus.forges`; the controller-manager Deployment maps each referenced env var (PAT `tokenEnv`, or App `privateKeyEnv`) from a `secretKeyRef`. The Secret backend (Vault, External Secrets, plain Secret) is the operator's concern, out of scope here.

## 8. Robustness

- `isDue`/`lastCheckedAt` pacing per entry to respect the GitHub rate limit (same pattern as `imageregistry`'s `select_due_images`).
- Per-entry isolation: a failing service becomes an `error` card; the reconcile of other entries proceeds and the published CR is not wiped.
- Retry + exponential backoff on network errors / 429 / 5xx; 4xx are not retried.
- Merge commits filtered from `pendingCommits`; list capped at 50 with `pendingTruncated=true` driving a "full diff" link in the UI.
- `deployedAt` is documented as a **proxy** (deployed commit date), not the real deploy timestamp.

## 9. Tests

Per-layer unit tests (the repo's `*_test.go` convention), each isolated:

- **OCI label parsing** (`resolve_oci_source`): source/revision present; missing labels → `unresolved`; semver-tag fallback path.
- **Lag computation** (`github_compare` / domain): `aheadBy` math, merge-commit filtering, 50-cap + `pendingTruncated`, `state` mapping (`ok`/`behind`/`unresolved`/`error`).
- **Forge client** (`internal/forgeclient/github`): retry on 429/5xx, no-retry on 4xx, default-branch resolution, best-effort run resolution + configurable-workflow / CI-page fallback.
- **Credentials config** (`internal/config` + controller): endpoint matching by source-URL host → correct `ForgeConfig`/token env; unmatched host → `unresolved`; empty token env → `error`; config loader validation of `forges`.
- **Feature gate**: service/MCP return disabled when `deployStatus=false`; default-true when unset.
- **Webhook**: rejects user edits / enforces controller-managed invariants.
- **Controller chain**: due selection pacing; per-entry error isolation does not fail the reconcile.
- **Federation**: `sync_remote_deploy_status` creates the shadow CR only for remote portals with the feature enabled; `IsRemote` CRs project remote entries without any forge call.

## 10. Open questions / decisions taken

- **Persistence**: decided — controller-managed `DeployStatus` CR + readstore (survives restarts, observable via `kubectl`), consistent with `ImageRegistry`. Rejected: in-memory-only readstore.
- **Deployed-version source**: decided — cluster truth via imageInventory. Rejected: forge-tag proxy (reference's approach) and explicit hybrid.
- **Image→git mapping**: decided — OCI image labels, with a **semver-tag fallback** when no `revision` label is present (compare the git tag matching the image tag against the default branch). Rejected: workload annotations and a dedicated mapping CRD.
- **Forge credentials**: decided — a configurable list of forge endpoints (`forges: [{host, kind, auth, deployWorkflow}]`), matched by the OCI source-URL host; secret values read from named env vars via `os.Getenv` (mirrors `SlackEmojiConfig`), never stored in config or a CR. Rejected: a single global token, and a global-token-plus-overrides variant.
- **GitHub auth modes**: decided — each forge uses exactly one of: a **fine-grained PAT** (`auth.tokenEnv`), or a **GitHub App installation** (`auth.app` = appID + installationID + privateKeyEnv), where the client mints/caches/refreshes short-lived installation tokens via a stdlib-signed RS256 App JWT. Both stay zero-dependency.
- **Genericity**: decided — no Agicap-specific coupling. Forge host, deploy-workflow, and token source are all configuration with generic defaults; the service list comes from the cluster, not a constant. Forge access sits behind a forge-agnostic port (GitHub impl in v1).
- **Federation**: decided — Deploy Status federates across clusters via the same remote-portal mechanism as the other features.
- **Feature name**: `deployStatus` (consistent with `statusPage`, `imageInventory`).
