"use client";

import { useEffect } from "react";
import { I18nProvider } from "@/lib/i18n";
import { ThemeProvider } from "@/lib/theme";
import { WebSocketProvider } from "@/lib/wsContext";
import ErrorBoundary from "./ErrorBoundary";
import SessionTimeoutWarning from "./SessionTimeoutWarning";
import RateLimitBanner from "./RateLimitBanner";
import NetworkStatusBanner from "./NetworkStatusBanner";

const CHUNK_ERROR_RE = /dynamically imported module|Loading chunk|Importing a module script|Failed to fetch dynamically/i;
const RELOAD_FLAG = "chunkErrorReloadAt";

/**
 * After a redeploy the old hashed JS chunks are removed server-side; a tab
 * still running the previous build then fails to lazily import route chunks
 * (404), which surfaces as blank views. Auto-reload once to pick up the new
 * build instead of leaving the operator with an empty screen.
 */
function useChunkErrorReload() {
  useEffect(() => {
    const reloadOnChunkError = () => {
      const last = Number(sessionStorage.getItem(RELOAD_FLAG) || 0);
      if (Date.now() - last < 15000) return;
      sessionStorage.setItem(RELOAD_FLAG, String(Date.now()));
      window.location.reload();
    };
    const onError = (e: ErrorEvent) => {
      if (CHUNK_ERROR_RE.test(String(e.message || ""))) reloadOnChunkError();
    };
    const onRejection = (e: PromiseRejectionEvent) => {
      if (CHUNK_ERROR_RE.test(String(e.reason?.message || e.reason || ""))) reloadOnChunkError();
    };
    window.addEventListener("error", onError);
    window.addEventListener("unhandledrejection", onRejection);
    return () => {
      window.removeEventListener("error", onError);
      window.removeEventListener("unhandledrejection", onRejection);
    };
  }, []);
}

export default function ClientProvider({ children }: { children: React.ReactNode }) {
  useChunkErrorReload();
  return (
    <ErrorBoundary>
      <ThemeProvider>
        <I18nProvider>
          <WebSocketProvider>
            {children}
            <SessionTimeoutWarning />
            <RateLimitBanner />
            <NetworkStatusBanner />
          </WebSocketProvider>
        </I18nProvider>
      </ThemeProvider>
    </ErrorBoundary>
  );
}
