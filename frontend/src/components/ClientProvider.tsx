"use client";

import { useEffect } from "react";
import { I18nProvider } from "@/lib/i18n";
import { ThemeProvider } from "@/lib/theme";
import { WebSocketProvider } from "@/lib/wsContext";
import ErrorBoundary from "./ErrorBoundary";
import SessionTimeoutWarning from "./SessionTimeoutWarning";
import RateLimitBanner from "./RateLimitBanner";
import NetworkStatusBanner from "./NetworkStatusBanner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useAppStore } from "@/lib/store";
import type { PermissionKey } from "@/lib/permission-keys";

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

type CurrentUser = {
  username?: string;
  role?: string;
  permissions?: string[];
};

function readCurrentUser(payload: unknown): CurrentUser {
  if (!payload || typeof payload !== "object") return {};
  const record = payload as Record<string, unknown>;
  const candidate = record.data && typeof record.data === "object"
    ? record.data as Record<string, unknown>
    : record;
  return {
    username: typeof candidate.username === "string" ? candidate.username : undefined,
    role: typeof candidate.role === "string" ? candidate.role : undefined,
    permissions: Array.isArray(candidate.permissions)
      ? candidate.permissions.filter((permission): permission is string => typeof permission === "string")
      : undefined,
  };
}

/** Load authorization independently of the sidebar so focus/mobile layouts
 * receive the same permission state. `unwrap:false` also tolerates legacy and
 * current `/api/me` response shapes during rolling upgrades. */
function useCurrentUserBootstrap() {
  const setCurrentUsername = useAppStore((state) => state.setCurrentUsername);
  const setCurrentUserRole = useAppStore((state) => state.setCurrentUserRole);
  const setCurrentPermissions = useAppStore((state) => state.setCurrentPermissions);

  useEffect(() => {
    let active = true;
    void api.get<unknown>(paths.auth.me, { unwrap: false }).then((payload) => {
      if (!active) return;
      const user = readCurrentUser(payload);
      if (user.username) setCurrentUsername(user.username);
      if (user.role) setCurrentUserRole(user.role);
      if (user.permissions) setCurrentPermissions(user.permissions as PermissionKey[]);
    }).catch(() => { /* the session and network banners surface auth failures */ });
    return () => { active = false; };
  }, [setCurrentPermissions, setCurrentUserRole, setCurrentUsername]);
}

export default function ClientProvider({ children }: { children: React.ReactNode }) {
  useChunkErrorReload();
  useCurrentUserBootstrap();
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
