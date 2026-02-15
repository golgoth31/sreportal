---
title: Web UI
weight: 5
---

SRE Portal includes an Angular 19 single-page application served directly by the operator.

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

The root URL redirects to the `main` portal's links page. Each portal defined in the cluster gets its own route.

## Features

### FQDN Display

The links page shows all FQDNs aggregated for the selected portal. FQDNs are displayed with their record type, targets, and description.

### Grouping

FQDNs are organized into groups based on:
- **Source**: `manual` (from DNS CR spec) or `external-dns` (auto-discovered)
- **Group name**: determined by annotations, labels, namespace mapping, or the default group (see [Annotations](../annotations))

### Search and Filters

The links page provides:
- **Search**: filter FQDNs by name
- **Source filter**: show only manual or external-dns entries
- **Namespace filter**: filter by originating namespace

### Portal Navigation

When multiple portals exist, the navigation bar allows switching between portals. Each portal shows only the FQDNs routed to it.

## Technology Stack

The web UI is built with:

- **Angular 19** with standalone components
- **Signals** for reactive state management
- **Connect protocol** clients for gRPC communication with the backend
- **Buf-generated** TypeScript clients (in `web/src/gen/`)

The Angular app is compiled at build time and served as static files by the Echo v5 web server inside the operator container.
