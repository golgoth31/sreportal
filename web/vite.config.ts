import path from "path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
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
          // React core + all React-dependent UI libs (radix, lucide, etc.) in one
          // chunk to avoid circular chunk dependencies (ui-vendor ↔ react-vendor).
          // Combined size ~430 kB — safely under the 500 kB threshold.
          return "vendor";
        },
      },
    },
  },
})
