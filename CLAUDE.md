# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SRE Portal is a Kubernetes operator with a web dashboard for managing service status pages and DNS discovery across multiple clusters.

### Current Implementation Status

**Implemented:**
- DNS discovery: 4 CRDs (DNS, DNSRecord, Portal, Alertmanager), Chain-of-Responsibility reconciliation, external-dns integration
- Portal routing via `sreportal.io/portal` annotation on K8s resources
- Alertmanager: CRD linked to Portal (portalRef), URL (local/remote), controller fetches active alerts from Alertmanager API and stores them in status
- Web UI: React 19 app (Vite + shadcn/ui) with Links page (FQDN display), Alerts page (per-portal), left sidebar (Links / Alerts)
- Connect protocol gRPC API (DNSService, PortalService, AlertmanagerService)
- ConfigMap-driven operator configuration
- MCP: two servers — DNS/portals at `/mcp` and `/mcp/dns`, alerts at `/mcp/alerts` (Streamable HTTP)

**Not yet implemented:**
- Status pages (Component, Incident, Maintenance CRDs)
- Additional web pages (status page UI)

## Architecture Principles

- **DDD (Domain-Driven Design)**: Separate domain logic from infrastructure
- **Clean Architecture**: Dependencies point inward (domain has no external dependencies)
- **Chain of Responsibility**: All controllers use this pattern for reconciliation steps
- **Idempotent reconciliation**: Controllers safe to run multiple times
- **Owner references**: Enable garbage collection with `SetControllerReference`

## Key Technologies

- **Operator**: Go 1.25, Kubebuilder, controller-runtime v0.23
- **API**: Connect protocol (connectrpc.com/connect v1.19), Buf for codegen
- **Web server**: Echo v5 with h2c (HTTP/2 without TLS)
- **Web UI**: React 19, Vite, Tailwind CSS v4, shadcn/ui, TanStack Query v5, React Router v7
- **External DNS**: sigs.k8s.io/external-dns v0.20
- **Testing**: Ginkgo v2 + Gomega with envtest
- **MCP**: Model Context Protocol (mark3labs/mcp-go): DNS server on `/mcp` and `/mcp/dns`, Alerts server on `/mcp/alerts`
- **Metrics**: Custom Prometheus metrics (prometheus/client_golang) registered on controller-runtime registry (`/metrics`)
- **Deployment**: Single container (controller + gRPC + web UI + MCP)

## Common Commands

```bash
# Development
make build              # Build manager binary to bin/manager
make run                # Run controller locally (uses current kubeconfig)
make manifests          # Generate CRDs/RBAC from kubebuilder markers
make generate           # Generate DeepCopy methods
make proto              # Generate Go + TypeScript from proto (buf)
make proto-lint         # Lint proto files with buf

# Testing
make test               # Unit tests with envtest (K8s API + etcd in-memory)
make test-e2e           # E2E tests on isolated Kind cluster
go test ./path/to/package -run TestName -v  # Run single test

# Linting
make lint               # Run golangci-lint
make lint-fix           # Auto-fix lint issues

# Web UI
make install-web        # npm install
make build-web          # Build React app (Vite)
npm test --prefix web   # Web unit tests (Vitest)

# Deployment
make docker-build IMG=<registry>/<image>:<tag>
make docker-push IMG=<registry>/<image>:<tag>
make install            # Install CRDs to cluster
make deploy IMG=<registry>/<image>:<tag>
```

## Controller Pattern: Chain of Responsibility

All controllers use a generic Chain-of-Responsibility framework defined in `internal/reconciler/handler.go`:

```go
type ReconcileContext[T any, D any] struct {
    Resource T
    Result   ctrl.Result
    Data     D  // Typed shared data between steps (e.g. dns.ChainData, alertmanager.ChainData)
}

type Handler[T any, D any] interface {
    Handle(ctx context.Context, rc *ReconcileContext[T, D]) error
}

type Chain[T any, D any] struct { handlers []Handler[T, D] }
// Execute runs handlers sequentially; short-circuits on requeue or error
```

## gRPC/Connect API

### DNSService (`proto/sreportal/v1/dns.proto`)
- `ListFQDNs` - Lists all FQDNs (supports filters: namespace, source, search, portal)
- `StreamFQDNs` - Streams FQDN updates (polls every 5s)

### PortalService (`proto/sreportal/v1/portal.proto`)
- `ListPortals` - Lists all portals

### AlertmanagerService (`proto/sreportal/v1/alertmanager.proto`)
- `ListAlerts` - Lists Alertmanager resources with active alerts (filters: portal, namespace, search, state)

