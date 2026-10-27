# Soft Material UI (tokens-first) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restyle the SRE Portal web UI to Soft Material (4px radius, opaque AppBar, soft elevation, calm 2026 density cues) while keeping shadcn, indigo brand, and Instrument Serif.

**Architecture:** Tokens-first. Change CSS variables and elevation in `web/src/index.css`, then align listed shadcn primitives and shell components to those tokens. No MUI. No feature-page layout rewrites. Move the static design preview out of `web/public` before merge so it is not embedded.

**Tech Stack:** React 19, Vite, Tailwind CSS v4, shadcn/ui, Vitest + happy-dom (`css: true`).

**Reference:** Design doc `docs/superpowers/specs/2026-10-27-soft-material-ui-design.md`. Preview intent: `web/public/material-preview.html` (relocated in Task 5).

## Global Constraints

- Keep shadcn CVA variant **names** (`default` / `outline` / `ghost` / etc.); only restyle them.
- Keep brand indigo + Instrument Serif + Inter + JetBrains Mono.
- No `@mui/material`, no MD3 pill nav, no colored AppBar, no ripple.
- Pass 1 file scope only: tokens, `button`/`card`/`badge`/`input`/`select`, `RootLayout` / `PortalSidebar` / `PortalNav`, preview relocate.
- Commits: conventional (`feat:` / `fix:` / `chore:` / `test:`). Only commit when the human asked or the plan step explicitly requires it in an agent session where the human already approved plan execution.

---

## File map

| File | Responsibility |
|---|---|
| `web/src/index.css` | Soft Material tokens (radius, surfaces, dark primary, elevation vars) |
| `web/src/theme/soft-material-tokens.test.ts` | Assert key CSS custom properties after importing tokens |
| `web/src/components/ui/button.tsx` | Soft Material filled/outlined/text look |
| `web/src/components/ui/card.tsx` | 4px radius, border + elevation-1 |
| `web/src/components/ui/badge.tsx` | 4px radius (drop pill `rounded-4xl`) |
| `web/src/components/ui/input.tsx` | 4px radius |
| `web/src/components/ui/select.tsx` | 4px radius on trigger/content |
| `web/src/components/RootLayout.tsx` | Opaque AppBar ~48px, soft elev, calmer footer |
| `web/src/components/PortalSidebar.tsx` | 4px nav radius, token-aligned active state |
| `web/src/components/PortalNav.tsx` | Radius/tracking aligned to Soft Material |
| `docs/superpowers/previews/material-preview.html` | Design reference (moved from `web/public`) |

---

### Task 1: Soft Material CSS tokens + token test

**Files:**
- Modify: `web/src/index.css`
- Create: `web/src/theme/soft-material-tokens.test.ts`

**Interfaces:**
- Consumes: existing `:root` / `.dark` / `@theme inline` structure in `index.css`
- Produces: `--radius: 0.25rem`; `--elevation-1` / `--elevation-2`; retuned `--background`, `--card`, `--sidebar*`, dark `--primary`; `@theme inline` may expose elevation if useful via arbitrary values (`shadow-[var(--elevation-1)]`)

- [ ] **Step 1: Write the failing token test**

Create `web/src/theme/soft-material-tokens.test.ts`:

```ts
import { beforeAll, describe, expect, it } from "vitest";

import "../index.css";

describe("soft material tokens", () => {
  beforeAll(() => {
    document.documentElement.classList.remove("dark");
  });

  it("sets Soft Material radius to 0.25rem on :root", () => {
    const radius = getComputedStyle(document.documentElement)
      .getPropertyValue("--radius")
      .trim();
    expect(radius).toBe("0.25rem");
  });

  it("defines elevation-1 for floating surfaces", () => {
    const elev = getComputedStyle(document.documentElement)
      .getPropertyValue("--elevation-1")
      .trim();
    expect(elev.length).toBeGreaterThan(0);
  });

  it("uses a brighter primary in dark mode than a near-black surface", () => {
    document.documentElement.classList.add("dark");
    const primary = getComputedStyle(document.documentElement)
      .getPropertyValue("--primary")
      .trim();
    const background = getComputedStyle(document.documentElement)
      .getPropertyValue("--background")
      .trim();
    expect(primary.length).toBeGreaterThan(0);
    expect(background.length).toBeGreaterThan(0);
    expect(primary).not.toBe(background);
    document.documentElement.classList.remove("dark");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test --prefix web -- src/theme/soft-material-tokens.test.ts`

