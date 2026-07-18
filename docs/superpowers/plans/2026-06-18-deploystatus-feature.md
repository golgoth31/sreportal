# Deploy Status Feature — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `deployStatus` portal feature that shows, per running service, the commits on its default branch not yet deployed (deployment lag) plus a best-effort link to the deploy-gate workflow run — computed from cluster truth (imageInventory) joined to the git source via OCI labels.

**Architecture:** Mirrors the existing `imageregistry`/`imageinventory` feature exactly: a controller-managed `DeployStatus` CRD populated by a Chain-of-Responsibility controller, served from an in-memory readstore through a Connect/gRPC service + MCP server + React page, gated by a `Portal` feature flag, federated across portals via a shadow `remote-<portal>` CR. Git-forge access sits behind a forge-agnostic port with a single GitHub implementation in v1; credentials follow the operator's `SlackEmojiConfig` convention (config holds a `forges[]` list, each with an `auth` mode — fine-grained PAT or GitHub App installation; all secret values read via `os.Getenv`, the App path minting/caching short-lived installation tokens via a stdlib-signed RS256 JWT).

**Tech Stack:** Go 1.26, Kubebuilder, controller-runtime v0.23, Connect (Buf codegen), Echo v5, MCP (mark3labs/mcp-go), React 19 + Vite + TanStack Query, Ginkgo v2 + Gomega + envtest, `go-github` (or raw REST) for the forge client.

**Spec:** [[2026-06-18-deploystatus-feature-design]]

**Repo workflow (from `CLAUDE.md`) this plan obeys:**
- Use the **kubebuilder CLI** to scaffold the API and webhook — never hand-create those files.
- **Never edit** generated files: `config/crd/bases/*`, `config/rbac/role.yaml`, `config/webhook/manifests.yaml`, `**/zz_generated.*.go`, `PROJECT`, `internal/grpc/gen/*`, `web/src/gen/*`. Regenerate them.
- After editing `*_types.go` or proto: `make generate-all` (`manifests generate proto`), then `make helm` and `make doc`.
- Verification gates: `make helm` → `make test` → `make lint` → `make doc`.
- Tests are **Ginkgo + Gomega with envtest**; pure-logic packages may use plain `testing` (the existing handlers do both — match the neighbouring package).

**Phasing (each phase ends green and is independently testable):**
- Phase 0 — Scaffolding (CRD + proto + feature flag) → `make generate-all` clean, CRD installs.
- Phase 1 — Config (`DeployStatusConfig`/`ForgeConfig`) → loader tests pass.
- Phase 2 — Forge port + token sources (PAT / GitHub App) + GitHub client → unit tests pass.
- Phase 3 — Domain + readstore → unit tests pass.
- Phase 4 — Controller chain handlers → unit + envtest pass.
- Phase 5 — Controller wiring (main.go, RBAC, field index) → operator reconciles in envtest.
- Phase 6 — gRPC service + feature gate.
- Phase 7 — MCP server.
- Phase 8 — Webhook.
- Phase 9 — Federation.
- Phase 10 — React UI.
- Phase 11 — Helm values.
- Phase 12 — Full verification + docs.

---

## Phase 0 — Scaffolding: CRD, feature flag, proto

### Task 0.1: Scaffold the `DeployStatus` API with kubebuilder

**Files:**
- Create (by CLI): `api/v1alpha1/deploystatus_types.go`, `internal/controller/deploystatus/deploystatus_controller.go` (placeholder), updates to `PROJECT`, `cmd/main.go` scaffold markers.

- [ ] **Step 1: Run kubebuilder to scaffold the API (no controller yet — we build the chain controller manually under `internal/`)**

```bash
kubebuilder create api --group sreportal --version v1alpha1 --kind DeployStatus --resource --controller=false
```

Expected: new `api/v1alpha1/deploystatus_types.go`, `PROJECT` updated, `// +kubebuilder:scaffold:*` markers preserved.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: success (the scaffolded types build).

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "chore(deploystatus): scaffold DeployStatus API via kubebuilder"
```

### Task 0.2: Define the `DeployStatus` types

**Files:**
- Modify: `api/v1alpha1/deploystatus_types.go`

- [ ] **Step 1: Replace the scaffolded spec/status with the design types**

```go
package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// DeployStatusSpec is controller-managed — derived from ImageRegistry observations.
type DeployStatusSpec struct {
	// portalRef is the Portal name this deploy status is derived from.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PortalRef string `json:"portalRef"`

	// namespace is the Kubernetes namespace observed (may differ from the CR's own namespace).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`

	// isRemote marks a shadow CR (`remote-<portal>`) whose entries are fetched from a
	// remote portal's DeployStatusService rather than computed locally (federation).
	// +optional
	IsRemote bool `json:"isRemote,omitempty"`

	// services is the list of per-workload deploy status entries.
	// +listType=map
	// +listMapKey=key
	// +optional
	Services []DeployStatusEntry `json:"services,omitempty"`
}

