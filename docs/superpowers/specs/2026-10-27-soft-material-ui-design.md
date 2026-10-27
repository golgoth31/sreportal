# Soft Material UI (shadcn tokens-first)

- **Date**: 2026-10-27
- **Status**: Design — pending implementation plan
- **Scope**: Web UI visual system only (`web/src/index.css`, selected `web/src/components/ui/*`, shell layout components)
- **Out of scope**: `@mui/material` migration, MD3 ripples / pill navigation, feature-page layout redesigns (KPI strips, dense tables), screenshot pipeline updates
- **Preview reference**: `docs/superpowers/previews/material-preview.html` (A Soft Material × 2026 calm/density trends, light + dark)

## 1. Motivation

The SRE Portal UI uses shadcn/ui + Tailwind v4 with an indigo brand and Instrument Serif display. The product should feel closer to Soft Material / Material Design elevation language (opaque AppBar, 4px radius, filled/outlined/text buttons, light elevation) while absorbing 2026 SaaS dashboard trends: minimal chrome, higher information density cues, one accent color, semantic status colors, and dark-mode tokens designed as first-class surfaces.

Chosen direction (user-validated):

- Approach: **tokens-first** on existing shadcn (not MUI, not a new component library).
- Visual: **Brand Soft Material** — keep indigo + Instrument Serif; Soft Material structure; 2026 calm density.
- Fidelity: Soft Material without ripple.

## 2. Design principles

1. **One accent** — brand indigo for interactive emphasis; green/amber only for status.
2. **Minimal chrome** — prefer 1px borders + soft elevation over heavy rings, glass blur, or decorative card grids.
3. **Soft Material structure** — radius ~4px; opaque AppBar; button trio filled / outlined / text; elevation level 1 on surfaces that float.
4. **Dark-first tokens** — dark palette is intentional (not a simple invert); primary lightens for contrast on dark surfaces.
5. **No MD3 chrome swap** — no colored AppBar, no pill nav, no Roboto-only rebrand.

## 3. Token changes (`web/src/index.css`)

| Token / concern | Light | Dark |
|---|---|---|
| `--radius` | `0.25rem` (4px) | same |
| `--primary` | keep current indigo brand | brighter indigo (preview target ~`#a78bfa` / matching oklch) |
| `--background` | near `#fafbfc` | near `#0b0e13` |
| `--card` / elevated surfaces | white | near `#12171f` |
| Borders | soft slate border | `rgba(255,255,255,.08)`-class |
| Elevation | CSS variables for soft shadow level 1 (and optional level 2 for AppBar) | darker, higher-opacity shadows + hairline border |
| Fonts | unchanged: Inter Variable, Instrument Serif, JetBrains Mono Variable | unchanged |

Add elevation utilities or CSS variables (e.g. `--elevation-1`) consumed by shell and `card` — avoid scattering raw `shadow-*` values.

Keep chart token roles; only retune if contrast fails on the new dark background.

## 4. Shell

### `RootLayout`

- AppBar: opaque surface (remove / stop relying on `backdrop-blur` frosted glass).
- Height ~48px; bottom border + soft elevation.
- Keep brand mark + Instrument Serif title; indigo italic accent on “Portal”.
- Footer: keep mono version line; reduce marketing-like tracking if it fights calm density.

### `PortalSidebar` / `PortalNav`

- Active item: surface fill + 2px inset primary bar (existing pattern, align radius to 4px).
- Inactive: muted; hover via `--accent` / muted surface — no pill shapes.
- Sidebar background from `--sidebar` tokens after retune.

## 5. shadcn component tweaks

Touch only what the Soft Material shell needs:

| Component | Change |
|---|---|
| `button` | Radius 4px; keep CVA variant **names** (`default` / `outline` / `ghost`); restyle them to Soft Material filled / outlined / text; remove playful `active:translate-y-px` if present |
| `card` | Radius 4px; replace heavy `ring` look with 1px border + `--elevation-1`; slightly tighter padding |
| `badge` | Radius 4px; status-friendly variants stay semantic |
| `input` / `select` | Radius 4px; borders aligned to new tokens |

Do **not** introduce MUI or new primitive packages. Do **not** rewrite feature pages in pass 1.

## 6. Pass 2 (shipped)

Pass 2 implemented — see `2026-10-27-soft-material-ui-pass-2-design.md`. Original scope below.

Optional follow-up after pass 1 ships and is reviewed in-product:

- Dense FQDN / alert lists (row height, mono targets).
- Compact KPI strip on overview-like pages.
- Collapsible sidebar icon-only mode.
- Regenerating `web/scripts/screenshots.ts` baselines if docs visuals drift.

## 7. Verification

- Manual: shell + DNS + Alerts in light and dark; compare against `material-preview.html` intent (not pixel-perfect).
- Automated: existing web unit tests (ThemeToggle and related) remain green; `npx tsc -b` in `web/` if types/classes change.
- No new visual regression suite required for pass 1.

## 8. Risks

| Risk | Mitigation |
|---|---|
| Cards feel too flat on dense pages | Keep elevation-1; bump only AppBar/card if hierarchy collapses |
| Dark primary too bright / washed | Tune oklch primary against card and background together |
| Screenshot / docs drift | Defer screenshot regen to pass 2 unless release docs block |
| Accidental page restyles | Limit file list to tokens + listed UI/shell files |

## 9. Acceptance criteria (pass 1)

- Light and dark themes use Soft Material radius, opaque AppBar, and soft elevation.
- Brand indigo + Instrument Serif preserved.
- No MUI dependency; no MD3 pill nav / colored AppBar.
- Preview lives at `docs/superpowers/previews/material-preview.html` (not in Vite `public/`, not in the operator UI bundle).