## Web UI (React 19)

Single page app with:
- **Stack**: React 19 + Vite + Tailwind CSS v4 + shadcn/ui + TanStack Query v5 + React Router v7
- **Build output**: `web/dist/web/browser/` (embedded into Go binary via `ui_embed.go`)
- **Architecture**: Feature-based Clean Architecture — domain → infrastructure → hooks → ui layers
- **Routes**: `''` → `/main/links`, `:portalName/links` → LinksPage, `:portalName/alerts` → AlertsPage, `/help` → McpPage
- **Sidebar**: Left menu per portal with Links and Alerts (Alerts only when portal has Alertmanager resources)
- **LinksPage**: FQDNs grouped by group name, search + group filter, sync status dots
- **AlertsPage**: Alertmanager resources with active alerts (search/state filters), collapsible cards per resource
- **State**: TanStack Query for server state (5s polling for FQDNs, 30s stale for portals)
- **Connect clients**: Module-level transport singletons in `dnsApi.ts`, `portalApi.ts`, `alertmanagerApi.ts`
- **Theme**: Light / dark / system toggle, stored in localStorage, applied via `.dark` class
- **shadcn/ui components installed**: button, skeleton, sonner, tooltip, badge, input, select, collapsible, separator, table
- **Error handling**: Router-level `errorElement` on all routes; error alert in LinksPage
- **`web/src/gen/`**: Auto-generated Connect clients (buf) — DO NOT EDIT; `erasableSyntaxOnly` disabled in tsconfig.app.json due to generated TypeScript enums

## Operator Configuration

ConfigMap-driven configuration (`internal/config/`):

```go
type OperatorConfig struct {
    Sources        SourcesConfig         // Service, Ingress, DNSEndpoint toggles
    GroupMapping   GroupMappingConfig     // FQDN grouping rules
    Reconciliation ReconciliationConfig  // Timing
}
type GroupMappingConfig struct {
    DefaultGroup string
    LabelKey     string
    ByNamespace  map[string]string  // Namespace -> group mapping
}
```

## Observability (Metrics)

All custom metrics are defined in `internal/metrics/metrics.go` and registered on the
controller-runtime Prometheus registry. They are served on the standard `/metrics` endpoint
(bind address controlled by `--metrics-bind-address`, default `0` = disabled).

### Custom Metrics Reference

#### Controller metrics (`sreportal_controller_*`)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_controller_reconcile_total` | Counter | `controller`, `result` | Reconciliation count (success / error) |
| `sreportal_controller_reconcile_duration_seconds` | Histogram | `controller` | Reconciliation latency |

#### DNS metrics (`sreportal_dns_*`)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_dns_fqdns_total` | Gauge | `portal`, `source` | Number of FQDNs per portal and source |
| `sreportal_dns_groups_total` | Gauge | `portal` | Number of DNS groups per portal |

#### Source metrics (`sreportal_source_*`)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_source_endpoints_collected` | Gauge | `source_type` | Endpoints collected per source type |
| `sreportal_source_errors_total` | Counter | `source_type` | Source collection errors |

#### Alertmanager metrics (`sreportal_alertmanager_*`)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_alertmanager_alerts_active` | Gauge | `portal`, `alertmanager` | Active alerts per resource |
| `sreportal_alertmanager_fetch_errors_total` | Counter | `alertmanager` | Alert fetch errors |

#### Portal metrics (`sreportal_portal_*`)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_portal_total` | Gauge | `type` | Portals by type (local / remote) |
| `sreportal_portal_remote_sync_errors_total` | Counter | `portal` | Remote sync errors |
| `sreportal_portal_remote_fqdns_synced` | Gauge | `portal` | FQDNs synced from remote |

#### HTTP server metrics (`sreportal_http_*`)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_http_requests_total` | Counter | `method`, `handler`, `code` | HTTP requests (handler: `connect`, `mcp`, `api`, `static`) |
| `sreportal_http_request_duration_seconds` | Histogram | `method`, `handler` | HTTP request latency |
| `sreportal_http_requests_in_flight` | Gauge | — | In-flight HTTP requests |

#### MCP server metrics (`sreportal_mcp_*`)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sreportal_mcp_tool_calls_total` | Counter | `server`, `tool` | MCP tool invocations |
| `sreportal_mcp_tool_call_duration_seconds` | Histogram | `server`, `tool` | MCP tool call latency |
| `sreportal_mcp_tool_call_errors_total` | Counter | `server`, `tool` | MCP tool call errors |
| `sreportal_mcp_sessions_active` | Gauge | `server` | Active MCP sessions (dns / alerts) |

