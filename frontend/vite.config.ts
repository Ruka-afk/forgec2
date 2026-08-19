import { fileURLToPath, URL } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig, type Plugin } from "vite";
import { staticExportPlugin } from "./src/lib/vite/staticExport";

// Inject <link rel="preload" as="font"> for the hashed woff2 assets emitted
// during the build. The hashed filename is only known after bundling, so the
// tags are appended in transformIndexHtml (build mode) from ctx.bundle.
function fontPreloadPlugin(): Plugin {
  return {
    name: "forgec2:font-preload",
    apply: "build",
    transformIndexHtml: {
      order: "post",
      handler(_html, ctx) {
        const bundle = ctx.bundle;
        if (!bundle) return [];
        return Object.keys(bundle)
          .filter((f) => f.endsWith(".woff2"))
          .map((f) => ({
            tag: "link",
            attrs: {
              rel: "preload",
              as: "font",
              type: "font/woff2",
              crossorigin: "",
              href: "/" + f,
            },
          }));
      },
    },
  };
}

export default defineConfig(({ command, mode }) => ({
  plugins: [
    react(),
    fontPreloadPlugin(),
    ...(command === "build" ? [staticExportPlugin()] : []),
  ],
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
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) return;
          // Order matters: react-router is a react lib too.
          if (id.includes("react-router")) return "vendor-router";
          if (id.includes("@xterm")) return "vendor-xterm";
          if (id.includes("@base-ui")) return "vendor-baseui";
          if (id.includes("lucide-react")) return "vendor-icons";
          if (id.includes("dompurify")) return "vendor-dompurify";
          if (id.includes("zod")) return "vendor-zod";
          if (id.includes("sonner")) return "vendor-sonner";
          // react-interop CJS packages (use-sync-external-store etc.) do a
          // top-level require("react"); they MUST live in the same chunk as
          // react itself or the circular chunk import crashes at boot
          // ("Cannot set properties of undefined (setting 'Activity')").
          if (id.includes("use-sync-external-store") || id.includes("react-is")) return "vendor-react";
          if (id.includes("react") || id.includes("scheduler")) return "vendor-react";
          return "vendor-misc";
        },
      },
    },
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
