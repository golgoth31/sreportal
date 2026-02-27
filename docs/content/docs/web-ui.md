---
title: Web UI
weight: 5
---

SRE Portal includes a React single-page application served directly by the operator.

![SRE Portal Web UI](/assets/img/web.png)

## Accessing the Dashboard

The web UI is served on port 8082 by default. Forward the port to access it locally:

```bash
kubectl port-forward -n sreportal-system svc/sreportal-controller-manager 8082:8082
```

Then open [http://localhost:8082](http://localhost:8082) in your browser.

## Routes

| Route | Description |
|-------|-------------|
| `/` | Redirects to `/main/links` |
| `/:portalName/links` | Displays FQDNs for the specified portal |
| `/help` | MCP setup instructions and available tools |

The root URL redirects to the `main` portal's links page. Each portal defined in the cluster gets its own route.

## Features

### FQDN Display

The links page shows all FQDNs aggregated for the selected portal. FQDNs are displayed with their record type, targets, and description.

### Grouping

FQDNs are organized into groups based on:
- **Source**: `manual` (from DNS CR spec), `external-dns` (auto-discovered), or `remote` (fetched from a remote portal)
- **Group name**: determined by annotations, labels, namespace mapping, or the default group (see [Annotations](../annotations))

### Search and Filters

The links page provides:
- **Search**: filter FQDNs by name
- **Source filter**: show only `manual`, `external-dns`, or `remote` entries
- **Namespace filter**: filter by originating namespace

### Portal Navigation

When multiple portals exist, the navigation bar allows switching between portals. Each portal shows only the FQDNs routed to it.

### Theme Toggle

The toolbar includes a theme toggle button that cycles between light, dark, and system modes. The selected theme is persisted in `localStorage` and applied via CSS class on the `<html>` element using Tailwind's dark mode class strategy.

### Help / MCP Page

The Help page (`/help`) provides:
- A table of available MCP tools (`search_fqdns`, `list_portals`, `get_fqdn_details`) with their descriptions and filters
- Setup instructions for Claude Desktop, Claude Code, and Cursor with copy-to-clipboard config snippets
- Example queries to try with an AI assistant

## Technology Stack

The web UI is built with:

- **React 19** with functional components and hooks
- **Vite** for development server and production bundling
- **Tailwind CSS v4** for styling
- **shadcn/ui** (Radix UI primitives) for UI components
- **TanStack Query v5** for server state and data fetching
- **React Router v7** for client-side routing
- **Connect protocol** clients for gRPC communication with the backend
- **Buf-generated** TypeScript clients (in `web/src/gen/`)

The React app is compiled at build time and served as static files by the Echo v5 web server inside the operator container.
