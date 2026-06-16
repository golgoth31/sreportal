# Internal — Go Backend

## Controller Pattern: Chain of Responsibility

All controllers use a generic Chain-of-Responsibility framework defined in `reconciler/handler.go`:

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

## DNSRecord Spec/Status Contract

`DNSRecord.spec.entries` is the canonical channel for endpoints, for
both `origin=auto` and `origin=manual`:

- **manual**: user writes `spec.entries`.
- **auto**: the DNS controller writes `spec.entries` via CreateOrUpdate
  (idempotent — no spec write when source content is unchanged).
  The validating webhook (`internal/webhook/v1alpha2`) restricts
  origin=auto updates to the operator ServiceAccount, so human edits
  on auto records are rejected at admission. Any other write — should
  one slip through — is overwritten at the next DNS reconcile.

`status` is observed-only and written exclusively by the DNSRecord
controller (`MaterialiseEntriesHandler`, `ResolveDNSHandler`,
`ProjectStoreHandler`). The DNSRecord controller's `For()` watch uses
`GenerationChangedPredicate`, so status patches do not re-trigger the
controller (no feedback loop).

DNS-resolution drift checks (`ResolveDNSHandler`) run on every reconcile.
Because spec changes are sparse, a constant `DNSRecordResolveInterval`
(1h) requeue keeps drift detection active without coupling it to the
source poll cadence.

## cmd/main.go Setup

Registers:
1. **DNSReconciler** - Chain-based reconciliation with field indexer on `spec.portalRef`
2. **SourceReconciler** - Periodic external-dns source polling (manager.Runnable)
3. **PortalReconciler** - Simple status updates + EnsureMainPortalRunnable
4. **AlertmanagerReconciler** - Chain FetchAlerts → UpdateStatus (Alertmanager API client injected)
5. **DNSWebhook** - Validates `spec.portalRef` exists
6. **Web server** (goroutine) - Echo v5 with h2c serving Connect handlers + React SPA + MCP at `/mcp`, `/mcp/dns`, `/mcp/alerts`, `/mcp/metrics`, `/mcp/releases`, `/mcp/netpol`, `/mcp/status`, `/mcp/image`

All ReadStores are created in `cmd/main.go` and wired: controllers receive writers, gRPC/MCP receive readers.

K8s scheme registers: core types, external-dns v1alpha1, sreportal v1alpha1.

## ReadStore (CQRS Read Path)

All gRPC/MCP services read from in-memory ReadStores (`internal/readstore/`) instead of the K8s API. Controllers push projections after reconciliation.

- Generic `readstore.Store[T]` — `map[string][]T` + `RWMutex` + broadcast channel
- Domain interfaces in `internal/domain/<ctx>/reader.go`, `writer.go`
- Store implementations in `internal/readstore/<ctx>/`
- Bounded contexts: DNS, Portal, Alertmanager, Network Policy (dual stores + portalRef), Release

## Operator Configuration

ConfigMap-driven configuration (`config/`):

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

## Testing

- **Unit tests**: Ginkgo + Gomega with envtest (`make test`)
- **E2E tests**: Kind cluster with `-tags=e2e` (`make test-e2e`)
- **Suite setup**: `**/suite_test.go` (BeforeSuite/AfterSuite for envtest)

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

## Auto-generated Files — DO NOT EDIT

- `grpc/gen/*` (Buf generated)
- `**/zz_generated.*.go` (kubebuilder)
