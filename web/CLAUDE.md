# Web UI — React 19

## Stack

React 19 + Vite + Tailwind CSS v4 + shadcn/ui + TanStack Query v5 + React Router v7

## Architecture

Feature-based Clean Architecture: `domain → infrastructure → hooks → ui` layers.

Features live in `src/features/` (dns, portal, alertmanager, mcp).

## Build

- **Output**: `dist/web/browser/` (embedded into Go binary via `ui_embed.go`)
- **Dev**: `npm run dev --prefix web`

## Routes

| Path | Page |
|------|------|
| `''` | Redirects to `/main/links` |
| `:portalName/links` | LinksPage |
| `:portalName/alerts` | AlertsPage |
| `/help` | McpPage |

## Key Details

- **Sidebar**: Left menu per portal with Links and Alerts (Alerts only when portal has Alertmanager resources)
- **LinksPage**: FQDNs grouped by group name, search + group filter, sync status dots
- **AlertsPage**: Alertmanager resources with active alerts (search/state filters), collapsible cards per resource
- **State**: TanStack Query for server state (5s polling for FQDNs, 30s stale for portals)
- **Connect clients**: Module-level transport singletons in `dnsApi.ts`, `portalApi.ts`, `alertmanagerApi.ts`
- **Theme**: Light / dark / system toggle, stored in localStorage, applied via `.dark` class
- **Error handling**: Router-level `errorElement` on all routes; error alert in LinksPage

## shadcn/ui

Installed components: button, skeleton, sonner, tooltip, badge, input, select, collapsible, separator, table

## Auto-generated — DO NOT EDIT

`src/gen/` contains Buf-generated Connect clients. `erasableSyntaxOnly` is disabled in `tsconfig.app.json` due to generated TypeScript enums.

## Testing

```bash
npm test --prefix web   # Vitest
```