Expected: FAIL — `--radius` still `0.625rem` and/or `--elevation-1` empty.

- [ ] **Step 3: Update `web/src/index.css` tokens**

In `:root`:

1. Set `--radius: 0.25rem;`
2. Retune surfaces toward calm Soft Material light (keep indigo primary hue ~277):

```css
--background: oklch(0.985 0.002 240);
--foreground: oklch(0.18 0.02 260);
--card: oklch(1 0 0);
--card-foreground: oklch(0.18 0.02 260);
/* keep --primary indigo brand roughly as today */
--muted: oklch(0.96 0.005 240);
--muted-foreground: oklch(0.5 0.02 250);
--border: oklch(0.9 0.01 240);
--input: oklch(0.9 0.01 240);
--sidebar: oklch(0.97 0.005 240);
--sidebar-foreground: oklch(0.18 0.02 260);
--sidebar-border: oklch(0.9 0.01 240);
--elevation-1: 0 1px 2px rgb(15 23 42 / 0.06), 0 1px 3px rgb(15 23 42 / 0.04);
--elevation-2: 0 1px 2px rgb(15 23 42 / 0.08), 0 2px 6px rgb(15 23 42 / 0.06);
```

In `.dark`:

```css
--background: oklch(0.16 0.015 260);
--foreground: oklch(0.93 0.01 240);
--card: oklch(0.2 0.02 260);
--card-foreground: oklch(0.93 0.01 240);
--primary: oklch(0.75 0.14 290); /* ~#a78bfa family */
--primary-foreground: oklch(0.22 0.05 290);
--muted: oklch(0.25 0.015 260);
--muted-foreground: oklch(0.7 0.02 250);
--border: oklch(1 0 0 / 8%);
--input: oklch(1 0 0 / 12%);
--sidebar: oklch(0.14 0.015 260);
--sidebar-foreground: oklch(0.93 0.01 240);
--sidebar-border: oklch(1 0 0 / 8%);
--sidebar-primary: oklch(0.75 0.14 290);
--elevation-1: 0 1px 2px rgb(0 0 0 / 0.45), 0 1px 3px rgb(0 0 0 / 0.35);
--elevation-2: 0 1px 3px rgb(0 0 0 / 0.5), 0 2px 8px rgb(0 0 0 / 0.4);
```

Keep existing chart tokens unless contrast clearly fails after visual check in Task 5.

Do **not** remove Instrument Serif / Inter / JetBrains imports.

- [ ] **Step 4: Run token test to verify it passes**

Run: `npm test --prefix web -- src/theme/soft-material-tokens.test.ts`

Expected: PASS

- [ ] **Step 5: Commit** (when executing with commit approval)

```bash
git add web/src/index.css web/src/theme/soft-material-tokens.test.ts
git commit -m "$(cat <<'EOF'
feat(web): soft material theme tokens and elevation

EOF
)"
```

---

### Task 2: Button + Card Soft Material chrome

**Files:**
- Modify: `web/src/components/ui/button.tsx`
- Modify: `web/src/components/ui/card.tsx`

**Interfaces:**
- Consumes: `--radius*`, `--elevation-1`, primary/border tokens from Task 1
- Produces: buttons without `active:translate-y-px`; cards with `rounded-md`/`rounded-[var(--radius)]` + `border` + `shadow-[var(--elevation-1)]` instead of `ring-1 ring-foreground/10` + `rounded-xl`

- [ ] **Step 1: Write failing assertions via a thin presentational smoke test**

Create `web/src/components/ui/soft-material-chrome.test.tsx`:

```tsx
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

describe("soft material chrome", () => {
  it("button root does not use translate-y press motion", () => {
    const { container } = render(<Button>Save</Button>);
    const btn = container.querySelector('[data-slot="button"]');
    expect(btn?.className).not.toMatch(/active:translate-y-px/);
  });

  it("card uses elevation shadow token class and not foreground ring", () => {
    const { container } = render(<Card>Body</Card>);
    const card = container.querySelector('[data-slot="card"]');
    expect(card?.className).toMatch(/shadow-\[var\(--elevation-1\)\]/);
    expect(card?.className).not.toMatch(/ring-1/);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test --prefix web -- src/components/ui/soft-material-chrome.test.tsx`

