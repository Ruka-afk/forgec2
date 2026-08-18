import { fileURLToPath, URL } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { staticExportPlugin } from "./src/lib/vite/staticExport";

export default defineConfig(({ command, mode }) => ({
  plugins: [react(), ...(command === "build" ? [staticExportPlugin()] : [])],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      "next/link": fileURLToPath(new URL("./src/lib/next/link.tsx", import.meta.url)),
      "next/navigation": fileURLToPath(new URL("./src/lib/next/navigation.ts", import.meta.url)),
      "next/dynamic": fileURLToPath(new URL("./src/lib/next/dynamic.tsx", import.meta.url)),
    },
  },
  define:
    mode === "test"
      ? {}
      : {
          "process.env.NEXT_PUBLIC_API_BASE": JSON.stringify(process.env.NEXT_PUBLIC_API_BASE ?? ""),
          "process.env.NEXT_PUBLIC_WS_URL": JSON.stringify(process.env.NEXT_PUBLIC_WS_URL ?? ""),
          "process.env.NEXT_PUBLIC_GO_BACKEND_PORT": JSON.stringify(process.env.NEXT_PUBLIC_GO_BACKEND_PORT ?? ""),
        },
  build: {
    outDir: "out",
    emptyOutDir: true,
    assetsDir: "assets",
    target: "es2020",
    chunkSizeWarningLimit: 1200,
  },
  server: {
    proxy: {
      "/api": { target: "http://127.0.0.1:8000", changeOrigin: true },
      "/login": { target: "http://127.0.0.1:8000", changeOrigin: true },
      "/extc2": { target: "http://127.0.0.1:8000", changeOrigin: true },
      "/ws": { target: "ws://127.0.0.1:8000", ws: true },
    },
  },
}));
