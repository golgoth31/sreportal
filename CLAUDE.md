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

## Repository Structure (actual)

```
cmd/main.go                          # Manager entry point (controller + gRPC + web UI)
api/v1alpha1/
  dns_types.go                       # DNS CRD (manual entries + aggregated status)
  dnsrecord_types.go                 # DNSRecord CRD (external-dns source results)
  portal_types.go                    # Portal CRD (web UI portal configuration)
  alertmanager_types.go              # Alertmanager CRD (portalRef, url.local/remote, activeAlerts status)
  groupversion_info.go               # Group/version registration
  zz_generated.deepcopy.go          # Auto-generated
internal/
  adapter/
    endpoint.go                      # Converts external-dns endpoints to CRD status
  config/                            # ConfigMap-based operator configuration
  controller/
    dns_controller.go                # DNS reconciler (chain-based)
    source_controller.go             # DNSRecord reconciler (periodic, manager.Runnable)
    portal_controller.go             # Portal reconciler (simple status)
    alertmanager_controller.go     # Alertmanager reconciler (chain: FetchAlerts → UpdateStatus)
    dns/                             # Chain handlers for DNS reconciliation
      aggregate_dnsrecords.go        # Fetch DNSRecords by portalRef
      collect_manual_entries.go      # Extract manual groups from DNS.spec
      aggregate_fqdns.go             # Merge external + manual groups
      update_status.go               # Write aggregated groups to DNS.status
    alertmanager/                    # Chain handlers for Alertmanager reconciliation
      fetch_alerts.go                # Fetch active alerts from spec.url.local (Alertmanager API v2)
      update_status.go               # Write ActiveAlerts + conditions to status
    portal/                          # Portal controller utilities
  domain/
    dns/                             # Pure FQDN domain types (no external deps)
    alertmanager/                    # Alert, Fetcher interface, sentinel errors
  alertmanagerclient/               # HTTP client for Alertmanager API (GET /api/v2/alerts)
  reconciler/
    handler.go                       # Generic Chain-of-Responsibility framework (generics)
  grpc/
    dns_service.go                   # DNSService Connect implementation
    portal_service.go                # PortalService Connect implementation
    alertmanager_service.go          # AlertmanagerService Connect implementation (ListAlerts)
    gen/                             # Auto-generated (buf) - DO NOT EDIT
  mcp/
    server.go                        # DNSServer (DNS + portal tools), mount at /mcp and /mcp/dns
    alerts_server.go                 # AlertsServer (list_alerts), mount at /mcp/alerts
    search_fqdns.go                  # search_fqdns tool handler
    list_portals.go                  # list_portals tool handler
    get_fqdn_details.go              # get_fqdn_details tool handler
  source/                            # external-dns source factory
  webhook/v1alpha1/
    dns_webhook.go                   # DNS validation (portalRef exists)
  webserver/
    server.go                        # Echo v5 server (static files + Connect handlers + MCP)
proto/sreportal/v1/
  dns.proto                          # DNSService (ListFQDNs, StreamFQDNs)
  portal.proto                       # PortalService (ListPortals)
  alertmanager.proto                 # AlertmanagerService (ListAlerts)
web/src/
  main.tsx                           # React entry point (QueryClient + RouterProvider)
  router.tsx                         # Routes + ErrorPage + PageSkeleton
  index.css                          # Tailwind + OKLCH theme tokens (light + dark)
  hooks/
    useTheme.ts                      # Light/dark/system theme hook (localStorage)
  components/
    RootLayout.tsx                   # Root layout (TooltipProvider, header, footer, Toaster)
    PortalNav.tsx                    # Portal navigation (NavLink for local, <a> for remote)
    PortalSidebar.tsx                # Left sidebar: Links + Alerts (when portal has Alertmanagers)
    ThemeToggle.tsx                  # Theme cycle button (light → dark → system)
    ui/                              # shadcn/ui components (button, badge, table, …)
  pages/
    LinksPage.tsx                    # Container: search/group filters + FqdnGroupList
    AlertsPage.tsx                   # Container: alerts per portal (search/state filters + AlertList)
  features/
    dns/
      domain/dns.types.ts            # Fqdn, FqdnGroup types + isSynced/hasSyncStatus helpers
      infrastructure/dnsApi.ts       # listFqdns() — module-level transport singleton
      hooks/useDns.ts                # TanStack Query + client-side filter/group logic
      ui/FqdnCard.tsx                # Single FQDN (sync dot, copy, targets, originRef)
      ui/FqdnGroupCard.tsx           # Collapsible group card
      ui/FqdnGroupList.tsx           # List with loading skeleton + empty state
    portal/
      domain/portal.types.ts         # Portal, RemoteSyncStatus types
      infrastructure/portalApi.ts    # listPortals() — module-level transport singleton
      hooks/usePortals.ts            # TanStack Query (30s stale)
    alertmanager/
      domain/alertmanager.types.ts   # Alert, AlertmanagerResource, formatAlertTime
      infrastructure/alertmanagerApi.ts  # listAlerts(portal?, search?, state?)
      hooks/useAlerts.ts             # TanStack Query per portal
      hooks/usePortalsWithAlerts.ts  # Set of portal names that have Alertmanagers (for sidebar)
      ui/AlertmanagerResourceCard.tsx # Collapsible card per Alertmanager CR + alerts table
      ui/AlertList.tsx               # List of resources + loading/empty state
    mcp/
      ui/McpPage.tsx                 # MCP setup page (DNS + Alerts endpoints, tools table)
  gen/                               # Auto-generated Connect clients (buf) - DO NOT EDIT
config/
  crd/bases/                         # Auto-generated CRD YAML - DO NOT EDIT
  rbac/                              # Auto-generated RBAC - DO NOT EDIT role.yaml
  samples/                           # Example manifests
```

