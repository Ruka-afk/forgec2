"use client";

import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import { onWSMessage } from "../wsContext";
import { createAuthSlice } from "./auth";
import { createUiSlice } from "./ui";
import { createStatsSlice } from "./stats";
import type { AppStore } from "./types";

// Persist key for UI preferences. Values are also seeded from the legacy
// single-key storage entries (forgec2_sidebar_collapsed / forgec2_density /
// forgec2_focus_mode) via the slice initializers, so pre-persist settings
// survive the migration.
const UI_PREFS_KEY = "forgec2_ui_prefs_v1";

export const useAppStore = create<AppStore>()(
  persist(
    (...a) => ({
      ...createAuthSlice(...a),
      ...createUiSlice(...a),
      ...createStatsSlice(...a),
    }),
    {
      name: UI_PREFS_KEY,
      storage: createJSONStorage(() => localStorage),
      partialize: (s) => ({
        sidebarCollapsed: s.sidebarCollapsed,
        density: s.density,
        focusMode: s.focusMode,
      }),
    },
  ),
);

// Apply persisted density/focus state to <html> on first client load.
// Persist hydrates synchronously (localStorage), so getState() is final here.
applyDensityFromStore();
applyFocusFromStore();

function applyDensityFromStore() {
  if (typeof document === "undefined") return;
  document.documentElement.setAttribute("data-density", useAppStore.getState().density);
}
function applyFocusFromStore() {
  if (typeof document === "undefined") return;
  document.documentElement.setAttribute("data-focus", useAppStore.getState().focusMode ? "on" : "off");
}

// Derived selector: width the sidebar occupies in document layout. The mobile
// Sheet is an overlay, so it never offsets TopBar/content even while open.
export function selectSidebarWidth(state: AppStore): number {
  if (state.isMobile) return 0;
  return state.sidebarCollapsed ? 64 : 224;
}

const STATS_REFRESH_DEBOUNCE_MS = 800;
// Events that change global counters (agents, tasks). Debounced so bursts of
// beacon chatter coalesce into a single dashboard refresh.
const STATS_EVENTS = new Set(["agent_online", "agent_offline", "agent_data_update", "task_update"]);

let statsWSInit = false;
let statsRefreshTimer: ReturnType<typeof setTimeout> | null = null;

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

export type { AppStore, OnlineUser, Density } from "./types";
