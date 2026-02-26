# SRE Portal

A Kubernetes operator that discovers DNS records from your cluster resources and presents them in a unified web dashboard. It integrates with external-dns sources (Services, Ingresses, Istio Gateways, DNSEndpoints) and supports manual DNS entries through Custom Resources.

## Features

- **DNS Discovery** -- Automatically discover DNS records from Services, Ingresses, Istio Gateways, and external-dns endpoints across all namespaces
- **Portal Routing** -- Organize endpoints into multiple portals using simple Kubernetes annotations (`sreportal.io/portal`)
- **Remote Portals** -- Federate DNS data across clusters by connecting portals to remote SRE Portal instances
- **Web Dashboard** -- Angular-powered SPA with search, filters, grouping, and light/dark theme served directly by the operator
- **MCP Server** -- Built-in [Model Context Protocol](https://modelcontextprotocol.io/) server for AI assistant integration (Claude Desktop, Claude Code, Cursor)
- **Connect API** -- gRPC-compatible [Connect protocol](https://connectrpc.com) API for listing and streaming FQDN updates
- **Flexible Grouping** -- Group FQDNs by annotation, label, namespace, or custom rules
- **Single Container** -- Controller, gRPC API, web UI, and MCP server all run in one container

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                   SRE Portal Pod                     │
│                                                      │
│  ┌──────────────┐  ┌─────────────┐  ┌────────────┐  │
│  │  Controllers  │  │ Connect API │  │   Web UI   │  │
│  │  (ctrl-runtime)│  │  (gRPC/h2c) │  │  (Echo v5) │  │
│  └──────┬───────┘  └──────┬──────┘  └─────┬──────┘  │
│         │                 │               │          │
│         └─────────┬───────┴───────┬───────┘          │
│                   │               │                  │
│            K8s API Server    MCP Server               │
│                              (/mcp endpoint)          │
└──────────────────────────────────────────────────────┘
```

SRE Portal defines three CRDs:

| CRD | Description |
|-----|-------------|
| **Portal** | Named web dashboard view with optional remote federation |
| **DNS** | Manual DNS entry groups linked to a portal |
| **DNSRecord** | Auto-discovered endpoints (managed by the operator) |

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
kubectl port-forward -n sreportal-system svc/sreportal-controller-manager 8082:8082
```

Open [http://localhost:8082](http://localhost:8082) in your browser.

### Annotate Your Services

The operator discovers DNS records from resources with the `external-dns.alpha.kubernetes.io/hostname` annotation:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-web-app
  annotations:
    external-dns.alpha.kubernetes.io/hostname: "myapp.example.com"
    sreportal.io/portal: "main"           # optional: route to a specific portal
    sreportal.io/groups: "Backend,APIs"    # optional: assign to groups
spec:
  type: ClusterIP
  ports:
    - port: 80
```

### Connect an AI Assistant (MCP)

SRE Portal exposes an MCP server at `/mcp` for AI-powered DNS lookups.

**Claude Code:**
```bash
claude mcp add sreportal --transport http http://localhost:8082/mcp
```

**Claude Desktop** (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "sreportal": {
      "transport": "http",
      "url": "http://localhost:8082/mcp"
    }
  }
}
```

Available MCP tools: `search_fqdns`, `list_portals`, `get_fqdn_details`.

## Documentation

Full documentation is available at the [documentation site](https://golgoth31.github.io/sreportal/):

- [Getting Started](https://golgoth31.github.io/sreportal/docs/getting-started/) -- Install and create your first portal
- [Architecture](https://golgoth31.github.io/sreportal/docs/architecture/) -- CRD relationships and controller patterns
- [Configuration](https://golgoth31.github.io/sreportal/docs/configuration/) -- Operator ConfigMap reference
- [Annotations](https://golgoth31.github.io/sreportal/docs/annotations/) -- Route endpoints and assign groups
- [Web UI](https://golgoth31.github.io/sreportal/docs/web-ui/) -- Dashboard features and routes
- [API Reference](https://golgoth31.github.io/sreportal/docs/api/) -- CRD field reference
- [Development](https://golgoth31.github.io/sreportal/docs/development/) -- Build, test, and contribute

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Operator | Go 1.25, Kubebuilder, controller-runtime v0.23 |
| API | Connect protocol (connectrpc.com/connect) |
| Web UI | Angular 19+, Angular Material, Signals |
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
make build-web      # Build Angular app
```

See [Development](https://golgoth31.github.io/sreportal/docs/development/) for the full guide.

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.
