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

- **Operator**: Go 1.26, Kubebuilder, controller-runtime v0.23
- **API**: Connect protocol (connectrpc.com/connect v1.19), Buf for codegen
- **Web server**: Echo v5 with h2c (HTTP/2 without TLS)
- **Web UI**: React 19, Vite, Tailwind CSS v4, shadcn/ui, TanStack Query v5, React Router v7
- **External DNS**: sigs.k8s.io/external-dns v0.20
- **Testing**: Ginkgo v2 + Gomega with envtest (Go), Vitest (Web)
- **MCP**: Model Context Protocol (mark3labs/mcp-go)
- **Metrics**: Custom Prometheus metrics (prometheus/client_golang) registered on controller-runtime registry
- **Deployment**: Single container (controller + gRPC + web UI + MCP)

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
make helm
make doc
```

**Always use kubebuilder CLI to create new APIs and webhooks. Never create these files manually.**

```bash
kubebuilder create api --group sreportal --version v1alpha1 --kind <Kind>
kubebuilder create webhook --group sreportal --version v1alpha1 --kind <Kind> \
  --defaulting --programmatic-validation
```

## External Dependencies

- `sigs.k8s.io/external-dns` - DNSEndpoint CRD and source interfaces
- `connectrpc.com/connect` - gRPC-compatible Connect protocol
- `github.com/labstack/echo/v5` - HTTP server with h2c support
- `github.com/mark3labs/mcp-go` - Model Context Protocol server
- `sigs.k8s.io/controller-runtime` - Kubernetes operator framework
- `golang.org/x/net` - HTTP/2 cleartext support

## Workflow

- Conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`
- PRs require: constitutional compliance, tests, 80%+ coverage, lint pass, generated code up to date
- After changes: `make helm` -> `make test` -> `make lint` -> `make doc`