Expected: FAIL on translate-y and/or ring/elevation class.

- [ ] **Step 3: Update `button.tsx`**

Change the base `cva` string:

- Replace leading `rounded-lg` with `rounded-md` (resolves via `--radius-md` ≈ 4px stack after Task 1).
- Remove `active:translate-y-px`.
- Keep variant names; optionally add soft elev on `default`: `shadow-[var(--elevation-1)]`.
- For size variants that hardcode `rounded-[min(var(--radius-md),10px)]` / `12px`, simplify to `rounded-md` so Soft Material radius wins.

Example base (keep other utilities intact):

```tsx
const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center rounded-md border border-transparent bg-clip-padding text-sm font-medium whitespace-nowrap transition-colors outline-none select-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground shadow-[var(--elevation-1)] [a]:hover:bg-primary/90",
        outline:
          "border-border bg-background hover:bg-muted hover:text-foreground aria-expanded:bg-muted aria-expanded:text-foreground dark:border-input dark:bg-input/30 dark:hover:bg-input/50",
        secondary:
          "bg-secondary text-secondary-foreground hover:bg-secondary/80 aria-expanded:bg-secondary aria-expanded:text-secondary-foreground",
        ghost:
          "hover:bg-muted hover:text-foreground aria-expanded:bg-muted aria-expanded:text-foreground dark:hover:bg-muted/50",
        destructive:
          "bg-destructive/10 text-destructive hover:bg-destructive/20 focus-visible:border-destructive/40 focus-visible:ring-destructive/20 dark:bg-destructive/20 dark:hover:bg-destructive/30 dark:focus-visible:ring-destructive/40",
        link: "text-primary underline-offset-4 hover:underline",
      },
      // sizes: use rounded-md; drop min(radius-md, 10px/12px) caps
      size: {
        default:
          "h-8 gap-1.5 px-2.5 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2",
        xs: "h-6 gap-1 rounded-md px-2 text-xs in-data-[slot=button-group]:rounded-md has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-7 gap-1 rounded-md px-2.5 text-[0.8rem] in-data-[slot=button-group]:rounded-md has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3.5",
        lg: "h-9 gap-1.5 px-2.5 has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3",
        icon: "size-8",
        "icon-xs":
          "size-6 rounded-md in-data-[slot=button-group]:rounded-md [&_svg:not([class*='size-'])]:size-3",
        "icon-sm":
          "size-7 rounded-md in-data-[slot=button-group]:rounded-md",
        "icon-lg": "size-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)
```

- [ ] **Step 4: Update `card.tsx`**

Replace Card root classes so Soft Material wins:

```tsx
className={cn(
  "group/card flex flex-col gap-3 overflow-hidden rounded-md border border-border bg-card py-3 text-sm text-card-foreground shadow-[var(--elevation-1)] has-data-[slot=card-footer]:pb-0 has-[>img:first-child]:pt-0 data-[size=sm]:gap-2 data-[size=sm]:py-2 data-[size=sm]:has-data-[slot=card-footer]:pb-0 *:[img:first-child]:rounded-t-md *:[img:last-child]:rounded-b-md",
  className
)}
```

Update `CardHeader` / `CardFooter` rounded corners from `rounded-t-xl` / `rounded-b-xl` to `rounded-t-md` / `rounded-b-md`. Keep `CardFooter` border-t; prefer `bg-muted/40` over heavy wash if present.

- [ ] **Step 5: Run chrome test + ThemeToggle**

Run:

```bash
npm test --prefix web -- src/components/ui/soft-material-chrome.test.tsx src/components/ThemeToggle.test.tsx
```

Expected: PASS

- [ ] **Step 6: Commit** (when executing with commit approval)

```bash
git add web/src/components/ui/button.tsx web/src/components/ui/card.tsx web/src/components/ui/soft-material-chrome.test.tsx
git commit -m "$(cat <<'EOF'
feat(web): soft material button and card chrome

EOF
)"
```

---

### Task 3: Badge, Input, Select radius alignment

**Files:**
- Modify: `web/src/components/ui/badge.tsx`
- Modify: `web/src/components/ui/input.tsx`
- Modify: `web/src/components/ui/select.tsx`
- Modify: `web/src/components/ui/soft-material-chrome.test.tsx`