## CRD Specifications (current)

### DNS

```go
// Spec
type DNSSpec struct {
    PortalRef string     `json:"portalRef"`         // Required: links to Portal
    Groups    []DNSGroup `json:"groups,omitempty"`   // Manual DNS entry groups
}
type DNSGroup struct {
    Name, Description string
    Entries           []DNSEntry  // FQDN + Description
}

// Status (aggregated by controller)
type DNSStatus struct {
    Groups            []FQDNGroupStatus  // Aggregated from external-dns + manual
    Conditions        []metav1.Condition
    LastReconcileTime *metav1.Time
}
type FQDNGroupStatus struct {
    Name, Description, Source string  // Source: "manual", "external-dns", or "remote"
    FQDNs                    []FQDNStatus
}
type FQDNStatus struct {
    FQDN, Description, RecordType string
    Targets                       []string
    LastSeen                      metav1.Time
}
```

### DNSRecord

```go
// Spec
type DNSRecordSpec struct {
    SourceType string `json:"sourceType"` // "service", "ingress", or "dnsendpoint"
    PortalRef  string `json:"portalRef"`  // Portal this record belongs to
}

// Status (populated by source controller)
type DNSRecordStatus struct {
    Endpoints         []EndpointStatus   // DNSName, RecordType, Targets, TTL, Labels
    LastReconcileTime *metav1.Time
    Conditions        []metav1.Condition
}
```

### Portal

```go
// Spec
type PortalSpec struct {
    Title   string            `json:"title"`              // Display title
    Main    bool              `json:"main,omitempty"`     // Default portal flag
    SubPath string            `json:"subPath,omitempty"`  // URL subpath (defaults to name)
    Remote  *RemotePortalSpec `json:"remote,omitempty"`   // Remote portal config (cannot be set if Main=true)
}

type RemotePortalSpec struct {
    URL    string            `json:"url"`              // Base URL of remote SRE Portal (required, ^https?://.*)
    Portal string            `json:"portal,omitempty"` // Portal name on remote (defaults to main)
    TLS    *RemoteTLSConfig  `json:"tls,omitempty"`    // TLS settings (defaults to system config)
}

type RemoteTLSConfig struct {
    InsecureSkipVerify bool       `json:"insecureSkipVerify,omitempty"` // Disable TLS cert verification
    CASecretRef        *SecretRef `json:"caSecretRef,omitempty"`        // Secret with "ca.crt" key
    CertSecretRef      *SecretRef `json:"certSecretRef,omitempty"`      // Secret with "tls.crt" and "tls.key" keys
}

type SecretRef struct {
    Name string `json:"name"` // Secret name (same namespace as Portal)
}

// Status
type PortalStatus struct {
    Ready      bool
    Conditions []metav1.Condition
    RemoteSync *RemoteSyncStatus  // Only populated when spec.remote is set
}

// RemoteSyncStatus contains status for remote portal synchronization
type RemoteSyncStatus struct {
    LastSyncTime  *metav1.Time  // Last successful sync timestamp
    LastSyncError string        // Error from last failed sync (empty if success)
    RemoteTitle   string        // Title fetched from remote portal
    FQDNCount     int           // Number of FQDNs fetched from remote
}
```