// DeployStatusEntry is one observed workload's deploy status.
type DeployStatusEntry struct {
	// key is sha256(image|workloadKind|workloadNamespace|workloadName|container)[:16].
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`

	// workload identifies the workload+container running the image.
	// +kubebuilder:validation:Required
	Workload DeployStatusWorkloadRef `json:"workload"`

	// image is the deployed image reference observed on the running Pod.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// sourceRepo is the git repo URL from the OCI source label. Empty when unresolved.
	// +optional
	SourceRepo string `json:"sourceRepo,omitempty"`

	// deployedRef is the deployed commit SHA (OCI revision label) or git tag (semver fallback).
	// +optional
	DeployedRef string `json:"deployedRef,omitempty"`

	// defaultBranch is the repo's default branch, read dynamically.
	// +optional
	DefaultBranch string `json:"defaultBranch,omitempty"`

	// aheadBy is the number of commits the default branch is ahead of deployedRef.
	// +optional
	AheadBy int `json:"aheadBy,omitempty"`

	// pendingCommits lists commits not yet deployed (merge commits filtered, capped at 50).
	// +optional
	PendingCommits []DeployStatusCommit `json:"pendingCommits,omitempty"`

	// pendingTruncated is true when more than 50 commits are pending.
	// +optional
	PendingTruncated bool `json:"pendingTruncated,omitempty"`

	// deployedAt is the commit date of the deployed ref (proxy — not the real deploy time).
	// +optional
	DeployedAt metav1.Time `json:"deployedAt,omitempty"`

	// deployRunURL links to the deploy workflow run gating prod (best-effort).
	// +optional
	DeployRunURL string `json:"deployRunUrl,omitempty"`

	// state is ok | behind | unresolved | error.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ok;behind;unresolved;error
	State string `json:"state"`

	// error carries the last per-entry error message (set when state=error).
	// +optional
	Error string `json:"error,omitempty"`

	// lastCheckedAt paces re-checks (isDue); set on every attempt, success or error.
	// +optional
	LastCheckedAt metav1.Time `json:"lastCheckedAt,omitempty"`
}

// DeployStatusWorkloadRef identifies a workload+container (mirrors ImageRegistryWorkloadRef).
type DeployStatusWorkloadRef struct {
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Container string `json:"container"`
}

// DeployStatusCommit is one pending commit.
type DeployStatusCommit struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Sha string `json:"sha"`
	// +optional
	Message string `json:"message,omitempty"`
	// +optional
	Author string `json:"author,omitempty"`
	// +optional
	Date metav1.Time `json:"date,omitempty"`
	// +optional
	URL string `json:"url,omitempty"`
}

// DeployStatusStatus defines the observed state of DeployStatus.
type DeployStatusStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	LastError string `json:"lastError,omitempty"`
	// +optional
	ServiceCount int `json:"serviceCount,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

- [ ] **Step 2: Set the printer columns on the root type**

Ensure the `// +kubebuilder:object:root=true` block above `type DeployStatus struct` carries:

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Portal",type=string,JSONPath=`.spec.portalRef`
// +kubebuilder:printcolumn:name="Namespace",type=string,JSONPath=`.spec.namespace`
// +kubebuilder:printcolumn:name="Services",type=integer,JSONPath=`.status.serviceCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
```

- [ ] **Step 3: Regenerate deepcopy + CRD manifests**

Run: `make generate manifests`
Expected: `zz_generated.deepcopy.go` updated, `config/crd/bases/sreportal.io_deploystatuses.yaml` created. No manual edits to generated files.

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(deploystatus): define DeployStatus CRD types"
```

### Task 0.3: Add the `deployStatus` portal feature flag

**Files:**
- Modify: `api/v1alpha1/portal_types.go`

- [ ] **Step 1: Add the flag field to `PortalFeatures`** (after `ImageInventory`)

```go
	// deployStatus enables the deploy status page (per-service deployment lag) for this portal.
	// +optional
	// +kubebuilder:default=true
	DeployStatus *bool `json:"deployStatus,omitempty"`
```

- [ ] **Step 2: Add the nil-safe accessor** (after `IsImageInventoryEnabled`)

```go
// IsDeployStatusEnabled returns true if the deploy status feature is enabled (nil-safe, defaults to true).
func (f *PortalFeatures) IsDeployStatusEnabled() bool {
	return f == nil || f.DeployStatus == nil || *f.DeployStatus
}
```

- [ ] **Step 3: Write the accessor test** in `api/v1alpha1/portal_types_test.go` (mirror the existing `IsImageInventoryEnabled` test if present)

```go
func TestIsDeployStatusEnabled(t *testing.T) {
	tru, fls := true, false
	cases := map[string]struct {
		f    *PortalFeatures
		want bool
	}{
		"nil features":   {nil, true},
		"nil flag":       {&PortalFeatures{}, true},
		"explicit true":  {&PortalFeatures{DeployStatus: &tru}, true},
		"explicit false": {&PortalFeatures{DeployStatus: &fls}, false},
	}
	for name, tc := range cases {
		if got := tc.f.IsDeployStatusEnabled(); got != tc.want {
			t.Errorf("%s: got %v want %v", name, got, tc.want)
		}
	}
}
```

- [ ] **Step 4: Run the test + regenerate**

Run: `go test ./api/v1alpha1/... -run TestIsDeployStatusEnabled -v && make generate manifests`
Expected: PASS; CRD regenerated.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(deploystatus): add deployStatus portal feature flag"
```

### Task 0.4: Author the proto contract

**Files:**
- Create: `proto/sreportal/v1/deploystatus.proto`

- [ ] **Step 1: Write the proto** (model on `proto/sreportal/v1/image.proto`; `ListDeployStatus` only — the CR write path is the controller, not the API)

```proto
syntax = "proto3";

package sreportal.v1;

import "google/protobuf/timestamp.proto";

// DeployStatusService exposes per-service deployment lag for a portal.
service DeployStatusService {
  rpc ListDeployStatus(ListDeployStatusRequest) returns (ListDeployStatusResponse);
}

message ListDeployStatusRequest {
  // portal filters to a Portal metadata.name (defaults to "main" if unset).
  string portal = 1;
  // search optionally filters by service/repo substring.
  string search = 2;
  // state_filter optionally filters by state (ok|behind|unresolved|error).
  string state_filter = 3;
}

message ListDeployStatusResponse {
  repeated DeployStatusEntry entries = 1;
}

message DeployStatusEntry {
  string key = 1;
  WorkloadRef workload = 2;
  string image = 3;
  string source_repo = 4;
  string deployed_ref = 5;
  string default_branch = 6;
  int32 ahead_by = 7;
  repeated DeployStatusCommit pending_commits = 8;
  bool pending_truncated = 9;
  google.protobuf.Timestamp deployed_at = 10;
  string deploy_run_url = 11;
  string state = 12;
  string error = 13;
  google.protobuf.Timestamp last_checked_at = 14;
}

message DeployStatusCommit {
  string sha = 1;
  string message = 2;
  string author = 3;
  google.protobuf.Timestamp date = 4;
  string url = 5;
}

message WorkloadRef {
  string kind = 1;
  string namespace = 2;
  string name = 3;
  string container = 4;
}
```

> Note: if a `WorkloadRef` message already exists in another proto in the same package, import/reuse it instead of redeclaring (check `proto/sreportal/v1/image.proto`). If it collides, name this one `DeployWorkloadRef`.

- [ ] **Step 2: Generate Go + TS bindings**

Run: `make proto`
Expected: `internal/grpc/gen/sreportal/v1/deploystatus.pb.go` and `…connect/deploystatus.connect.go` created; `web/src/gen/...` TS created. (Generated — do not edit.)

- [ ] **Step 3: Verify Go + TS build**

Run: `go build ./... && (cd web && npx tsc -b)`
Expected: both succeed.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(deploystatus): add DeployStatusService proto + generated bindings"
```

---

## Phase 1 — Operator config (`DeployStatusConfig` / `ForgeConfig`)

### Task 1.1: Add config types

**Files:**
- Modify: `internal/config/types.go`

- [ ] **Step 1: Add the field to `OperatorConfig`**

```go
	DeployStatus *DeployStatusConfig `json:"deployStatus,omitempty" yaml:"deployStatus,omitempty"`
```

- [ ] **Step 2: Add the config types** (mirror `SlackEmojiConfig`'s "secret read from env" comment convention)

```go
// DeployStatusConfig configures the deploy-status feature. Token VALUES are never
// stored here — only the name of the env var holding each forge token (see ForgeConfig.TokenEnv).
type DeployStatusConfig struct {
	// Enabled toggles the feature at operator level.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// RefreshInterval paces per-entry re-checks (isDue). Default 5m when zero.
	RefreshInterval Duration `json:"refreshInterval,omitempty" yaml:"refreshInterval,omitempty"`
	// Forges lists the configured git forges. The OCI source-label host is matched
	// against Forge.Host; no match -> entry state "unresolved".
	Forges []ForgeConfig `json:"forges,omitempty" yaml:"forges,omitempty"`
}

// ForgeConfig configures one git forge endpoint.
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
	AppID          int64  `json:"appID" yaml:"appID"`
	InstallationID int64  `json:"installationID" yaml:"installationID"`
	// PrivateKeyEnv names the env var holding the App private key (PEM).
	PrivateKeyEnv  string `json:"privateKeyEnv" yaml:"privateKeyEnv"`
}
```

- [ ] **Step 3: Build**

Run: `go build ./internal/config/...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/config/types.go
git commit -m "feat(deploystatus): add DeployStatusConfig/ForgeConfig operator config"
```

### Task 1.2: Validate config in the loader

**Files:**
- Modify: `internal/config/loader.go`
- Test: `internal/config/loader_test.go`

- [ ] **Step 1: Write the failing validation test** (match the file's existing validation-test style; if it uses Ginkgo, use Ginkgo)

```go
func TestValidate_DeployStatus_RejectsForgeWithoutHost(t *testing.T) {
	cfg := &OperatorConfig{DeployStatus: &DeployStatusConfig{
		Enabled: true,
		Forges:  []ForgeConfig{{Host: "", Auth: ForgeAuthConfig{TokenEnv: "X"}}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for forge with empty host")
	}
}

func TestValidate_DeployStatus_RejectsNoAuthMode(t *testing.T) {
	cfg := &OperatorConfig{DeployStatus: &DeployStatusConfig{
		Enabled: true,
		Forges:  []ForgeConfig{{Host: "github.com", Auth: ForgeAuthConfig{}}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error: neither tokenEnv nor app set")
	}
}

func TestValidate_DeployStatus_RejectsBothAuthModes(t *testing.T) {
	cfg := &OperatorConfig{DeployStatus: &DeployStatusConfig{
		Enabled: true,
		Forges: []ForgeConfig{{Host: "github.com", Auth: ForgeAuthConfig{
			TokenEnv: "X", App: &GitHubAppConfig{AppID: 1, InstallationID: 2, PrivateKeyEnv: "K"},
		}}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error: both tokenEnv and app set")
	}
}

func TestValidate_DeployStatus_RejectsIncompleteApp(t *testing.T) {
	cfg := &OperatorConfig{DeployStatus: &DeployStatusConfig{
		Enabled: true,
		Forges: []ForgeConfig{{Host: "github.com", Auth: ForgeAuthConfig{
			App: &GitHubAppConfig{AppID: 1}, // missing installationID + privateKeyEnv
		}}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error: incomplete app config")
	}
}

func TestValidate_DeployStatus_AcceptsPAT(t *testing.T) {
	cfg := &OperatorConfig{DeployStatus: &DeployStatusConfig{
		Enabled: true,
		Forges:  []ForgeConfig{{Host: "github.com", Kind: "github", Auth: ForgeAuthConfig{TokenEnv: "GITHUB_TOKEN"}}},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_DeployStatus_AcceptsApp(t *testing.T) {
	cfg := &OperatorConfig{DeployStatus: &DeployStatusConfig{
		Enabled: true,
		Forges: []ForgeConfig{{Host: "github.com", Kind: "github", Auth: ForgeAuthConfig{
			App: &GitHubAppConfig{AppID: 1, InstallationID: 2, PrivateKeyEnv: "GH_APP_KEY"},
		}}},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

> If the loader's validation entry point is not `(*OperatorConfig).Validate()`, adapt the call to the existing one (grep `func.*Validate` in `internal/config/`). Reuse the existing error style in `internal/config/errors.go`.

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/config/... -run TestValidate_DeployStatus -v`
Expected: FAIL (no validation yet).

- [ ] **Step 3: Add the validation** to the existing validate path

```go
// inside the existing OperatorConfig validation, after other sections:
if c.DeployStatus != nil && c.DeployStatus.Enabled {
	for i, f := range c.DeployStatus.Forges {
		if f.Host == "" {
			return fmt.Errorf("deployStatus.forges[%d]: host is required", i)
		}
		if f.Kind != "" && f.Kind != "github" {
			return fmt.Errorf("deployStatus.forges[%d]: unsupported kind %q (v1 supports \"github\")", i, f.Kind)
		}
		hasPAT := f.Auth.TokenEnv != ""
		hasApp := f.Auth.App != nil
		switch {
		case hasPAT && hasApp:
			return fmt.Errorf("deployStatus.forges[%d]: set exactly one of auth.tokenEnv or auth.app, not both", i)
		case !hasPAT && !hasApp:
			return fmt.Errorf("deployStatus.forges[%d]: one of auth.tokenEnv or auth.app is required", i)
		case hasApp:
			a := f.Auth.App
			if a.AppID == 0 || a.InstallationID == 0 || a.PrivateKeyEnv == "" {
				return fmt.Errorf("deployStatus.forges[%d]: auth.app requires appID, installationID and privateKeyEnv", i)
			}
		}
	}
}
```

- [ ] **Step 4: Run to confirm pass**

Run: `go test ./internal/config/... -run TestValidate_DeployStatus -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/loader.go internal/config/loader_test.go
git commit -m "feat(deploystatus): validate forge config in loader"
```

---

## Phase 2 — Forge port + GitHub client

### Task 2.1: Define the forge-agnostic port

**Files:**
- Create: `internal/domain/forge/port.go`

- [ ] **Step 1: Write the port interface + value types**

```go
// Package forge defines the forge-agnostic interface the deploy-status controller
// depends on. v1 ships a single GitHub implementation in internal/forgeclient/github.
package forge

import (
	"context"
	"time"
)

// Client is the read-only forge interface used to compute deployment lag.
type Client interface {
	// DefaultBranch returns the repo's default branch name.
	DefaultBranch(ctx context.Context, owner, repo string) (string, error)
	// Compare returns how far `head` is ahead of `base` and the pending commits
	// (merge commits included — filtering/capping is the caller's concern).
	Compare(ctx context.Context, owner, repo, base, head string) (CompareResult, error)
	// LatestWorkflowRun returns the URL of the most recent run of `workflowFile`
	// on `branch`. Returns ("", nil) when not resolvable (caller falls back).
	LatestWorkflowRun(ctx context.Context, owner, repo, workflowFile, branch string) (string, error)
}

// CompareResult is the outcome of a base...head comparison.
type CompareResult struct {
	AheadBy int
	Commits []Commit
}

// Commit is one commit in a compare result.
type Commit struct {
	Sha     string
	Message string
	Author  string
	Date    time.Time
	URL     string
	IsMerge bool
}

// RepoRef identifies a repo parsed from an OCI source URL.
type RepoRef struct {
	Host  string
	Owner string
	Repo  string
}
```

- [ ] **Step 2: Build**

Run: `go build ./internal/domain/forge/...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/forge/port.go
git commit -m "feat(deploystatus): add forge-agnostic port"
```

### Task 2.2: Parse OCI source URL → RepoRef (pure function, TDD)

**Files:**
- Create: `internal/domain/forge/parse.go`
- Test: `internal/domain/forge/parse_test.go`

- [ ] **Step 1: Write the failing test**

```go
package forge

import "testing"

func TestParseSourceURL(t *testing.T) {
	cases := map[string]struct {
		in   string
		want RepoRef
		ok   bool
	}{
		"https":      {"https://github.com/golgoth31/sreportal", RepoRef{"github.com", "golgoth31", "sreportal"}, true},
		"with .git":  {"https://github.com/golgoth31/sreportal.git", RepoRef{"github.com", "golgoth31", "sreportal"}, true},
		"ssh scp":    {"git@github.com:golgoth31/sreportal.git", RepoRef{"github.com", "golgoth31", "sreportal"}, true},
		"ghe host":   {"https://ghe.example.com/team/svc", RepoRef{"ghe.example.com", "team", "svc"}, true},
		"empty":      {"", RepoRef{}, false},
		"no path":    {"https://github.com/", RepoRef{}, false},
	}
	for name, tc := range cases {
		got, err := ParseSourceURL(tc.in)
		if tc.ok && (err != nil || got != tc.want) {
			t.Errorf("%s: got %+v err=%v want %+v", name, got, err, tc.want)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/domain/forge/ -run TestParseSourceURL -v`
Expected: FAIL (undefined `ParseSourceURL`).

- [ ] **Step 3: Implement**

```go
package forge

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseSourceURL parses an OCI org.opencontainers.image.source value (https or
// scp-style ssh) into a RepoRef. Trailing ".git" and slashes are stripped.
func ParseSourceURL(raw string) (RepoRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return RepoRef{}, fmt.Errorf("empty source url")
	}
	// scp-style: git@host:owner/repo(.git)
	if !strings.Contains(raw, "://") && strings.Contains(raw, "@") && strings.Contains(raw, ":") {
		at := strings.Index(raw, "@")
		rest := raw[at+1:]
		colon := strings.Index(rest, ":")
		host := rest[:colon]
		path := strings.TrimSuffix(strings.Trim(rest[colon+1:], "/"), ".git")
		return splitOwnerRepo(host, path)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return RepoRef{}, err
	}
	path := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
	return splitOwnerRepo(u.Host, path)
}

func splitOwnerRepo(host, path string) (RepoRef, error) {
	parts := strings.Split(path, "/")
	if host == "" || len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return RepoRef{}, fmt.Errorf("cannot parse owner/repo from %q", path)
	}
	return RepoRef{Host: host, Owner: parts[0], Repo: parts[1]}, nil
}
```

- [ ] **Step 4: Run to confirm pass**

Run: `go test ./internal/domain/forge/ -run TestParseSourceURL -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/forge/parse.go internal/domain/forge/parse_test.go
git commit -m "feat(deploystatus): parse OCI source URL into RepoRef"
```

### Task 2.3: Token sources — PAT + GitHub App installation token (TDD, stdlib only)

**Files:**
- Create: `internal/forgeclient/github/token.go` (PAT + App JWT minting + installation-token cache)
- Test: `internal/forgeclient/github/token_test.go`

The client gets its bearer from a `TokenSource`. Two implementations: a static PAT, and a GitHub App source that signs an RS256 App JWT with the Go stdlib and exchanges it for a cached, auto-refreshing installation token.

- [ ] **Step 1: Write the failing test for the App installation token (httptest fakes the access_tokens endpoint; mint once, cache until near expiry)**

```go
package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))
}

func TestAppTokenSource_MintsAndCaches(t *testing.T) {
	var mints int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Must present a Bearer App JWT.
		if got := r.Header.Get("Authorization"); len(got) < 8 || got[:7] != "Bearer " {
			t.Errorf("missing bearer App JWT, got %q", got)
		}
		atomic.AddInt32(&mints, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"ghs_installation","expires_at":"` +
			time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + `"}`))
	}))
	defer srv.Close()

	ts, err := NewAppTokenSource(AppAuth{
		AppID: 1, InstallationID: 2, PrivateKeyPEM: testPEM(t),
		BaseURL: srv.URL, HTTPClient: srv.Client(),
		now: func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatal(err)
	}
	tok1, err := ts.Token(context.Background())
	if err != nil || tok1 != "ghs_installation" {
		t.Fatalf("tok1=%q err=%v", tok1, err)
	}
	tok2, _ := ts.Token(context.Background()) // cached, no second mint
	if tok2 != "ghs_installation" {
		t.Fatalf("tok2=%q", tok2)
	}
	if mints != 1 {
		t.Fatalf("expected 1 mint (cached), got %d", mints)
	}
}