**Interfaces:**
- Consumes: Task 1 radius tokens
- Produces: non-pill badges (`rounded-md` not `rounded-4xl`); input/select triggers `rounded-md`

- [ ] **Step 1: Extend chrome test (failing)**

Add to `soft-material-chrome.test.tsx`:

```tsx
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";

it("badge is not fully pill-rounded", () => {
  const { container } = render(<Badge>ok</Badge>);
  const el = container.querySelector('[data-slot="badge"]');
  expect(el?.className).not.toMatch(/rounded-4xl/);
  expect(el?.className).toMatch(/rounded-md/);
});

it("input uses rounded-md", () => {
  const { container } = render(<Input aria-label="q" />);
  const el = container.querySelector('[data-slot="input"]');
  expect(el?.className).toMatch(/rounded-md/);
  expect(el?.className).not.toMatch(/rounded-lg/);
});
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `npm test --prefix web -- src/components/ui/soft-material-chrome.test.tsx`

- [ ] **Step 3: Patch components**

`badge.tsx` base string: change `rounded-4xl` → `rounded-md`.

`input.tsx`: change `rounded-lg` → `rounded-md`.

`select.tsx`:

- `SelectTrigger`: `rounded-lg` → `rounded-md`; replace `data-[size=sm]:rounded-[min(var(--radius-md),10px)]` with `data-[size=sm]:rounded-md`.
- `SelectContent`: `rounded-lg` → `rounded-md` (keep shadow; optional `shadow-[var(--elevation-2)]` instead of `shadow-md`).

- [ ] **Step 4: Run tests**

Run:

```bash
npm test --prefix web -- src/components/ui/soft-material-chrome.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit** (when executing with commit approval)

```bash
git add web/src/components/ui/badge.tsx web/src/components/ui/input.tsx web/src/components/ui/select.tsx web/src/components/ui/soft-material-chrome.test.tsx
git commit -m "$(cat <<'EOF'
feat(web): soft material radius on badge input select

EOF
)"
```

---

### Task 4: Shell — RootLayout, PortalSidebar, PortalNav

**Files:**
- Modify: `web/src/components/RootLayout.tsx`
- Modify: `web/src/components/PortalSidebar.tsx`
- Modify: `web/src/components/PortalNav.tsx`
- Create: `web/src/components/RootLayout.soft-material.test.tsx`

**Interfaces:**
- Consumes: `--elevation-2`, opaque surfaces from Task 1
- Produces: AppBar without `backdrop-blur`; ~`h-12` (48px); footer tracking reduced

- [ ] **Step 1: Write failing shell class test**

Create `web/src/components/RootLayout.soft-material.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router";

import { RootLayout } from "@/components/RootLayout";

vi.mock("@/features/portal/hooks/usePortals", () => ({
  usePortals: () => ({ portals: [], isLoading: false }),
}));

vi.mock("@/features/version/hooks/useVersion", () => ({
  useVersion: () => ({ version: "test" }),
}));

describe("RootLayout soft material shell", () => {
  it("app bar is opaque without backdrop blur", () => {
    const { container } = render(
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<RootLayout />}>
            <Route index element={<div>home</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );
    const header = container.querySelector("header");
    expect(header?.className).not.toMatch(/backdrop-blur/);
    expect(header?.className).toMatch(/bg-background/);
    expect(header?.className).toMatch(/shadow-\[var\(--elevation-2\)\]|shadow-\[var\(--elevation-1\)\]/);
    expect(screen.getByText(/SRE/i)).toBeInTheDocument();
  });
});
```

If `RootLayout` routing mock needs adjustment for this repo’s router patterns, keep the assertion on the real `header` element className — do not weaken by skipping the blur check.

- [ ] **Step 2: Run test — expect FAIL** (blur still present)

Run: `npm test --prefix web -- src/components/RootLayout.soft-material.test.tsx`

- [ ] **Step 3: Update `RootLayout.tsx` header + footer**

Replace header classes:

```tsx
<header className="sticky top-0 z-40 h-12 border-b border-border bg-background shadow-[var(--elevation-2)]">
  <div className="flex h-12 items-center gap-3 px-3 max-w-screen-xl mx-auto w-full">
```

Keep brand `font-display` + italic `text-primary` on “Portal”.

