---
title: MCP Server
weight: 6
---

SRE Portal includes built-in [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) servers that let AI assistants query DNS records and alerts.

## Endpoints

Several MCP servers are mounted on the same port as the web UI (8090 by default). All use **Streamable HTTP** transport.

| Endpoint | Description |
|----------|-------------|
| `/mcp` | DNS and portal tools (same as `/mcp/dns`; kept for backward compatibility) |
| `/mcp/dns` | DNS and portal tools |
| `/mcp/alerts` | Alertmanager alerts tools |
| `/mcp/metrics` | Prometheus metrics tools |
| `/mcp/releases` | Release tracking tools |
| `/mcp/netpol` | Network flow tools |
| `/mcp/image` | Image inventory tools |

Base URL: `http://<sreportal-host>:8090`.

## Available Tools

### DNS / Portals (at `/mcp` and `/mcp/dns`)

| Tool | Description | Parameters |
|------|-------------|------------|
| `search_fqdns` | Search for FQDNs matching criteria | `query`, `source`, `group`, `portal`, `namespace` |
| `list_portals` | List all available portals | _(none)_ |
| `get_fqdn_details` | Get detailed info about a specific FQDN | `fqdn` (required) |

### Alerts (at `/mcp/alerts`)

| Tool | Description | Parameters |
|------|-------------|------------|
| `list_alerts` | List Alertmanager resources and their active alerts (optionally filtered by portal, search, state) | `portal`, `search`, `state` (optional) |

### Metrics (at `/mcp/metrics`)

| Tool | Description | Parameters |
|------|-------------|------------|
| `list_metrics` | List Prometheus metrics from the operator's metrics registry | _(none)_ |

### Releases (at `/mcp/releases`)

| Tool | Description | Parameters |
|------|-------------|------------|
| `list_releases` | List release entries for a day | `day` (optional, YYYY-MM-DD; defaults to the latest day with data). Response includes `previous_day` and `next_day` |

### Network Flows (at `/mcp/netpol`)

| Tool | Description | Parameters |
|------|-------------|------------|
| `list_network_flows` | List all network flows (nodes and directional edges) between services, databases, crons, and external endpoints derived from Kubernetes NetworkPolicies and FQDNNetworkPolicies | `portal`, `namespace`, `search` (all optional) |
| `get_service_flows` | Get all incoming and outgoing flows for a specific service (which services call it and which services/databases/externals it calls) | `service` (required), `portal` (optional) |

### Image Inventory (at `/mcp/image`)

| Tool | Description | Parameters |
|------|-------------|------------|
| `list_images` | List container images discovered by ImageInventory resources. Returns images with tag type (semver, commit, digest, latest, other), registry, repository, and the workloads using them | `portal`, `search`, `registry`, `tag_type` (all optional) |

## Setup

### Claude Code

**DNS and portals:**
```bash
claude mcp add sreportal --transport http http://localhost:8090/mcp
```

**Alerts:**
```bash
claude mcp add sreportal-alerts --transport http http://localhost:8090/mcp/alerts
```

**Metrics:**
```bash
claude mcp add sreportal-metrics --transport http http://localhost:8090/mcp/metrics
```

**Releases:**
```bash
claude mcp add sreportal-releases --transport http http://localhost:8090/mcp/releases
```

**Network flows:**
```bash
claude mcp add sreportal-netpol --transport http http://localhost:8090/mcp/netpol
```

**Image inventory:**
```bash
claude mcp add sreportal-image --transport http http://localhost:8090/mcp/image
```

### Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "sreportal": {
      "transport": "http",
      "url": "http://localhost:8090/mcp"
    },
    "sreportal-alerts": {
      "transport": "http",
      "url": "http://localhost:8090/mcp/alerts"
    },
    "sreportal-metrics": {
      "transport": "http",
      "url": "http://localhost:8090/mcp/metrics"
    },
    "sreportal-releases": {
      "transport": "http",
      "url": "http://localhost:8090/mcp/releases"
    },
    "sreportal-netpol": {
      "transport": "http",
      "url": "http://localhost:8090/mcp/netpol"
    },
    "sreportal-image": {
      "transport": "http",
      "url": "http://localhost:8090/mcp/image"
    }
  }
}
```

### Cursor

Add MCP servers:

- **DNS/portals**: Type URL, URL: `http://localhost:8090/mcp` (or `http://localhost:8090/mcp/dns`)
- **Alerts**: Type URL, URL: `http://localhost:8090/mcp/alerts`
- **Metrics**: Type URL, URL: `http://localhost:8090/mcp/metrics`
- **Releases**: Type URL, URL: `http://localhost:8090/mcp/releases`
- **Network flows**: Type URL, URL: `http://localhost:8090/mcp/netpol`
- **Image inventory**: Type URL, URL: `http://localhost:8090/mcp/image`

## Example Queries

Once connected to the DNS server, you can ask:

- "List all available portals"
- "Search for FQDNs containing `api`"
- "Get details for `api.example.com`"
- "Show all external-dns entries in the production portal"
- "What DNS records are in the monitoring group?"

With the alerts server:

- "List active alerts for the main portal"
- "Show firing alerts"
- "List alerts containing 'disk'"

With the metrics server:

- "Show me all SRE Portal metrics"
- "What's the current reconciliation rate?"

With the releases server:

- "List today's releases"
- "Add a deployment release for v2.1.0 from CI/CD"
- "Show releases for 2026-03-19"

With the network flows server:

- "List all network flows in the main portal"
- "Which services call the payment service?"
- "Show incoming and outgoing flows for `api`"

With the image inventory server:

- "List all container images in the main portal"
- "Which workloads use images tagged `latest`?"
- "Show images from `ghcr.io`"
- "List images with semver tags"

## In-App Help

The web UI includes a Help page at `/help` with the same setup instructions and a live display of all MCP endpoint URLs and their tools.