func TestPATTokenSource_ReturnsStatic(t *testing.T) {
	ts := PATTokenSource("ghp_pat")
	got, err := ts.Token(context.Background())
	if err != nil || got != "ghp_pat" {
		t.Fatalf("got %q err %v", got, err)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/forgeclient/github/ -run 'TokenSource' -v`
Expected: FAIL (undefined symbols).

- [ ] **Step 3: Implement `token.go`** (RS256 JWT + installation-token cache, stdlib only)

```go
package github

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TokenSource yields a bearer token for forge requests.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// PATTokenSource is a static (fine-grained) PAT.
type PATTokenSource string

func (p PATTokenSource) Token(context.Context) (string, error) {
	if p == "" {
		return "", fmt.Errorf("github: empty PAT")
	}
	return string(p), nil
}

// AppAuth configures a GitHub App installation token source.
type AppAuth struct {
	AppID          int64
	InstallationID int64
	PrivateKeyPEM  string
	BaseURL        string // e.g. https://api.github.com (or GHES /api/v3)
	HTTPClient     *http.Client
	now            func() time.Time // injected for tests
}

// AppTokenSource mints and caches installation access tokens.
type AppTokenSource struct {
	auth AppAuth
	key  *rsa.PrivateKey

	mu      sync.Mutex
	token   string
	expires time.Time
}

// NewAppTokenSource parses the PEM and returns a caching token source.
func NewAppTokenSource(a AppAuth) (*AppTokenSource, error) {
	key, err := parseRSAPrivateKey(a.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	if a.HTTPClient == nil {
		a.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if a.now == nil {
		a.now = time.Now
	}
	if a.BaseURL == "" {
		a.BaseURL = "https://api.github.com"
	}
	return &AppTokenSource{auth: a, key: key}, nil
}

// Token returns a cached installation token, refreshing it ~1min before expiry.
func (s *AppTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && s.auth.now().Before(s.expires.Add(-time.Minute)) {
		return s.token, nil
	}
	jwt, err := s.appJWT()
	if err != nil {
		return "", err
	}
	tok, exp, err := s.mintInstallationToken(ctx, jwt)
	if err != nil {
		return "", err
	}
	s.token, s.expires = tok, exp
	return tok, nil
}

func (s *AppTokenSource) appJWT() (string, error) {
	now := s.auth.now()
	header := base64url(`{"alg":"RS256","typ":"JWT"}`)
	claims := fmt.Sprintf(`{"iat":%d,"exp":%d,"iss":"%d"}`,
		now.Add(-30*time.Second).Unix(), now.Add(9*time.Minute).Unix(), s.auth.AppID)
	signingInput := header + "." + base64url(claims)
	h := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (s *AppTokenSource) mintInstallationToken(ctx context.Context, jwt string) (string, time.Time, error) {
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", s.auth.BaseURL, s.auth.InstallationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := s.auth.HTTPClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("github: installation token mint failed: status %d", resp.StatusCode)
	}
	var out struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", time.Time{}, err
	}
	return out.Token, out.ExpiresAt, nil
}

func base64url(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemStr)))
	if block == nil {
		return nil, fmt.Errorf("github: invalid PEM private key")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("github: unsupported private key format: %w", err)
	}
	rsaKey, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("github: private key is not RSA")
	}
	return rsaKey, nil
}
```

- [ ] **Step 4: Run to confirm pass**

Run: `go test ./internal/forgeclient/github/ -run 'TokenSource' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/forgeclient/github/token.go internal/forgeclient/github/token_test.go
git commit -m "feat(deploystatus): PAT + GitHub App installation token sources (stdlib RS256)"
```

### Task 2.4: GitHub client implementing the port (TDD with httptest)

**Files:**
- Create: `internal/forgeclient/github/client.go`
- Test: `internal/forgeclient/github/client_test.go`

> Use the existing HTTP/retry conventions of `internal/alertmanagerclient/client.go` (read it first; reuse the same backoff helper if one is exported). Retry on network errors / 429 / 5xx with exponential backoff; do **not** retry 4xx. The bearer comes from the injected `TokenSource` (Task 2.3) — PAT or App.

- [ ] **Step 1: Write the failing test for retry + compare parsing** (httptest server returns 429 then 200)

```go
package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCompare_RetriesOn429ThenParses(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ahead_by":2,"commits":[
			{"sha":"a1","commit":{"message":"feat: x","author":{"date":"2026-06-01T00:00:00Z","name":"me"}},"parents":[{}],"html_url":"u1"},
			{"sha":"b2","commit":{"message":"merge","author":{"date":"2026-06-02T00:00:00Z","name":"me"}},"parents":[{},{}],"html_url":"u2"}
		]}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, TokenSource: PATTokenSource("t"), MaxRetries: 3, BaseBackoff: time.Millisecond})
	res, err := c.Compare(context.Background(), "o", "r", "base", "head")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.AheadBy != 2 || len(res.Commits) != 2 {
		t.Fatalf("got aheadBy=%d commits=%d", res.AheadBy, len(res.Commits))
	}
	if !res.Commits[1].IsMerge {
		t.Errorf("commit b2 should be flagged as merge (2 parents)")
	}
	if calls != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", calls)
	}
}

