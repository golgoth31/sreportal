---
title: MCP Server
weight: 6
---

SRE Portal includes built-in [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) servers that let AI assistants query DNS records and alerts.

## Endpoints

Two MCP servers are mounted on the same port as the web UI (8082 by default). Both use **Streamable HTTP** transport.

| Endpoint | Description |
|----------|-------------|
| `/mcp` | DNS and portal tools (same as `/mcp/dns`; kept for backward compatibility) |
| `/mcp/dns` | DNS and portal tools |
| `/mcp/alerts` | Alertmanager alerts tools |

Base URL: `http://<sreportal-host>:8082`.

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

## Setup

### Claude Code

**DNS and portals:**
```bash
claude mcp add sreportal --transport http http://localhost:8082/mcp
```

**Alerts:**
```bash
claude mcp add sreportal-alerts --transport http http://localhost:8082/mcp/alerts
```

### Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "sreportal": {
      "transport": "http",
      "url": "http://localhost:8082/mcp"
    },
    "sreportal-alerts": {
      "transport": "http",
      "url": "http://localhost:8082/mcp/alerts"
    }
  }
}
```

### Cursor

Add two MCP servers:

- **DNS/portals**: Type URL, URL: `http://localhost:8082/mcp` (or `http://localhost:8082/mcp/dns`)
- **Alerts**: Type URL, URL: `http://localhost:8082/mcp/alerts`

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

## In-App Help

The web UI includes a Help page at `/help` with the same setup instructions and a live display of both MCP endpoint URLs and their tools.
