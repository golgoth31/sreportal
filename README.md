# SRE Portal

A Kubernetes operator that discovers DNS records, tracks service status, monitors network flows, and manages image inventories across your cluster — all presented in a unified web dashboard.

## Features

- **DNS Discovery** — Automatically discover DNS records from Services, Ingresses, Istio Gateways, Gateway API routes (HTTPRoute, GRPCRoute, TLSRoute, TCPRoute, UDPRoute), and external-dns endpoints across all namespaces
- **Portal Routing** — Organize endpoints into multiple portals using simple Kubernetes annotations (`sreportal.io/portal`)
- **Remote Portals** — Federate DNS data across clusters by connecting portals to remote SRE Portal instances
- **Alertmanager Integration** — Link Prometheus Alertmanager instances to portals; display active alerts in the dashboard
- **Status Pages** — Declare Components, Incidents, and Maintenances; expose aggregated platform status per portal
- **Release Tracker** — Track and expose release entries per portal and date
- **Network Flow Discovery** — Map service-to-service network flows via FlowObserver; query topology with AI tools
- **Image Inventory** — Track container images across registries, detect available upgrades and runtime mutations
- **Web Dashboard** — React-powered SPA with Links (FQDNs), Alerts, and Status pages; sidebar navigation; light/dark theme
- **MCP Servers** — 7 built-in [Model Context Protocol](https://modelcontextprotocol.io/) endpoints for AI assistants
- **Connect API** — gRPC-compatible [Connect protocol](https://connectrpc.com) API for FQDNs, portals, and alerts
- **Flexible Grouping** — Group FQDNs by annotation, label, namespace, or custom rules
- **Single Container** — Controller, gRPC API, web UI, and MCP servers all run in one container

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                   SRE Portal Pod                     │
│                                                      │
│  ┌──────────────┐  ┌─────────────┐  ┌────────────┐  │
│  │  Controllers  │  │ Connect API │  │   Web UI   │  │
│  │ (ctrl-runtime)│  │  (gRPC/h2c) │  │  (Echo v5) │  │
│  └──────┬───────┘  └──────┬──────┘  └─────┬──────┘  │
│         │                 │               │          │
│         └─────────┬───────┴───────┬───────┘          │
│                   │               │                  │
│            K8s API Server    MCP (/mcp, /mcp/*)      │
└──────────────────────────────────────────────────────┘
```

### CRDs

| CRD | Group | Description |
|-----|-------|-------------|
| **Portal** | v1alpha1 | Named dashboard view with optional remote federation |
| **DNS** | v1alpha2 | Manual DNS entry groups linked to a portal |
| **DNSRecord** | v1alpha2 | Auto-discovered endpoints (managed by the operator) |
| **Alertmanager** | v1alpha1 | Alertmanager instance linked to a portal |
| **Component** | v1alpha1 | Platform component with operational status |
| **Incident** | v1alpha1 | Declared incident with timeline updates |
| **Maintenance** | v1alpha1 | Scheduled maintenance window |
| **Release** | v1alpha1 | Release entry for the release tracker |
| **FlowObserver** | v1alpha1 | Network flow discovery configuration |
| **NetworkFlowDiscovery** | v1alpha1 | Discovered network flow topology |
| **ImageInventory** | v1alpha1 | Container image inventory linked to a registry |
| **ImageRegistry** | v1alpha1 | Image registry configuration |

## Quick Start

### Prerequisites

- Kubernetes cluster v1.28+
- `kubectl` configured to access the cluster
- Helm 3+ (for Helm install)

### Install with Helm

```bash
helm install sreportal oci://ghcr.io/golgoth31/charts/sreportal \
  --namespace sreportal-system --create-namespace
```

### Access the Dashboard

```bash
kubectl port-forward -n sreportal-system svc/sreportal-controller-manager 8090:8090
```

Open [http://localhost:8090](http://localhost:8090) in your browser.

### Annotate Your Services

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-web-app
  annotations:
    external-dns.alpha.kubernetes.io/hostname: "myapp.example.com"
    sreportal.io/portal: "main"           # optional: route to a specific portal
    sreportal.io/groups: "Backend,APIs"   # optional: assign to groups
spec:
  type: ClusterIP
  ports:
    - port: 80
```

## MCP Servers

SRE Portal exposes 7 MCP servers (Streamable HTTP) for AI assistant integration.

MCP must be enabled with `--enable-mcp` (disabled by default). Transport: `streamable-http` (default) or `stdio`.

| Endpoint | Tools |
|----------|-------|
| `/mcp` or `/mcp/dns` | `search_fqdns`, `list_portals`, `get_fqdn_details` |
| `/mcp/alerts` | `list_alerts` |
| `/mcp/status` | `list_components`, `list_maintenances`, `list_incidents`, `get_platform_status` |
| `/mcp/releases` | `list_releases` |
| `/mcp/netpol` | `list_network_flows`, `get_service_flows` |
| `/mcp/image` | `list_images`, `list_upgrades`, `list_mutations` |
| `/mcp/metrics` | `list_metrics` |

**Claude Code:**
```bash
claude mcp add sreportal --transport http http://localhost:8090/mcp
claude mcp add sreportal-alerts --transport http http://localhost:8090/mcp/alerts
claude mcp add sreportal-status --transport http http://localhost:8090/mcp/status
claude mcp add sreportal-netpol --transport http http://localhost:8090/mcp/netpol
claude mcp add sreportal-image --transport http http://localhost:8090/mcp/image
```

**Claude Desktop** (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "sreportal": { "transport": "http", "url": "http://localhost:8090/mcp" },
    "sreportal-alerts": { "transport": "http", "url": "http://localhost:8090/mcp/alerts" },
    "sreportal-status": { "transport": "http", "url": "http://localhost:8090/mcp/status" },
    "sreportal-netpol": { "transport": "http", "url": "http://localhost:8090/mcp/netpol" },
    "sreportal-image": { "transport": "http", "url": "http://localhost:8090/mcp/image" }
  }
}
```

## Documentation

Full documentation: [golgoth31.github.io/sreportal](https://golgoth31.github.io/sreportal/)

- [Getting Started](https://golgoth31.github.io/sreportal/docs/getting-started/)
- [Architecture](https://golgoth31.github.io/sreportal/docs/architecture/)
- [Configuration](https://golgoth31.github.io/sreportal/docs/configuration/)
- [Annotations](https://golgoth31.github.io/sreportal/docs/annotations/)
- [Web UI](https://golgoth31.github.io/sreportal/docs/web-ui/)
- [API Reference](https://golgoth31.github.io/sreportal/docs/api/)
- [Development](https://golgoth31.github.io/sreportal/docs/development/)

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Operator | Go 1.26, Kubebuilder, controller-runtime v0.23 |
| API | Connect protocol (connectrpc.com/connect) |
| Web UI | React 19, Vite, Tailwind CSS v4, shadcn/ui, TanStack Query v5 |
| MCP | Model Context Protocol (mark3labs/mcp-go) |
| Web server | Echo v5 with h2c |
| Codegen | Buf (protobuf) |
| DNS sources | sigs.k8s.io/external-dns |
| Testing | Ginkgo v2, Gomega, envtest |

## Development

```bash
make build          # Build manager binary
make run            # Run locally with current kubeconfig
make test           # Unit tests with envtest
make manifests      # Regenerate CRDs/RBAC
make proto          # Regenerate Go + TypeScript from proto
make build-web      # Build React app (Vite)
```

See [Development](https://golgoth31.github.io/sreportal/docs/development/) for the full guide.

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.