### Alertmanager

```go
// Spec
type AlertmanagerSpec struct {
    PortalRef string           `json:"portalRef"`         // Required: Portal this resource is linked to
    URL       *AlertmanagerURL `json:"url,omitempty"`     // Alertmanager API URLs
}
type AlertmanagerURL struct {
    Local  string `json:"local"`            // Required: URL used by controller to fetch alerts (e.g. http://alertmanager:9093)
    Remote string `json:"remote,omitempty"` // Optional: externally-reachable URL for dashboard links
}

// Status (populated by controller)
type AlertmanagerStatus struct {
    ActiveAlerts      []AlertStatus   // Firing alerts from GET /api/v2/alerts
    Conditions        []metav1.Condition
    LastReconcileTime *metav1.Time
}
type AlertStatus struct {
    Fingerprint string
    Labels, Annotations map[string]string
    State string       // active, suppressed, unprocessed
    StartsAt, EndsAt, UpdatedAt *metav1.Time
}
```

Controller fetches alerts from `spec.url.local` (Alertmanager API v2) at each reconciliation; `url.remote` is for UI links only.

**Remote Portal Feature:**
- When `spec.remote` is set, the Portal fetches DNS data from a remote SRE Portal instance
- The main portal (`spec.main=true`) cannot have `spec.remote` set (validated by webhook)
- Remote portals are excluded from local source collection (SourceController)
- PortalReconciler periodically syncs with remote portals (every 5 minutes)
- Remote portal status includes sync time, error state, and FQDN count
- TLS can be configured via `spec.remote.tls`: insecure mode, custom CA (Secret with `ca.crt`), mTLS client certs (Secret with `tls.crt`/`tls.key`)

## Controller Pattern: Chain of Responsibility

All controllers use a generic Chain-of-Responsibility framework defined in `internal/reconciler/handler.go`:

```go
type ReconcileContext[T any] struct {
    Resource T
    Result   ctrl.Result
    Data     map[string]any  // Shared data between steps
}

type Handler[T any] interface {
    Handle(ctx context.Context, rc *ReconcileContext[T]) error
}

type Chain[T any] struct { handlers []Handler[T] }
// Execute runs handlers sequentially; short-circuits on requeue or error
```

### DNS Controller Chain (4 steps)
1. **AggregateDNSRecords** - Fetch DNSRecords by portalRef, convert to groups
2. **CollectManualEntries** - Extract manual groups from DNS.spec.groups
3. **AggregateFQDNs** - Merge external + manual groups (manual wins on conflict)
4. **UpdateStatus** - Write aggregated groups to DNS.status

### Source Controller (DNSRecord)
Not chain-based. Implements `manager.Runnable` for periodic reconciliation (default: 5 min).
- Builds external-dns sources (Service, Ingress, DNSEndpoint)
- Routes endpoints to portals via `sreportal.io/portal` annotation (falls back to main portal)
- Creates/updates DNSRecord CR per `(portal, sourceType)` pair

### Portal Controller
Simple controller: sets `status.ready = true` with Ready condition.
Includes `EnsureMainPortalRunnable` that creates a default "main" portal on startup.

### Alertmanager Controller (Chain)
1. **FetchAlerts** - GET Alertmanager API at `spec.url.local` (/api/v2/alerts), store alerts in context
2. **UpdateStatus** - Write ActiveAlerts, LastReconcileTime, Ready condition to status
Requeue every 2 minutes.

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

See `INSTRUCTIONS.md` for full CRD specifications and design details.

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
