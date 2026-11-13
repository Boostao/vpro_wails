import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";

const explicitPort = process.env.WAILS_VITE_PORT?.trim();

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(explicitPort) || 9245,
    strictPort: Boolean(explicitPort),
  },
  plugins: [tailwindcss(), svelte(), wails("./bindings")],
  build: {
    chunkSizeWarningLimit: 1120,
    rollupOptions: {
      output: {
        manualChunks(id) {
          const normalized = id.replaceAll("\\", "/");
          if (
            id.includes("vite/preload-helper") ||
            id.includes("commonjsHelpers") ||
            normalized.includes("/node_modules/tslib/")
          ) {
            return "preload-helper";
          }
          if (
            normalized.includes("/node_modules/maplibre-gl/") ||
            normalized.includes("/node_modules/pmtiles/") ||
            normalized.includes("/node_modules/fflate/")
          ) {
            return "vendor-maplibre";
          }
          if (
            normalized.includes("/node_modules/@deck.gl/") ||
            normalized.includes("/node_modules/@luma.gl/") ||
            normalized.includes("/node_modules/@math.gl/") ||
            normalized.includes("/node_modules/@loaders.gl/") ||
            normalized.includes("/node_modules/@probe.gl/") ||
            normalized.includes("/node_modules/mjolnir.js/")
          ) {
            return "vendor-deckgl";
          }
          if (normalized.endsWith("/resources/fs882-xl-layout.json")) {
            return "layout-fs882-xl";
          }
          if (normalized.endsWith("/resources/fs882-two-page-layout.json")) {
            return "layout-fs882-two-page";
          }
          if (normalized.endsWith("/resources/fs882-two-page-chars-layout.json")) {
            return "layout-fs882-two-page-chars";
          }
          return undefined;
        },
      },
    },
  },
});
