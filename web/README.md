# Web UI

The SRE Portal web UI is a React 19 single-page application built with Vite.

## Tech Stack

- **React 19** — UI framework
- **Vite** — dev server and bundler
- **Tailwind CSS v4** — utility-first styling
- **shadcn/ui** (Radix UI primitives) — accessible UI components
- **TanStack Query v5** — server state management
- **React Router v7** — client-side routing
- **Connect protocol** — gRPC-compatible API client (Buf-generated, in `src/gen/`)

## Development

Start the Vite development server:

```bash
npm run dev
```

The app runs at `http://localhost:5173/`. It proxies API requests to the operator backend (configure in `vite.config.ts`).

## Building

```bash
npm run build
```

Or via the root Makefile:

```bash
make build-web
```

Build output goes to `dist/web/browser/`, which is embedded in the operator binary.

## Testing

```bash
npm test
```

Tests use [Vitest](https://vitest.dev/).

## Linting

```bash
npm run lint
```

## Code Generation

The `src/gen/` directory contains Buf-generated Connect clients for the DNS and Portal services. **Do not edit these files manually.** Regenerate with:

```bash
make proto   # from the repository root
```
