"use client";

import { create } from "zustand";
import { api } from "./api";
import { paths } from "./api-paths";
import { onWSMessage } from "./wsContext";
import type { DashboardStats } from "@/types/agent";

export interface OnlineUser {
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
  getSidebarWidth: () => number;

  // Online users
  onlineUsers: OnlineUser[];
  setOnlineUsers: (users: OnlineUser[]) => void;
  currentUsername: string;
  setCurrentUsername: (name: string) => void;
  currentUserRole: string;
  setCurrentUserRole: (role: string) => void;

  // Command palette
  commandPaletteOpen: boolean;
  setCommandPaletteOpen: (open: boolean) => void;

  fetchStats: () => Promise<void>;
}

export const useAppStore = create<AppState>((set, get) => ({
  stats: null,
  onlineUsers: [],
  currentUsername: "",
  currentUserRole: "",

  sidebarCollapsed: typeof window !== "undefined"
    ? localStorage.getItem("forgec2_sidebar_collapsed") === "true"
    : false,
  isMobile: false,
  mobileMenuOpen: false,
  commandPaletteOpen: false,

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

  getSidebarWidth: () => {
    const state = get();
    if (state.isMobile && !state.mobileMenuOpen) return 0;
    return state.sidebarCollapsed ? 64 : 192;
  },

  setOnlineUsers: (users) => set({ onlineUsers: users }),
  setCurrentUsername: (name) => set({ currentUsername: name }),
  setCurrentUserRole: (role) => set({ currentUserRole: role }),
  setCommandPaletteOpen: (open) => set({ commandPaletteOpen: open }),

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
