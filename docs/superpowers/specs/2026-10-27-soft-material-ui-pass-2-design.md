# Soft Material UI — Pass 2 (density + shell)

- **Date**: 2026-10-27
- **Status**: Implemented (plan: `docs/superpowers/plans/2026-10-27-soft-material-ui-pass-2.md`)
- **Depends on**: Pass 1 shipped (`feat(web): Soft Material UI tokens-first restyle`, polish dark/nav)
- **Parent**: `docs/superpowers/specs/2026-10-27-soft-material-ui-design.md` §6
- **Scope**: Dense FQDN/alert lists, compact dashboard KPI, collapsible sidebar, screenshot baseline regen
- **Out of scope**: MUI, Images card-grid / Status banner layout redesigns, list virtualization, responsive auto-collapse, feature data/filter rewrites
- **Note**: Shared `table.tsx` densification is Soft Material chrome (like pass-1 button/card). Side effect on Release / Netpol / Metrics / MCP tables is acceptable; do not redesign those pages beyond the shared primitive.

## 1. Motivation

Pass 1 locked Soft Material tokens, opaque AppBar, elevation, and chrome radius. Pass 2 applies the deferred density and shell items from the parent spec so product pages match the calm/dense Soft Material intent in-product and in docs screenshots.

## 2. Decisions (validated)

| Topic | Choice |
|---|---|
| Scope | All four deferred items (lists, KPI, sidebar, screenshots) |
| FQDN layout | Convert card grid → dense rows (alerts-like), not tighten-only cards |
| KPI | `StatCard` `density="compact"`; dashboard uses it; default unchanged |
| Sidebar | Explicit toggle + `localStorage`; collapsed = icons + tooltips; default expanded |
| Approach | Density tokens + component changes (not ad-hoc page classes, not a parallel dense DS) |

## 3. Density tokens (`web/src/index.css`)

Add CSS variables (same values light/dark unless contrast fails):

| Token | Value | Usage |
|---|---|---|
| `--row-height` | `2rem` (32px) | FQDN rows; alert table body |
| `--row-height-tight` | `1.75rem` (28px) | table head / nested where needed |
| `--density-gap` | `0.25rem` | list gaps |
| `--sidebar-width` | `12rem` | expanded sidebar |
| `--sidebar-width-collapsed` | `3.5rem` | icon-only sidebar |

Do not change `--radius`, brand indigo, Instrument Serif, or pass-1 `--elevation-*`. Prefer consuming these variables over scattering raw height utilities.

Optional: thin `@utility` helpers only if they reduce duplication; otherwise use `h-[var(--row-height)]` / width vars directly.

## 4. Dense FQDN / alert lists

### FQDN

- `FqdnGroupCard`: replace multi-column card grid with a single-column vertical list.
- `FqdnCard`: become a dense row — `min-h-[var(--row-height)]`, horizontal `px-3`, one primary line: sync indicator · FQDN (`font-mono`) · type badge · target (`font-mono`) · copy control.
- Overflow / secondary fields (description, originRef): tooltip (keep one-row height).
- Group header: tighten to ~`py-2`; list skeleton ~`h-8`.
- Keep Soft Material `surface` / elevation-1; do **not** switch DNS to HTML `<table>` (custom link + copy UX stays).

### Alerts

- Shared `table.tsx` (chrome): `TableHead` height → `var(--row-height)` or `--row-height-tight`; `TableCell` → `py-1 px-2`. Applies to all table consumers (Alerts, Release, Netpol cross-ns, Metrics, MCP) as Soft Material density — not per-page one-offs.
- `AlertGroupCard` only: instance, summary, and receivers use `font-mono text-xs`.
- Resource / group headers: ~`py-2`.
- No filter/data model changes.

### Explicitly untouched (feature layouts)

Image inventory card grid, netpol matrix collapse UX, status banner / page composition — no pass-2 redesign beyond any shared `table.tsx` side effect.

## 5. Compact KPI (`StatCard` + dashboard)

- Add optional prop `density?: "default" | "compact"` (default `"default"`).
- `compact`: value `text-2xl` (not `text-4xl`); tighter header/content padding; icon ~`h-3.5 w-3.5`.
- `DashboardPage`: pass `density="compact"` to all six stat cards; grid gap `gap-2` / `gap-3`.
- No non-Card KPI strip; no KPI strip on Links/Alerts pages.
- Tests: assert compact vs default classes; existing tests remain on default.

## 6. Collapsible sidebar (icon-only)

- New hook `useSidebarCollapsed` mirroring `useTheme` persistence pattern:
  - key: `sreportal-sidebar-collapsed`
  - type: boolean
  - default: `false` (expanded)
- `PortalSidebar`:
  - expanded: `w-[var(--sidebar-width)]`, labels + “Resources” section label
  - collapsed: `w-[var(--sidebar-width-collapsed)]`, centered icons, tooltips on hover, labels visually hidden or omitted
  - toggle control (chevrons) with `aria-expanded`
- Active indicator (3px primary bar) unchanged; adjust horizontal padding when collapsed.
- Alerts badge: compact mark (dot or short count) when collapsed.
- No viewport-based auto-collapse.
- Tests: expanded/collapsed classes + localStorage persistence (mocked).

## 7. Screenshot baselines

- After UI lands: run `make screenshots` (Playwright + chromium via Makefile).
- Overwrites `docs/content/assets/img/{light,dark}/*.png`.
- Prefer no logic changes in `web/scripts/screenshots.ts` unless mocks break; default expanded sidebar keeps baselines stable.
- Commit regenerated PNGs with the UI changes.

## 8. Verification

- Automated: Vitest for density tokens, FQDN row smoke, table density, `StatCard` compact, sidebar collapse/persist; `npx tsc -b` in `web/`.
- Manual: Links, Alerts, Dashboard in light and dark; sidebar toggle + reload persistence.
- No new visual regression suite beyond screenshot regen.

## 9. Acceptance criteria

- FQDN groups render as dense mono-forward rows (not multi-col cards).
- Alert tables are denser; instance/summary/receivers are mono.
- Dashboard KPIs use `StatCard` `density="compact"`.
- Sidebar toggles to icon-only, persists in `localStorage`, tooltips when collapsed.
- Docs screenshot baselines regenerated for light and dark.
- No MUI; Soft Material radius/elevation/brand from pass 1 preserved.
- Image card grid / Status page composition unchanged (shared table chrome may densify Netpol/Release/Metrics/MCP tables).

## 10. Risks

| Risk | Mitigation |
|---|---|
| FQDN row loses useful description | Prefer tooltip over second line; keep copy/link affordances |
| Shared table density surprises other pages | Intentional Soft Material chrome; screenshots cover affected pages; no per-page layout rewrite |
| Collapsed sidebar hides badges | Compact badge/dot in collapsed mode |
| Screenshot flakiness / chromium install | Use existing `make screenshots`; retry once; do not invent alternate pipeline |
| Accidental Images/Status redesign | Do not edit Image* / Status* feature layouts; only tokens, DNS, alerts mono, StatCard/dashboard, sidebar/hook, table chrome, screenshots |