Help `NavLink`: `rounded-md` (already), size `size-8` to match 48px bar.

Footer:

```tsx
<footer className="border-t border-border py-2 px-4">
  <p className="text-center text-[11px] font-mono text-muted-foreground">
    SRE Portal{version ? ` · ${version}` : ""}
  </p>
</footer>
```

(Remove heavy `uppercase tracking-[0.14em]`.)

- [ ] **Step 4: Update `PortalSidebar.tsx`**

- `aside`: `border-border` (not `/60`), `bg-sidebar` (solid, drop `/40` if it fights Soft Material).
- `linkClass`: keep inset bar; ensure `rounded-md`; active `bg-background` or `bg-primary/10` with `before:` primary bar.
- Section labels: keep mono; reduce tracking from `0.16em` to `0.08em` or drop uppercase intensity.

- [ ] **Step 5: Update `PortalNav.tsx`**

- SelectTrigger: `rounded-md` (already); ensure no extra large radius.
- Label spans: reduce `tracking-[0.14em]` similarly to sidebar.

- [ ] **Step 6: Run shell + related tests**

Run:

```bash
npm test --prefix web -- src/components/RootLayout.soft-material.test.tsx src/components/ThemeToggle.test.tsx
```

Expected: PASS. If MemoryRouter setup fails for unrelated Outlet reasons, fix the test harness (not by removing Soft Material assertions).

- [ ] **Step 7: Commit** (when executing with commit approval)

```bash
git add web/src/components/RootLayout.tsx web/src/components/PortalSidebar.tsx web/src/components/PortalNav.tsx web/src/components/RootLayout.soft-material.test.tsx
git commit -m "$(cat <<'EOF'
feat(web): opaque soft material app shell

EOF
)"
```

---

### Task 5: Relocate preview + full web verification

**Files:**
- Move: `web/public/material-preview.html` → `docs/superpowers/previews/material-preview.html`
- (No production UI code in this task)

**Interfaces:**
- Consumes: completed pass 1 styling
- Produces: preview not shipped via Vite `public/`; verification evidence for acceptance criteria

- [ ] **Step 1: Move preview file**

```bash
mkdir -p docs/superpowers/previews
git mv web/public/material-preview.html docs/superpowers/previews/material-preview.html
```

If `git mv` unavailable because untracked: `mv` then `git add` / ensure `web/public/material-preview.html` is gone.

Update the design doc preview path line in `docs/superpowers/specs/2026-10-27-soft-material-ui-design.md` to `docs/superpowers/previews/material-preview.html`.

- [ ] **Step 2: Run full web unit tests + typecheck**

```bash
npm test --prefix web
npm exec --prefix web -- tsc -b --pretty false
```

Expected: all tests PASS; `tsc` exit 0.

- [ ] **Step 3: Manual visual checklist (human or browser)**

Open app light + dark:

1. AppBar opaque, ~48px, no frosted glass.
2. Radius ~4px on buttons, cards, inputs, badges.
3. Cards: border + soft shadow, not heavy ring.
4. Brand indigo + Instrument Serif title intact.
5. Sidebar active state: bar + fill, not pill.
6. DNS + Alerts pages still usable (layout may look denser from tokens only — OK for pass 1).

- [ ] **Step 4: Commit** (when executing with commit approval)

```bash
git add docs/superpowers/previews/material-preview.html docs/superpowers/specs/2026-10-27-soft-material-ui-design.md
git add -u web/public/material-preview.html
git commit -m "$(cat <<'EOF'
chore(web): move soft material preview out of public bundle

EOF
)"
```

---

## Self-review (plan vs spec)

| Spec requirement | Task |
|---|---|
| `--radius` 4px + surface/primary dark retune + elevation vars | Task 1 |
| Opaque AppBar, brand preserved, calmer footer | Task 4 |
| Sidebar Soft Material active (bar, not pill) | Task 4 |
| button/card/badge/input/select Soft Material | Tasks 2–3 |
| No MUI / no MD3 chrome swap / no page rewrites | Global Constraints + file map |
| Move preview out of `web/public` | Task 5 |
| Verification tests + tsc + manual L/D | Tasks 1–5 |
| Pass 2 deferred (KPI, dense tables, screenshots) | Not scheduled (correct) |

No TBD placeholders. Variant names preserved. Elevation token name `--elevation-1` consistent across tasks.