### Grafana Dashboard

A pre-built Grafana dashboard is available at `config/grafana/sreportal-dashboard.json`.

**Import**: Grafana > Dashboards > Import > Upload JSON file.

**Variables**:
- `datasource` — Prometheus datasource picker (type: `datasource`, query: `prometheus`)
- `job` — Prometheus job filter, multi-select with "All" (auto-discovered from `sreportal_controller_reconcile_total`)

**Row 1 — Application Metrics**:
- Reconciliations / sec (by controller and result)
- Reconciliation duration (p50 / p95 / p99)
- FQDNs total and DNS groups (stat panels)
- Active alerts and remote FQDNs synced (stat panels with thresholds)
- HTTP requests / sec (stacked by handler and status code)
- HTTP latency (p50 / p95 / p99)
- HTTP in-flight and MCP active sessions
- MCP tool calls / sec (bar chart by server/tool)
- Source endpoints collected
- Errors / sec (source + alertmanager + remote sync + MCP combined)
- Portals gauge (local / remote)

**Row 2 — System Metrics (sreportal process)**:
- CPU usage (`process_cpu_seconds_total`)
- Memory (RSS, heap alloc, heap in-use, stack in-use)
- Goroutines (`go_goroutines`)
- Open file descriptors (open vs max)
- GC duration (p50 / p75 / p100)
- K8s REST client requests / sec (`rest_client_requests_total` stacked)
- Workqueue depth per controller

## cmd/main.go Setup

Registers:
1. **DNSReconciler** - Chain-based reconciliation with field indexer on `spec.portalRef`
2. **SourceReconciler** - Periodic external-dns source polling (manager.Runnable)
3. **PortalReconciler** - Simple status updates + EnsureMainPortalRunnable
4. **AlertmanagerReconciler** - Chain FetchAlerts → UpdateStatus (Alertmanager API client injected)
5. **DNSWebhook** - Validates `spec.portalRef` exists
6. **Web server** (goroutine) - Echo v5 with h2c serving Connect handlers + React SPA + MCP at `/mcp`, `/mcp/dns`, `/mcp/alerts`

K8s scheme registers: core types, external-dns v1alpha1, sreportal v1alpha1.

## Critical Rules

**Never edit (auto-generated):**
- `config/crd/bases/*.yaml`
- `config/rbac/role.yaml`
- `config/webhook/manifests.yaml`
- `**/zz_generated.*.go`
- `PROJECT`
- `internal/grpc/gen/*` (Buf generated)
- `web/src/gen/*` (Buf generated)

**Never remove scaffold markers:**
```go
// +kubebuilder:scaffold:*
```

**After editing `*_types.go` or markers:**
```bash
make manifests generate
```

**Always use kubebuilder CLI to create new APIs and webhooks. Never create these files manually.**

```bash
kubebuilder create api --group sreportal --version v1alpha1 --kind <Kind>
kubebuilder create webhook --group sreportal --version v1alpha1 --kind <Kind> \
  --defaulting --programmatic-validation
```

## Planned CRDs (from INSTRUCTIONS.md, not yet implemented)

- **Cluster** - Operator chaining / multi-cluster federation
- **Component** - Health monitoring with HTTP/gRPC/K8s/Prometheus checks
- **Incident** - Incident tracking with status updates and severity
- **Maintenance** - Scheduled maintenance windows

## Testing

- **Unit tests**: Ginkgo + Gomega with envtest (`make test`)
- **E2E tests**: Kind cluster with `-tags=e2e` (`make test-e2e`)
- **Suite setup**: `**/suite_test.go` (BeforeSuite/AfterSuite for envtest)
- **Web tests**: `npm test --prefix web`

### Test Structure (BDD style)
```go
var _ = Describe("Controller", func() {
    Context("when resource is created", func() {
        It("should reconcile", func() {
            Eventually(func(g Gomega) { /* assertions */ }).Should(Succeed())
        })
    })
})
```

## External Dependencies

- `sigs.k8s.io/external-dns` - DNSEndpoint CRD and source interfaces
- `connectrpc.com/connect` - gRPC-compatible Connect protocol
- `github.com/labstack/echo/v5` - HTTP server with h2c support
- `github.com/mark3labs/mcp-go` - Model Context Protocol server (served on `/mcp`)
- `sigs.k8s.io/controller-runtime` - Kubernetes operator framework
- `golang.org/x/net` - HTTP/2 cleartext support