func TestCompare_DoesNotRetry4xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL, TokenSource: PATTokenSource("t"), MaxRetries: 3, BaseBackoff: time.Millisecond})
	if _, err := c.Compare(context.Background(), "o", "r", "base", "head"); err == nil {
		t.Fatal("expected error on 404")
	}
	if calls != 1 {
		t.Errorf("expected no retry on 404, got %d calls", calls)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/forgeclient/github/ -v`
Expected: FAIL (undefined `New`/`Config`).

- [ ] **Step 3: Implement the client** (BaseURL defaults to `https://api.github.com`; GHES uses `https://<host>/api/v3`)

```go
// Package github implements the forge.Client port against the GitHub REST API.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golgoth31/sreportal/internal/domain/forge"
)

// Config configures the GitHub forge client.
type Config struct {
	BaseURL     string        // e.g. https://api.github.com or https://ghe.example.com/api/v3
	TokenSource TokenSource   // PAT or GitHub App installation token source (Task 2.3)
	MaxRetries  int           // default 3
	BaseBackoff time.Duration // default 500ms
	HTTPClient  *http.Client  // default: &http.Client{Timeout: 15s}
}

// Client implements forge.Client.
type Client struct {
	cfg Config
	hc  *http.Client
}

var _ forge.Client = (*Client)(nil)

// New builds a Client with sane defaults.
func New(cfg Config) *Client {
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.BaseBackoff == 0 {
		cfg.BaseBackoff = 500 * time.Millisecond
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{cfg: cfg, hc: hc}
}

type ghCommit struct {
	Sha    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string    `json:"name"`
			Date time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Parents []struct{} `json:"parents"`
	HTMLURL string     `json:"html_url"`
}

type ghCompare struct {
	AheadBy int        `json:"ahead_by"`
	Commits []ghCommit `json:"commits"`
}

// Compare implements forge.Client.
func (c *Client) Compare(ctx context.Context, owner, repo, base, head string) (forge.CompareResult, error) {
	path := fmt.Sprintf("/repos/%s/%s/compare/%s...%s", owner, repo, base, head)
	var out ghCompare
	if err := c.getJSON(ctx, path, &out); err != nil {
		return forge.CompareResult{}, err
	}
	res := forge.CompareResult{AheadBy: out.AheadBy, Commits: make([]forge.Commit, 0, len(out.Commits))}
	for _, gc := range out.Commits {
		res.Commits = append(res.Commits, forge.Commit{
			Sha:     gc.Sha,
			Message: gc.Commit.Message,
			Author:  gc.Commit.Author.Name,
			Date:    gc.Commit.Author.Date,
			URL:     gc.HTMLURL,
			IsMerge: len(gc.Parents) > 1,
		})
	}
	return res, nil
}

// DefaultBranch implements forge.Client.
func (c *Client) DefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	var out struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := c.getJSON(ctx, fmt.Sprintf("/repos/%s/%s", owner, repo), &out); err != nil {
		return "", err
	}
	return out.DefaultBranch, nil
}

// LatestWorkflowRun implements forge.Client. Best-effort: ("", nil) when not resolvable.
func (c *Client) LatestWorkflowRun(ctx context.Context, owner, repo, workflowFile, branch string) (string, error) {
	if workflowFile == "" {
		return "", nil
	}
	var out struct {
		WorkflowRuns []struct {
			HTMLURL string `json:"html_url"`
		} `json:"workflow_runs"`
	}
	path := fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/runs?branch=%s&per_page=1", owner, repo, workflowFile, branch)
	if err := c.getJSON(ctx, path, &out); err != nil {
		return "", nil // best-effort
	}
	if len(out.WorkflowRuns) == 0 {
		return "", nil
	}
	return out.WorkflowRuns[0].HTMLURL, nil
}

func (c *Client) getJSON(ctx context.Context, path string, v any) error {
	base := c.cfg.BaseURL
	if base == "" {
		base = "https://api.github.com"
	}
	var lastErr error
	backoff := c.cfg.BaseBackoff
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if err != nil {
			return err
		}
		if c.cfg.TokenSource != nil {
			tok, terr := c.cfg.TokenSource.Token(ctx)
			if terr != nil {
				return terr // auth failure (e.g. App token mint) — surfaced as entry error upstream
			}
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = err // network error: retry
		} else {
			func() {
				defer resp.Body.Close()
				switch {
				case resp.StatusCode >= 200 && resp.StatusCode < 300:
					lastErr = json.NewDecoder(resp.Body).Decode(v)
				case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
					lastErr = fmt.Errorf("forge: retryable status %d", resp.StatusCode)
				default:
					lastErr = &nonRetryableError{status: resp.StatusCode}
				}
			}()
			if lastErr == nil {
				return nil
			}
			var nr *nonRetryableError
			if errors.As(lastErr, &nr) {
				return lastErr // 4xx: do not retry
			}
		}
		if attempt < c.cfg.MaxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}
	}
	return lastErr
}

type nonRetryableError struct{ status int }

func (e *nonRetryableError) Error() string { return fmt.Sprintf("forge: non-retryable status %d", e.status) }
```

- [ ] **Step 4: Run to confirm pass**

Run: `go test ./internal/forgeclient/github/ -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add internal/forgeclient/github/
git commit -m "feat(deploystatus): GitHub forge client with retry/backoff"
```

---

## Phase 3 — Domain read model + readstore

### Task 3.1: Domain read model + reader/writer interfaces

**Files:**
- Create: `internal/domain/deploystatus/read_model.go`, `reader.go`, `writer.go`

> Read `internal/domain/image/{read_model,reader,writer}.go` first and mirror the exact interface shape (method names like `ReplaceForNamespace`, `RemoveForNamespace`, `List`, `Count`, `Subscribe`).

- [ ] **Step 1: Write the read model**

```go
// Package deploystatus holds the read model and store interfaces for the deploy-status feature.
package deploystatus

import "time"

// Entry is the read-model projection of one service's deploy status.
type Entry struct {
	Key            string
	Workload       WorkloadRef
	Image          string
	SourceRepo     string
	DeployedRef    string
	DefaultBranch  string
	AheadBy        int
	PendingCommits []Commit
	PendingTrunc   bool
	DeployedAt     time.Time
	DeployRunURL   string
	State          string // ok | behind | unresolved | error
	Error          string
	LastCheckedAt  time.Time
}

type WorkloadRef struct{ Kind, Namespace, Name, Container string }

type Commit struct {
	Sha     string
	Message string
	Author  string
	Date    time.Time
	URL     string
}
```

- [ ] **Step 2: Write reader/writer interfaces** (match `internal/domain/image` signatures; adapt names below to whatever that package actually uses)

```go
// reader.go
package deploystatus

// Reader is the read side consumed by gRPC/MCP.
type Reader interface {
	// List returns all entries for a portal (deduplicated across contributing namespaces).
	List(portalRef string) []Entry
	Count(portalRef string) int
	Subscribe() (<-chan struct{}, func())
}
```

```go
// writer.go
package deploystatus

// Writer is the write side the controller pushes projections into.
type Writer interface {
	// ReplaceForNamespace replaces all entries contributed by (portalRef, namespace).
	ReplaceForNamespace(portalRef, namespace string, entries []Entry)
	// RemoveForNamespace drops a (portalRef, namespace) contribution (CR deletion).
	RemoveForNamespace(portalRef, namespace string)
}
```

- [ ] **Step 3: Build**

Run: `go build ./internal/domain/deploystatus/...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/domain/deploystatus/
git commit -m "feat(deploystatus): domain read model + store interfaces"
```

### Task 3.2: Readstore implementation (TDD)

**Files:**
- Create: `internal/readstore/deploystatus/store.go`
- Test: `internal/readstore/deploystatus/store_test.go`

> Mirror `internal/readstore/image/store.go` (likely built on the generic `readstore.Store[T]`). Reuse the generic store if that's what image does.

- [ ] **Step 1: Write the failing test**

```go
package deploystatus

import (
	"testing"

	dom "github.com/golgoth31/sreportal/internal/domain/deploystatus"
)

func TestStore_ReplaceListRemove_DedupesAcrossNamespaces(t *testing.T) {
	s := NewStore()
	s.ReplaceForNamespace("main", "ns1", []dom.Entry{{Key: "a", State: "behind"}})
	s.ReplaceForNamespace("main", "ns2", []dom.Entry{{Key: "b", State: "ok"}})
	if got := s.Count("main"); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	s.RemoveForNamespace("main", "ns1")
	if got := s.Count("main"); got != 1 {
		t.Fatalf("after remove count = %d, want 1", got)
	}
	if s.List("main")[0].Key != "b" {
		t.Errorf("unexpected remaining entry")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/readstore/deploystatus/ -v`
Expected: FAIL.

- [ ] **Step 3: Implement the store** (copy `internal/readstore/image/store.go` structure; if it wraps `readstore.Store[T]`, do the same — keying contributions by `portalRef|namespace`)

```go
// Package deploystatus implements the in-memory read store for deploy status entries.
package deploystatus

import (
	"sync"

	dom "github.com/golgoth31/sreportal/internal/domain/deploystatus"
)

// Store is an in-memory, concurrency-safe deploy-status read store.
type Store struct {
	mu        sync.RWMutex
	byScope   map[string][]dom.Entry // key: portalRef|namespace
	subs      map[chan struct{}]struct{}
}

var (
	_ dom.Reader = (*Store)(nil)
	_ dom.Writer = (*Store)(nil)
)

func NewStore() *Store {
	return &Store{byScope: map[string][]dom.Entry{}, subs: map[chan struct{}]struct{}{}}
}

func scopeKey(portalRef, ns string) string { return portalRef + "|" + ns }

func (s *Store) ReplaceForNamespace(portalRef, namespace string, entries []dom.Entry) {
	s.mu.Lock()
	s.byScope[scopeKey(portalRef, namespace)] = entries
	s.mu.Unlock()
	s.broadcast()
}

func (s *Store) RemoveForNamespace(portalRef, namespace string) {
	s.mu.Lock()
	delete(s.byScope, scopeKey(portalRef, namespace))
	s.mu.Unlock()
	s.broadcast()
}

func (s *Store) List(portalRef string) []dom.Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []dom.Entry
	prefix := portalRef + "|"
	for k, entries := range s.byScope {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			out = append(out, entries...)
		}
	}
	return out
}

func (s *Store) Count(portalRef string) int { return len(s.List(portalRef)) }

func (s *Store) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.subs, ch)
		close(ch)
		s.mu.Unlock()
	}
}

func (s *Store) broadcast() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
```

- [ ] **Step 4: Run to confirm pass**

Run: `go test ./internal/readstore/deploystatus/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/readstore/deploystatus/
git commit -m "feat(deploystatus): in-memory deploy-status read store"
```

---

## Phase 4 — Controller chain handlers

The chain (per spec §2): `select_due` → `resolve_oci_source` → `forge_compare` → `resolve_deploy_run` → `update_readstore` → `update_status`. `ChainData` carries the in-flight entries.

### Task 4.1: ChainData + lag-computation pure logic (TDD)

**Files:**
- Create: `internal/controller/deploystatus/chain/handlers.go` (ChainData), `internal/controller/deploystatus/chain/compute.go`
- Test: `internal/controller/deploystatus/chain/compute_test.go`

- [ ] **Step 1: Define ChainData**

```go
// Package chain holds the deploy-status reconciliation handlers.
package chain

import (
	"github.com/golgoth31/sreportal/internal/domain/forge"
)

// ChainData is shared state across the deploy-status handler chain.
type ChainData struct {
	// Due are the entries selected for a forge check this cycle, keyed by entry Key.
	Due []WorkItem
	// Computed holds the resulting projection entries to write to the readstore + CR.
	Computed []ComputedEntry
}

// WorkItem is one service to evaluate.
type WorkItem struct {
	Key       string
	Image     string
	Workload  forge.RepoRef // host/owner/repo once resolved (zero until resolve_oci_source)
	// raw fields needed downstream:
	WorkloadKind, WorkloadNamespace, WorkloadName, WorkloadContainer string
	SourceURL  string // OCI source label
	DeployedRef string // OCI revision label or semver tag fallback
}

// ComputedEntry is the per-service result.
type ComputedEntry struct {
	Key            string
	Image          string
	SourceRepo     string
	DeployedRef    string
	DefaultBranch  string
	AheadBy        int
	PendingCommits []forge.Commit
	PendingTrunc   bool
	DeployRunURL   string
	State          string
	Error          string
}
```

- [ ] **Step 2: Write the failing test for `ComputeLag`** (filters merges, caps at 50, maps state)

```go
package chain

import (
	"testing"

	"github.com/golgoth31/sreportal/internal/domain/forge"
)

func TestComputeLag_FiltersMergesAndCaps(t *testing.T) {
	commits := make([]forge.Commit, 0, 60)
	commits = append(commits, forge.Commit{Sha: "m", IsMerge: true})
	for i := 0; i < 55; i++ {
		commits = append(commits, forge.Commit{Sha: "c"})
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
		if c.Sha == "m" {
			t.Error("merge commit should be filtered out")
		}
	}
}

func TestStateFor(t *testing.T) {
	if StateFor(0) != "ok" {
		t.Error("aheadBy 0 -> ok")
	}
	if StateFor(3) != "behind" {
		t.Error("aheadBy 3 -> behind")
	}
}
```

- [ ] **Step 3: Run to confirm failure**

Run: `go test ./internal/controller/deploystatus/chain/ -run 'TestComputeLag|TestStateFor' -v`
Expected: FAIL.

- [ ] **Step 4: Implement**

```go
package chain

import "github.com/golgoth31/sreportal/internal/domain/forge"

const pendingCap = 50

// ComputeLag filters merge commits and caps the list at 50, returning the
// capped list and whether truncation occurred.
func ComputeLag(cr forge.CompareResult) (pending []forge.Commit, truncated bool) {
	for _, c := range cr.Commits {
		if c.IsMerge {
			continue
		}
		pending = append(pending, c)
	}
	if len(pending) > pendingCap {
		return pending[:pendingCap], true
	}
	return pending, false
}

// StateFor maps an aheadBy count to a state for a successfully-compared entry.
func StateFor(aheadBy int) string {
	if aheadBy == 0 {
		return "ok"
	}
	return "behind"
}
```

- [ ] **Step 5: Run to confirm pass**

Run: `go test ./internal/controller/deploystatus/chain/ -run 'TestComputeLag|TestStateFor' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/controller/deploystatus/chain/handlers.go internal/controller/deploystatus/chain/compute.go internal/controller/deploystatus/chain/compute_test.go
git commit -m "feat(deploystatus): lag computation (merge filter, 50 cap, state)"
```

### Task 4.2: `resolve_oci_source` handler (forge endpoint matching, TDD)

**Files:**
- Create: `internal/controller/deploystatus/chain/resolve_oci_source.go`
- Test: `internal/controller/deploystatus/chain/resolve_oci_source_test.go`

This handler: for each `WorkItem`, parse `SourceURL` → `RepoRef`, match the host against the configured `forges[]`; unmatched host → mark the computed entry `unresolved` (and drop it from further forge work).

- [ ] **Step 1: Write the failing test** (table: matched host keeps the item; unmatched host yields an `unresolved` ComputedEntry and no Due item)

```go
package chain

import (
	"context"
	"testing"

	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/reconciler"
	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

func TestResolveOCISource_UnmatchedHostMarksUnresolved(t *testing.T) {
	h := NewResolveOCISourceHandler([]config.ForgeConfig{{Host: "github.com", TokenEnv: "X"}})
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: &sreportalv1alpha1.DeployStatus{},
		Data: ChainData{Due: []WorkItem{
			{Key: "a", SourceURL: "https://gitlab.com/o/r"},        // unmatched
			{Key: "b", SourceURL: "https://github.com/o/r"},        // matched
			{Key: "c", SourceURL: ""},                              // no label -> unresolved
		}},
	}
	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatal(err)
	}
	// a and c become unresolved ComputedEntry; b remains Due.
	gotUnresolved := 0
	for _, e := range rc.Data.Computed {
		if e.State == "unresolved" {
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
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/controller/deploystatus/chain/ -run TestResolveOCISource -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
package chain

import (
	"context"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ResolveOCISourceHandler parses each WorkItem's OCI source URL and matches it
// against configured forges. Unmatched / unparseable items become "unresolved"
// computed entries and are removed from the Due set.
type ResolveOCISourceHandler struct {
	forgesByHost map[string]config.ForgeConfig
}

func NewResolveOCISourceHandler(forges []config.ForgeConfig) *ResolveOCISourceHandler {
	m := make(map[string]config.ForgeConfig, len(forges))
	for _, f := range forges {
		m[f.Host] = f
	}
	return &ResolveOCISourceHandler{forgesByHost: m}
}

func (h *ResolveOCISourceHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]) error {
	kept := rc.Data.Due[:0]
	for _, wi := range rc.Data.Due {
		if wi.SourceURL == "" {
			rc.Data.Computed = append(rc.Data.Computed, unresolved(wi, "no OCI source label"))
			continue
		}
		ref, err := forge.ParseSourceURL(wi.SourceURL)
		if err != nil {
			rc.Data.Computed = append(rc.Data.Computed, unresolved(wi, "unparseable source url"))
			continue
		}
		if _, ok := h.forgesByHost[ref.Host]; !ok {
			rc.Data.Computed = append(rc.Data.Computed, unresolved(wi, "no forge configured for host "+ref.Host))
			continue
		}
		wi.Workload = ref
		kept = append(kept, wi)
	}
	rc.Data.Due = kept
	return nil
}

func unresolved(wi WorkItem, msg string) ComputedEntry {
	return ComputedEntry{
		Key: wi.Key, Image: wi.Image, DeployedRef: wi.DeployedRef,
		State: "unresolved", Error: msg,
	}
}
```

- [ ] **Step 4: Run to confirm pass**

Run: `go test ./internal/controller/deploystatus/chain/ -run TestResolveOCISource -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/deploystatus/chain/resolve_oci_source*.go
git commit -m "feat(deploystatus): resolve OCI source + match forge host"
```

### Task 4.3: `forge_compare` + `resolve_deploy_run` handlers (TDD with a fake forge.Client)

**Files:**
- Create: `internal/controller/deploystatus/chain/forge_compare.go`, `resolve_deploy_run.go`
- Test: `internal/controller/deploystatus/chain/forge_compare_test.go`

The compare handler needs a `forge.Client` per host. Inject a `func(host string) forge.Client` resolver so tests pass a fake.

- [ ] **Step 1: Write the failing test with a fake client**

```go
package chain

import (
	"context"
	"testing"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

type fakeForge struct {
	branch string
	cmp    forge.CompareResult
	err    error
}

func (f *fakeForge) DefaultBranch(context.Context, string, string) (string, error) { return f.branch, f.err }
func (f *fakeForge) Compare(context.Context, string, string, string, string) (forge.CompareResult, error) {
	return f.cmp, f.err
}
func (f *fakeForge) LatestWorkflowRun(context.Context, string, string, string, string) (string, error) {
	return "", nil
}

func TestForgeCompare_ProducesBehindEntry(t *testing.T) {
	fc := &fakeForge{branch: "main", cmp: forge.CompareResult{AheadBy: 2, Commits: []forge.Commit{{Sha: "c1"}, {Sha: "c2"}}}}
	h := NewForgeCompareHandler(func(string) forge.Client { return fc })
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: &sreportalv1alpha1.DeployStatus{},
		Data: ChainData{Due: []WorkItem{
			{Key: "b", DeployedRef: "v1", Workload: forge.RepoRef{Host: "github.com", Owner: "o", Repo: "r"}},
		}},
	}
	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatal(err)
	}
	if len(rc.Data.Computed) != 1 {
		t.Fatalf("computed = %d, want 1", len(rc.Data.Computed))
	}
	e := rc.Data.Computed[0]
	if e.State != "behind" || e.AheadBy != 2 || e.DefaultBranch != "main" {
		t.Fatalf("unexpected entry %+v", e)
	}
}

func TestForgeCompare_ErrorMarksErrorState(t *testing.T) {
	fc := &fakeForge{err: context.DeadlineExceeded}
	h := NewForgeCompareHandler(func(string) forge.Client { return fc })
	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]{
		Resource: &sreportalv1alpha1.DeployStatus{},
		Data: ChainData{Due: []WorkItem{{Key: "b", Workload: forge.RepoRef{Host: "github.com", Owner: "o", Repo: "r"}}}},
	}
	if err := h.Handle(context.Background(), rc); err != nil {
		t.Fatalf("handler must not fail the chain on per-entry error: %v", err)
	}
	if rc.Data.Computed[0].State != "error" {
		t.Errorf("state = %q, want error", rc.Data.Computed[0].State)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/controller/deploystatus/chain/ -run TestForgeCompare -v`
Expected: FAIL.

- [ ] **Step 3: Implement the compare handler** (per-entry error isolation — never fails the chain)

```go
package chain

import (
	"context"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/log"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ForgeCompareHandler computes deployment lag for each resolved Due item.
type ForgeCompareHandler struct {
	clientFor func(host string) forge.Client
}

func NewForgeCompareHandler(clientFor func(host string) forge.Client) *ForgeCompareHandler {
	return &ForgeCompareHandler{clientFor: clientFor}
}

func (h *ForgeCompareHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]) error {
	logger := log.FromContext(ctx).WithName("forge-compare")
	for _, wi := range rc.Data.Due {
		cl := h.clientFor(wi.Workload.Host)
		entry := ComputedEntry{Key: wi.Key, Image: wi.Image, DeployedRef: wi.DeployedRef, SourceRepo: wi.SourceURL}

		branch, err := cl.DefaultBranch(ctx, wi.Workload.Owner, wi.Workload.Repo)
		if err != nil {
			logger.V(1).Info("default branch lookup failed", "key", wi.Key, "err", err)
			entry.State, entry.Error = "error", err.Error()
			rc.Data.Computed = append(rc.Data.Computed, entry)
			continue
		}
		entry.DefaultBranch = branch

		cr, err := cl.Compare(ctx, wi.Workload.Owner, wi.Workload.Repo, wi.DeployedRef, branch)
		if err != nil {
			entry.State, entry.Error = "error", err.Error()
			rc.Data.Computed = append(rc.Data.Computed, entry)
			continue
		}
		entry.AheadBy = cr.AheadBy
		entry.PendingCommits, entry.PendingTrunc = ComputeLag(cr)
		entry.State = StateFor(cr.AheadBy)
		rc.Data.Computed = append(rc.Data.Computed, entry)
	}
	return nil
}
```

- [ ] **Step 4: Implement the deploy-run handler** (best-effort; only enriches `behind`/`ok` entries that resolved)

```go
// resolve_deploy_run.go
package chain

import (
	"context"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// ResolveDeployRunHandler enriches computed entries with a best-effort deploy
// workflow run URL. Failures are swallowed (the link is optional).
type ResolveDeployRunHandler struct {
	clientFor func(host string) forge.Client
	workflowByHost map[string]string // host -> deployWorkflow file
	repoByKey      map[string]forge.RepoRef
}

func NewResolveDeployRunHandler(clientFor func(host string) forge.Client, forges []config.ForgeConfig) *ResolveDeployRunHandler {
	wf := make(map[string]string, len(forges))
	for _, f := range forges {
		wf[f.Host] = f.DeployWorkflow
	}
	return &ResolveDeployRunHandler{clientFor: clientFor, workflowByHost: wf}
}

func (h *ResolveDeployRunHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]) error {
	// Build key->repo from Due (only resolved items have a repo).
	repo := make(map[string]forge.RepoRef, len(rc.Data.Due))
	for _, wi := range rc.Data.Due {
		repo[wi.Key] = wi.Workload
	}
	for i := range rc.Data.Computed {
		e := &rc.Data.Computed[i]
		ref, ok := repo[e.Key]
		if !ok || e.State == "error" || e.State == "unresolved" {
			continue
		}
		wf := h.workflowByHost[ref.Host]
		url, _ := h.clientFor(ref.Host).LatestWorkflowRun(ctx, ref.Owner, ref.Repo, wf, e.DefaultBranch)
		e.DeployRunURL = url
	}
	return nil
}
```

- [ ] **Step 5: Run all chain tests**

Run: `go test ./internal/controller/deploystatus/chain/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/controller/deploystatus/chain/forge_compare*.go internal/controller/deploystatus/chain/resolve_deploy_run.go
git commit -m "feat(deploystatus): forge compare + best-effort deploy-run handlers"
```

### Task 4.4: `select_due` + `update_readstore` + `update_status` handlers

**Files:**
- Create: `internal/controller/deploystatus/chain/select_due.go`, `update_readstore.go`, `update_status.go`
- Test: `internal/controller/deploystatus/chain/select_due_test.go`, `update_readstore_test.go`

`select_due`: reads the ImageRegistry-derived workloads for this `(portalRef, namespace)` and builds `WorkItem`s for entries whose `LastCheckedAt` is older than `RefreshInterval` (mirror `select_due_images.go` pacing). For v1, the WorkItems come from the `DeployStatus` CR's own `Spec.Services` (populated by the image source — see Task 5.2 wiring) plus their `LastCheckedAt`.

- [ ] **Step 1: Write the failing test for due selection** (entry checked < refreshInterval ago is skipped; never-checked is due)

```go
package chain

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

func TestSelectDue_SkipsRecentlyChecked(t *testing.T) {
	now := time.Now()
	svcs := []sreportalv1alpha1.DeployStatusEntry{
		{Key: "fresh", LastCheckedAt: metav1.NewTime(now.Add(-time.Minute))},
		{Key: "stale", LastCheckedAt: metav1.NewTime(now.Add(-time.Hour))},
		{Key: "never"},
	}
	due := selectDue(svcs, 5*time.Minute, now)
	got := map[string]bool{}
	for _, d := range due {
		got[d.Key] = true
	}
	if got["fresh"] {
		t.Error("fresh entry should not be due")
	}
	if !got["stale"] || !got["never"] {
		t.Error("stale and never-checked entries should be due")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/controller/deploystatus/chain/ -run TestSelectDue -v`
Expected: FAIL.

- [ ] **Step 3: Implement `select_due.go`** (pure `selectDue` helper + a `SelectDueHandler` that fills `rc.Data.Due` from `rc.Resource.Spec.Services`)

```go
package chain

import (
	"context"
	"time"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// SelectDueHandler builds the Due work list from the CR's services whose
// LastCheckedAt is older than refreshInterval.
type SelectDueHandler struct {
	refreshInterval time.Duration
	now             func() time.Time
}

func NewSelectDueHandler(refreshInterval time.Duration) *SelectDueHandler {
	if refreshInterval == 0 {
		refreshInterval = 5 * time.Minute
	}
	return &SelectDueHandler{refreshInterval: refreshInterval, now: time.Now}
}

func (h *SelectDueHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]) error {
	rc.Data.Due = selectDue(rc.Resource.Spec.Services, h.refreshInterval, h.now())
	return nil
}

func selectDue(svcs []sreportalv1alpha1.DeployStatusEntry, refresh time.Duration, now time.Time) []WorkItem {
	var out []WorkItem
	for _, s := range svcs {
		if !s.LastCheckedAt.IsZero() && now.Sub(s.LastCheckedAt.Time) < refresh {
			continue
		}
		out = append(out, WorkItem{
			Key: s.Key, Image: s.Image,
			WorkloadKind: s.Workload.Kind, WorkloadNamespace: s.Workload.Namespace,
			WorkloadName: s.Workload.Name, WorkloadContainer: s.Workload.Container,
			SourceURL: s.SourceRepo, DeployedRef: s.DeployedRef,
		})
	}
	return out
}
```

> Note: the OCI label read (source/revision) happens in `resolve_oci_source` for items lacking `SourceRepo`/`DeployedRef`; in v1 the image-source population (Task 5.2) may pre-fill `SourceRepo`. Keep `resolve_oci_source` as the authority that fetches labels when `SourceURL == ""` but the image is known — see Task 5.2 note on whether label-fetch lives in the image-source step or the handler. Either way the handler signatures here are unchanged.

- [ ] **Step 4: Implement `update_readstore.go`** (projects `Computed` into the readstore for this `(portalRef, namespace)`)

```go
package chain

import (
	"context"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	dom "github.com/golgoth31/sreportal/internal/domain/deploystatus"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// UpdateReadStoreHandler projects computed entries into the deploy-status read store.
type UpdateReadStoreHandler struct {
	store dom.Writer
}

func NewUpdateReadStoreHandler(store dom.Writer) *UpdateReadStoreHandler {
	return &UpdateReadStoreHandler{store: store}
}

func (h *UpdateReadStoreHandler) Handle(_ context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]) error {
	entries := make([]dom.Entry, 0, len(rc.Data.Computed))
	for _, c := range rc.Data.Computed {
		entries = append(entries, dom.Entry{
			Key: c.Key, Image: c.Image, SourceRepo: c.SourceRepo, DeployedRef: c.DeployedRef,
			DefaultBranch: c.DefaultBranch, AheadBy: c.AheadBy,
			PendingCommits: toDomCommits(c.PendingCommits), PendingTrunc: c.PendingTrunc,
			DeployRunURL: c.DeployRunURL, State: c.State, Error: c.Error,
		})
	}
	h.store.ReplaceForNamespace(rc.Resource.Spec.PortalRef, rc.Resource.Spec.Namespace, entries)
	return nil
}
```

> Add a small `toDomCommits([]forge.Commit) []dom.Entry`-style mapper in this file mapping `forge.Commit` → `dom.Commit`.

- [ ] **Step 5: Implement `update_status.go`** (patch CR `Spec.Services[i].LastCheckedAt` for due items + `Status.ServiceCount`/`ObservedGeneration`; mirror `imageregistry` `update_status.go` patch style)

```go
package chain

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

// UpdateStatusHandler stamps LastCheckedAt on processed services and updates status counters.
type UpdateStatusHandler struct {
	client client.Client
	now    func() metav1.Time
}

func NewUpdateStatusHandler(c client.Client) *UpdateStatusHandler {
	return &UpdateStatusHandler{client: c, now: func() metav1.Time { return metav1.Now() }}
}

func (h *UpdateStatusHandler) Handle(ctx context.Context, rc *reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, ChainData]) error {
	cr := rc.Resource
	computedByKey := make(map[string]ComputedEntry, len(rc.Data.Computed))
	for _, c := range rc.Data.Computed {
		computedByKey[c.Key] = c
	}
	now := h.now()
	for i := range cr.Spec.Services {
		if c, ok := computedByKey[cr.Spec.Services[i].Key]; ok {
			s := &cr.Spec.Services[i]
			s.LastCheckedAt = now
			s.State = c.State
			s.AheadBy = c.AheadBy
			s.Error = c.Error
			s.DefaultBranch = c.DefaultBranch
			s.DeployRunURL = c.DeployRunURL
			// pendingCommits/pendingTruncated mapped likewise
		}
	}
	if err := h.client.Update(ctx, cr); err != nil {
		return err
	}
	cr.Status.ServiceCount = len(cr.Spec.Services)
	cr.Status.ObservedGeneration = cr.Generation
	return h.client.Status().Update(ctx, cr)
}
```

- [ ] **Step 6: Run all chain tests**

Run: `go test ./internal/controller/deploystatus/chain/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/controller/deploystatus/chain/select_due*.go internal/controller/deploystatus/chain/update_readstore.go internal/controller/deploystatus/chain/update_status.go
git commit -m "feat(deploystatus): select-due, readstore projection, status update handlers"
```

---

## Phase 5 — Controller wiring + main.go + RBAC

### Task 5.1: The reconciler + chain assembly + field index

**Files:**
- Create: `internal/controller/deploystatus/deploystatus_controller.go`
- Modify: `internal/controller/deploystatus/` (suite_test.go via envtest — see Task 5.3)

> Mirror `internal/controller/imageregistry/imageregistry_controller.go`: struct embeds `client.Client`, holds a `*reconciler.Chain[...]`, a `NewDeployStatusReconciler(...)` constructor that builds the handler slice, `Reconcile` that loads the CR, runs the chain, and `SetupWithManager` with a `spec.portalRef` field indexer + finalizer for readstore cleanup.

- [ ] **Step 1: Write the reconciler**

```go
// Package deploystatus contains the reconciler for the DeployStatus CRD.
package deploystatus

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	"github.com/golgoth31/sreportal/internal/config"
	dschain "github.com/golgoth31/sreportal/internal/controller/deploystatus/chain"
	domds "github.com/golgoth31/sreportal/internal/domain/deploystatus"
	"github.com/golgoth31/sreportal/internal/domain/forge"
	"github.com/golgoth31/sreportal/internal/reconciler"
)

const (
	finalizerName  = "deploystatus.sreportal.io/cleanup"
	portalRefField = "spec.portalRef"
	requeueEvery   = 5 * time.Minute
)

// DeployStatusReconciler reconciles a DeployStatus object.
type DeployStatusReconciler struct {
	client.Client
	chain *reconciler.Chain[*sreportalv1alpha1.DeployStatus, dschain.ChainData]
	store domds.Writer
}

// +kubebuilder:rbac:groups=sreportal.io,resources=deploystatuses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sreportal.io,resources=deploystatuses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sreportal.io,resources=deploystatuses/finalizers,verbs=update

// NewDeployStatusReconciler wires the handler chain.
func NewDeployStatusReconciler(
	c client.Client,
	store domds.Writer,
	clientFor func(host string) forge.Client,
	cfg *config.DeployStatusConfig,
) *DeployStatusReconciler {
	refresh := time.Duration(cfg.RefreshInterval)
	chain := reconciler.NewChain[*sreportalv1alpha1.DeployStatus, dschain.ChainData](
		"deploystatus",
		dschain.NewSelectDueHandler(refresh),
		dschain.NewResolveOCISourceHandler(cfg.Forges),
		dschain.NewForgeCompareHandler(clientFor),
		dschain.NewResolveDeployRunHandler(clientFor, cfg.Forges),
		dschain.NewUpdateReadStoreHandler(store),
		dschain.NewUpdateStatusHandler(c),
	)
	return &DeployStatusReconciler{Client: c, chain: chain, store: store}
}

func (r *DeployStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cr sreportalv1alpha1.DeployStatus
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Deletion: drop the readstore contribution.
	if !cr.DeletionTimestamp.IsZero() {
		r.store.RemoveForNamespace(cr.Spec.PortalRef, cr.Spec.Namespace)
		controllerutilRemoveFinalizer(&cr, finalizerName)
		return ctrl.Result{}, r.Update(ctx, &cr)
	}
	controllerutilAddFinalizer(&cr, finalizerName)

	rc := &reconciler.ReconcileContext[*sreportalv1alpha1.DeployStatus, dschain.ChainData]{Resource: &cr}
	if err := r.chain.Execute(ctx, rc); err != nil {
		return ctrl.Result{}, err
	}
	if rc.Result.RequeueAfter > 0 {
		return rc.Result, nil
	}
	return ctrl.Result{RequeueAfter: requeueEvery}, nil
}

func (r *DeployStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &sreportalv1alpha1.DeployStatus{}, portalRefField,
		func(o client.Object) []string {
			return []string{o.(*sreportalv1alpha1.DeployStatus).Spec.PortalRef}
		}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&sreportalv1alpha1.DeployStatus{}).
		Named("deploystatus").
		Complete(r)
}
```

> Replace `controllerutilAddFinalizer`/`controllerutilRemoveFinalizer` with the real `controllerutil.AddFinalizer`/`RemoveFinalizer` calls and `apierrors` usage exactly as `imageregistry_controller.go` does — copy that file's finalizer block verbatim and adapt the type.

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: success (adjust imports to match the real controllerutil pattern).

- [ ] **Step 3: Commit**

```bash
git add internal/controller/deploystatus/deploystatus_controller.go
git commit -m "feat(deploystatus): reconciler + chain assembly + portalRef index"
```

### Task 5.2: Image-source population — feed `DeployStatus` CRs from imageInventory

**Files:**
- Create: `internal/controller/deploystatus/chain/sync_from_images.go` OR extend the imageinventory controller to also project into a DeployStatus CR.

> Decision for the implementer: the cleanest seam mirrors how `ImageRegistry` is itself populated by the imageInventory controller (`sync_registry_crs.go`). Add a sibling handler in the imageinventory chain (or a small dedicated controller watching `ImageRegistry`) that creates/updates one `DeployStatus` CR per `(portalRef, namespace)` whose `Spec.Services` mirror the observed images (Key, Workload, Image), pre-filling `SourceRepo`/`DeployedRef` when the image's OCI labels are already known, else leaving them empty for `resolve_oci_source` to fetch. Read `internal/controller/imageinventory/chain/sync_registry_crs.go` and replicate its CreateOrUpdate pattern.

- [ ] **Step 1: Read the reference** `internal/controller/imageinventory/chain/sync_registry_crs.go` to copy the CreateOrUpdate-by-(portal,ns) pattern.

- [ ] **Step 2: Implement the projection** that upserts a `DeployStatus` CR named like `<portal>-<namespace>` (match the imageregistry naming in `internal/domain/imageregistry/naming.go`) with `Spec.Services` built from observed images.

- [ ] **Step 3: Write an envtest** that creates an ImageRegistry and asserts a DeployStatus CR appears (BDD/Ginkgo, mirror imageinventory suite).

- [ ] **Step 4: Run**

Run: `make test` (envtest)
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/deploystatus/ internal/controller/imageinventory/
git commit -m "feat(deploystatus): project DeployStatus CRs from observed images"
```

### Task 5.3: Wire into `cmd/main.go` (readstore, reconciler, forge clients, token env)

**Files:**
- Modify: `cmd/main.go`

> Read the existing wiring blocks (the CLAUDE.md lists them: readstores created in main.go, controllers receive writers, gRPC/MCP receive readers; SlackEmojiConfig token read via `os.Getenv`). Insert analogous blocks.

- [ ] **Step 1: Create the readstore + build the per-host forge client resolver from config + env tokens**

```go
// after other readstore creations:
deployStatusStore := readstoredeploystatus.NewStore()

// build forge clients keyed by host; each forge resolves its token source
// (PAT or GitHub App installation token) from named env vars.
var clientFor func(host string) forge.Client
if operatorConfig.DeployStatus != nil && operatorConfig.DeployStatus.Enabled {
	clients := map[string]forge.Client{}
	for _, f := range operatorConfig.DeployStatus.Forges {
		baseURL := githubBaseURLForHost(f.Host)
		var ts githubforge.TokenSource
		switch {
		case f.Auth.App != nil:
			pem := os.Getenv(f.Auth.App.PrivateKeyEnv)
			if pem == "" {
				setupLog.Info("deploystatus: App private key env empty; entries for this host will error",
					"host", f.Host, "privateKeyEnv", f.Auth.App.PrivateKeyEnv)
			}
			src, err := githubforge.NewAppTokenSource(githubforge.AppAuth{
				AppID: f.Auth.App.AppID, InstallationID: f.Auth.App.InstallationID,
				PrivateKeyPEM: pem, BaseURL: baseURL,
			})
			if err != nil {
				setupLog.Error(err, "deploystatus: invalid GitHub App config", "host", f.Host)
				os.Exit(1)
			}
			ts = src
		default: // PAT
			token := os.Getenv(f.Auth.TokenEnv)
			if token == "" {
				setupLog.Info("deploystatus: forge token env empty; entries for this host will error",
					"host", f.Host, "tokenEnv", f.Auth.TokenEnv)
			}
			ts = githubforge.PATTokenSource(token)
		}
		clients[f.Host] = githubforge.New(githubforge.Config{BaseURL: baseURL, TokenSource: ts})
	}
	clientFor = func(host string) forge.Client { return clients[host] }
}
```

> Add a tiny `githubBaseURLForHost(host)` helper in main.go: `github.com` → `https://api.github.com`, else `https://<host>/api/v3`.

- [ ] **Step 2: Register the reconciler** (guarded by `operatorConfig.DeployStatus.Enabled`)

```go
if operatorConfig.DeployStatus != nil && operatorConfig.DeployStatus.Enabled {
	if err := deploystatus.NewDeployStatusReconciler(mgr.GetClient(), deployStatusStore, clientFor, operatorConfig.DeployStatus).
		SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DeployStatus")
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add cmd/main.go
git commit -m "feat(deploystatus): wire reconciler, readstore and forge clients in main"
```

### Task 5.4: Regenerate RBAC + manifests

- [ ] **Step 1: Regenerate**

Run: `make manifests generate`
Expected: `config/rbac/role.yaml` gains the deploystatus verbs; CRD up to date. (Generated — don't hand-edit.)

- [ ] **Step 2: Commit**

```bash
git add config/
git commit -m "chore(deploystatus): regenerate RBAC and CRD manifests"
```

---

## Phase 6 — gRPC / Connect service + feature gate

### Task 6.1: Implement `DeployStatusService`

**Files:**
- Create: `internal/grpc/deploystatus_service.go`
- Test: `internal/grpc/deploystatus_service_test.go`

> Read `internal/grpc/image_service.go` and `internal/grpc/feature_gate.go` first; copy the struct/constructor/marshal pattern and the feature-gate check (it gates per-portal using `IsFeatureEnabled` / a `Check*` helper).

- [ ] **Step 1: Write the failing test** (reader returns 2 entries → response has 2; feature disabled → permission/unavailable error per the existing gate convention)

```go
package grpc

// TestDeployStatusService_List mirrors image_service_test.go: seed a fake reader,
// call ListDeployStatus, assert the proto response maps entries 1:1 and that the
// feature gate blocks when the portal disables deployStatus.
```

> Fill in concretely by copying `image_service_test.go` and swapping the types — the test infra (fake reader, feature-gate stub) already exists there.

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/grpc/ -run DeployStatus -v`
Expected: FAIL.

- [ ] **Step 3: Implement the service** mapping `domds.Entry` → `genv1.DeployStatusEntry`, gated by `IsDeployStatusEnabled` via the same gate used by image service. Register it in the gRPC server setup file alongside `image_service` (grep where `NewImageService`/handler registration happens — same file).

- [ ] **Step 4: Run to confirm pass**

Run: `go test ./internal/grpc/ -run DeployStatus -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/grpc/deploystatus_service.go internal/grpc/deploystatus_service_test.go <server-registration-file>
git commit -m "feat(deploystatus): Connect DeployStatusService with feature gate"
```

---

## Phase 7 — MCP server

### Task 7.1: `/mcp/deploystatus` server

**Files:**
- Create: `internal/mcp/deploystatus_server.go`

> Copy `internal/mcp/image_server.go` (struct, constructor with hooks, tool registration, `Handler()`), swap reader + result formatting. Mount it in `cmd/main.go` web-server setup next to `/mcp/image` (the CLAUDE.md lists the existing mount points).

- [ ] **Step 1: Implement the server** exposing a `list_deploy_status` tool (params: `portal`, optional `state`).
- [ ] **Step 2: Mount in main.go** at `/mcp/deploystatus`.
- [ ] **Step 3: Build + smoke test**

Run: `go build ./... && go test ./internal/mcp/ -run DeployStatus -v`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/deploystatus_server.go cmd/main.go
git commit -m "feat(deploystatus): MCP server at /mcp/deploystatus"
```

---

## Phase 8 — Validating webhook

### Task 8.1: Scaffold + implement the webhook

**Files:**
- Create (by CLI): `internal/webhook/v1alpha1/deploystatus_webhook.go`

- [ ] **Step 1: Scaffold with kubebuilder**

```bash
kubebuilder create webhook --group sreportal --version v1alpha1 --kind DeployStatus --programmatic-validation
```

Expected: webhook file + `config/webhook` manifests regenerated. (Don't hand-edit generated manifests.)

- [ ] **Step 2: Implement validation** mirroring `internal/webhook/v1alpha1/imageregistry_webhook.go` — the CR is controller-managed, so validate invariants (portalRef + namespace non-empty; entries have keys/state in the enum). Copy that file's `ValidateCreate/Update/Delete` shape.

- [ ] **Step 3: Write the webhook test** (reject empty portalRef; accept a well-formed CR) mirroring `imageregistry_webhook_test.go`.

- [ ] **Step 4: Run + regenerate**

Run: `make manifests generate && go test ./internal/webhook/... -run DeployStatus -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/webhook/ config/webhook/ config/crd/ PROJECT
git commit -m "feat(deploystatus): validating webhook"
```

---

## Phase 9 — Remote federation

### Task 9.1: `sync_remote_deploy_status` portal handler

**Files:**
- Create: `internal/controller/portal/chain/sync_remote_deploy_status.go`
- Test: `internal/controller/portal/chain/sync_remote_deploy_status_test.go`

> Copy `sync_remote_image_inventory.go` verbatim and adapt the type to `DeployStatus`: for each remote portal, CreateOrUpdate a shadow CR `remote-<portal>` with `Spec.IsRemote=true`. No-op for local portals or when `IsDeployStatusEnabled()` is false.

- [ ] **Step 1: Write the handler** (copy + adapt the reference; same finalizer/owner-ref/CreateOrUpdate logic).
- [ ] **Step 2: Write the test** (mirror `sync_remote_image_inventory_test.go`): remote portal with feature on → shadow CR created; feature off → none.
- [ ] **Step 3: Register the handler** in the portal controller chain (same place `SyncRemoteImageInventoryHandler` is added).
- [ ] **Step 4: Run**

Run: `go test ./internal/controller/portal/... -run DeployStatus -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/portal/chain/sync_remote_deploy_status*.go
git commit -m "feat(deploystatus): federate via shadow remote-<portal> CR"
```

### Task 9.2: `IsRemote` branch in the deploy-status controller

**Files:**
- Modify: `internal/controller/deploystatus/deploystatus_controller.go`
- Modify: add `internal/controller/deploystatus/chain/fetch_remote.go`

> When `cr.Spec.IsRemote`, skip the local compute chain and instead use the existing remote-client plumbing (`internal/controller/portal/chain/build_remote_client.go` / `fetch_remote_data.go`) to call the remote portal's `DeployStatusService.ListDeployStatus`, then project entries into the local readstore. Read those files and reuse the remote client builder.

- [ ] **Step 1: Add a guard** at the top of `Reconcile`: if `cr.Spec.IsRemote`, run a remote-fetch path (a small alternate chain or direct call) and return.
- [ ] **Step 2: Implement `fetch_remote.go`** using the shared remote client to call `ListDeployStatus` and `store.ReplaceForNamespace`.
- [ ] **Step 3: Write an envtest/unit** asserting a remote CR populates the readstore from a stub remote service.
- [ ] **Step 4: Run**

Run: `make test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/deploystatus/
git commit -m "feat(deploystatus): project remote portal deploy status (IsRemote)"
```

---

## Phase 10 — React UI

### Task 10.1: Deploy Status page + nav + feature gate

**Files:**
- Create: `web/src/features/deploy-status/` (page component + query hook using the generated Connect client `web/src/gen/.../deploystatus`)
- Modify: the router + sidebar registration files, and the feature-flag gate (mirror the image-inventory page).

> Read the existing image-inventory feature under `web/src/features/` and copy its structure: a page component, a TanStack Query hook calling the generated `DeployStatusService` client, a route entry, and a sidebar item gated by the portal feature flag (the frontend already reads portal features — mirror how `imageInventory` toggles its nav item).

- [ ] **Step 1: Build the query hook + page** listing services as cards (state badge: ok/behind/unresolved/error; behind shows aheadBy + pending commits + deploy-run link; truncated shows a "full diff" link).
- [ ] **Step 2: Register the route + sidebar item**, gated by `features.deployStatus`.
- [ ] **Step 3: Add a Vitest** for the page (mirror the image-inventory test) — renders entries from a mocked client.
- [ ] **Step 4: Run web checks**

Run: `cd web && npx tsc -b && npm run test`
Expected: types clean, tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/
git commit -m "feat(deploystatus): React Deploy Status page + nav (feature-gated)"
```

---

## Phase 11 — Helm values

### Task 11.1: Surface `deployStatus.forges` + per-`TokenEnv` secretKeyRef

**Files:**
- Modify: `helm/values.yaml`, the controller-manager Deployment template under `helm/` (env block), and the operator ConfigMap template that renders `OperatorConfig`.

> Run `make helm` first to regenerate from the kustomize base, then add the feature-specific values that aren't auto-generated (the `deployStatus` config block + env `secretKeyRef`s). Add the `deployStatus` feature flag to `portals.features` if that's surfaced in values (check the sreportal helm chart consumer).

- [ ] **Step 1: Add to `values.yaml`**

```yaml
deployStatus:
  enabled: false
  refreshInterval: 5m
  forges: []
    # PAT (fine-grained):
    # - host: github.com
    #   kind: github
    #   deployWorkflow: deploy-prod.yml
    #   auth:
    #     tokenEnv: GITHUB_TOKEN
    # GitHub App installation:
    # - host: github.com
    #   kind: github
    #   deployWorkflow: deploy-prod.yml
    #   auth:
    #     app:
    #       appID: 123456
    #       installationID: 7891011
    #       privateKeyEnv: GH_APP_PRIVATE_KEY
  # secretEnv maps an env var name (any tokenEnv / privateKeyEnv referenced above)
  # to a Secret key, rendered as env valueFrom.secretKeyRef:
  secretEnv: {}
    # GITHUB_TOKEN:
    #   name: deploystatus-forge-secrets
    #   key: github-token
    # GH_APP_PRIVATE_KEY:
    #   name: deploystatus-forge-secrets
    #   key: app-private-key
```

- [ ] **Step 2: Render env in the Deployment template** — for each `secretEnv` entry, emit an env var with `valueFrom.secretKeyRef` (covers both PAT `tokenEnv` and App `privateKeyEnv`).
- [ ] **Step 3: Render the `deployStatus` block into the operator ConfigMap** consumed by `config.LoadFromFile`.
- [ ] **Step 4: Verify the chart renders**

Run: `helm template ./helm --set deployStatus.enabled=true --set 'deployStatus.forges[0].host=github.com' --set 'deployStatus.forges[0].tokenEnv=GITHUB_TOKEN' | grep -A3 -i deploystatus`
Expected: config + env rendered correctly. **Re-read the rendered YAML for structural correctness** (indentation, block placement) — not just that the command exits 0.

- [ ] **Step 5: Commit**

```bash
git add helm/
git commit -m "feat(deploystatus): Helm values for forges + token secretKeyRefs"
```

---

## Phase 12 — Full verification + docs

### Task 12.1: Repo verification gates

- [ ] **Step 1: Generate everything**

Run: `make generate-all` (`manifests generate proto`)
Expected: no diff beyond intended generated files.

- [ ] **Step 2: Helm + tests + lint + docs**

Run: `make helm && make test && make lint && make doc`
Expected: all green; coverage ≥ 80% on new packages.

- [ ] **Step 3: Manual review pass** — re-read every changed non-generated file (handlers, controller, service, webhook, values) for structural correctness and intent match (per the "relecture avant terminé" rule). Verify `helm template` output structure.

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "docs(deploystatus): regenerate docs and manifests"
```

---

## Self-Review notes (for the implementer)

- **Pattern fidelity over invention.** Every "copy file X" instruction means: open X, replicate its exact signatures and conventions (finalizer naming, CreateOrUpdate, feature-gate calls, marshal helpers). The code blocks here are correct in shape but the neighbouring files are the source of truth for the exact controller-runtime/connect/mcp idioms.
- **Generated files**: never hand-edit; always regenerate (`make generate-all`, `make proto`, `make manifests`).
- **TDD**: pure-logic packages (`forge`, `chain/compute`, `readstore`) have plain `testing` tests written first; controller/webhook integration uses Ginkgo+envtest mirroring existing suites.
- **Open implementer decision (Task 5.2)**: whether the OCI-label fetch lives in the image-source projection step or in `resolve_oci_source`. Either is fine; keep `resolve_oci_source` authoritative when `SourceURL`/`DeployedRef` are empty.
