---
title: MCP Server
weight: 6
---

SRE Portal includes a built-in [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server that lets AI assistants query your DNS records directly.

## Endpoint

The MCP server is mounted at `/mcp` on the same port as the web UI (8082 by default). It uses **Streamable HTTP** transport.

```
http://<sreportal-host>:8082/mcp
```

## Available Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `search_fqdns` | Search for FQDNs matching criteria | `query`, `source`, `group`, `portal`, `namespace` |
| `list_portals` | List all available portals | _(none)_ |
| `get_fqdn_details` | Get detailed info about a specific FQDN | `fqdn` (required) |

## Setup

### Claude Code

```bash
claude mcp add sreportal --transport http http://localhost:8082/mcp
```

### Claude Desktop

Add to your `claude_desktop_config.json`:

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

### Cursor

Add a new MCP server with:
- **Type**: URL
- **URL**: `http://localhost:8082/mcp`

## Example Queries

Once connected, you can ask your AI assistant:

- "List all available portals"
- "Search for FQDNs containing `api`"
- "Get details for `api.example.com`"
- "Show all external-dns entries in the production portal"
- "What DNS records are in the monitoring group?"

## In-App Help

The web UI includes a Help page at `/help` with the same setup instructions and a live display of the MCP endpoint URL for the current instance.
