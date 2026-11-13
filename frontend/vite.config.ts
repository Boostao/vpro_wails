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
});
