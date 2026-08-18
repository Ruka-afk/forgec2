"use client";

import { create } from "zustand";
import { api } from "./api";
import { paths } from "./api-paths";
import { onWSMessage } from "./wsContext";
import type { DashboardStats } from "@/types/agent";

interface OnlineUser {
  username: string;
  connected_at: string;
}

let statsInFlight: Promise<void> | null = null;
let statsWSInit = false;
let statsRefreshTimer: ReturnType<typeof setTimeout> | null = null;

const STATS_REFRESH_DEBOUNCE_MS = 800;

// Events that change global counters (agents, tasks). Debounced so bursts of
// beacon chatter coalesce into a single dashboard refresh.
const STATS_EVENTS = new Set(["agent_online", "agent_offline", "agent_data_update", "task_update"]);

interface AppState {
  stats: DashboardStats | null;
  statsError?: string;

  // Sidebar layout state
  sidebarCollapsed: boolean;
  isMobile: boolean;
  mobileMenuOpen: boolean;
  toggleSidebar: () => void;
  setMobileMenuOpen: (open: boolean) => void;
  setIsMobile: (mobile: boolean) => void;

  // Online users
  onlineUsers: OnlineUser[];
  setOnlineUsers: (users: OnlineUser[]) => void;
currentUsername: string;
  setCurrentUsername: (name: string) => void;
  currentUserRole: string;
  setCurrentUserRole: (role: string) => void;
  /** null = not yet loaded from /api/me; UI gates fail open until known. */
  currentPermissions: string[] | null;
  setCurrentPermissions: (perms: string[]) => void;

  // Command palette
  commandPaletteOpen: boolean;
  setCommandPaletteOpen: (open: boolean) => void;

  // Display density (comfortable | compact) — drives [data-density] on <html>
  density: "comfortable" | "compact";
  setDensity: (d: "comfortable" | "compact") => void;

  // Focus mode — hides shell chrome for a full-viewport console
  focusMode: boolean;
  toggleFocusMode: () => void;
  setFocusMode: (b: boolean) => void;

  fetchStats: () => Promise<void>;
}

const DENSITY_KEY = "forgec2_density";
const FOCUS_KEY = "forgec2_focus_mode";

function applyDensity(d: "comfortable" | "compact") {
  if (typeof document === "undefined") return;
  document.documentElement.setAttribute("data-density", d);
}
function applyFocus(on: boolean) {
  if (typeof document === "undefined") return;
  document.documentElement.setAttribute("data-focus", on ? "on" : "off");
}

export const useAppStore = create<AppState>((set, get) => ({
  stats: null,
  onlineUsers: [],
  currentUsername: "",
  currentUserRole: "",
  currentPermissions: null,

  sidebarCollapsed: typeof window !== "undefined"
    ? localStorage.getItem("forgec2_sidebar_collapsed") === "true"
    : false,
  isMobile: false,
  mobileMenuOpen: false,
  commandPaletteOpen: false,

  density:
    typeof window !== "undefined" &&
    (localStorage.getItem(DENSITY_KEY) === "compact" ||
      localStorage.getItem(DENSITY_KEY) === "comfortable")
      ? (localStorage.getItem(DENSITY_KEY) as "comfortable" | "compact")
      : "comfortable",
  focusMode:
    typeof window !== "undefined" && localStorage.getItem(FOCUS_KEY) === "true",

  toggleSidebar: () => {
    const { isMobile, mobileMenuOpen, sidebarCollapsed } = get();
    if (isMobile) set({ mobileMenuOpen: !mobileMenuOpen });
    else {
      const next = !sidebarCollapsed;
      localStorage.setItem("forgec2_sidebar_collapsed", String(next));
      set({ sidebarCollapsed: next });
    }
  },

  setMobileMenuOpen: (open: boolean) => set({ mobileMenuOpen: open }),
  setIsMobile: (mobile: boolean) =>
    set({ isMobile: mobile, mobileMenuOpen: mobile ? false : get().mobileMenuOpen }),

  setOnlineUsers: (users) => set({ onlineUsers: users }),
  setCurrentUsername: (name) => set({ currentUsername: name }),
  setCurrentUserRole: (role) => set({ currentUserRole: role }),
  setCurrentPermissions: (perms) => set({ currentPermissions: perms }),
  setCommandPaletteOpen: (open) => set({ commandPaletteOpen: open }),

  setDensity: (d) => {
    if (typeof window !== "undefined") localStorage.setItem(DENSITY_KEY, d);
    applyDensity(d);
    set({ density: d });
  },
  toggleFocusMode: () => {
    const next = !get().focusMode;
    if (typeof window !== "undefined") localStorage.setItem(FOCUS_KEY, String(next));
    applyFocus(next);
    set({ focusMode: next });
  },
  setFocusMode: (b) => {
    if (typeof window !== "undefined") localStorage.setItem(FOCUS_KEY, String(b));
    applyFocus(b);
    set({ focusMode: b });
  },

  fetchStats: async () => {
    if (statsInFlight) return statsInFlight;
    statsInFlight = (async () => {
      try {
        const stats = await api.get<DashboardStats>(paths.dashboard.v1);
        set({ stats, statsError: undefined });
      } catch (e) {
        if (process.env.NODE_ENV === "development") console.error("[store] fetchStats failed", e);
        set({ statsError: e instanceof Error ? e.message : "Failed to load stats" });
      } finally {
        statsInFlight = null;
      }
    })();
    return statsInFlight;
  },
}));

// Apply persisted density/focus state to <html> on first client load.
applyDensity(useAppStore.getState().density);
applyFocus(useAppStore.getState().focusMode);

// Wire WebSocket events to a debounced stats refresh. Idempotent — safe to
// call from anywhere (Sidebar mounts it once).
// Derived selector: width the sidebar occupies when rendered (0 on mobile
// when the drawer is closed). Referenced by AppLayout/TopBar via useAppStore.
export function selectSidebarWidth(state: AppState): number {
  if (state.isMobile && !state.mobileMenuOpen) return 0;
  return state.sidebarCollapsed ? 64 : 192;
}

// Wire WebSocket events to a debounced stats refresh. Idempotent — safe to
// call from anywhere (Sidebar mounts it once).
export function initStatsWSListener() {
  if (statsWSInit) return;
  statsWSInit = true;
  onWSMessage((msg) => {
    if (!STATS_EVENTS.has(String(msg.type))) return;
    if (statsRefreshTimer) clearTimeout(statsRefreshTimer);
    statsRefreshTimer = setTimeout(() => {
      statsRefreshTimer = null;
      void useAppStore.getState().fetchStats();
    }, STATS_REFRESH_DEBOUNCE_MS);
  });
}
