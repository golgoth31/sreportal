/// <reference types="vitest/config" />
import path from "path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  test: {
    // happy-dom aligns Request/AbortSignal with MSW interceptors (Connect-RPC fetch).
    environment: "happy-dom",
    setupFiles: "./src/test/setup.ts",
    include: ["src/**/*.test.{ts,tsx}"],
    css: true,
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    // Match the path expected by the Go embed directive in ui_embed.go
    outDir: "dist/web/browser",
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) return;
          // TanStack Query — isolated because it changes at its own cadence
          if (id.includes("/@tanstack/")) return "query-vendor";
          // Connect/Buf runtime — isolated because it's large and stable
          if (id.includes("/@bufbuild/") || id.includes("/@connectrpc/")) {
            return "connect-vendor";
          }
          // React core — split from UI libs to keep each chunk under Rollup's 500 kB warning
          if (
            id.includes("/node_modules/react/") ||
            id.includes("/node_modules/react-dom/") ||
            id.includes("/node_modules/scheduler/")
          ) {
            return "react-vendor";
          }
          // Heavy leaf deps (would otherwise inflate the catch-all vendor chunk ~845 kB)
          if (id.includes("/recharts")) return "recharts-vendor";
          if (id.includes("/lucide-react")) return "lucide-vendor";
          return "vendor";
        },
      },
    },
  },
})
